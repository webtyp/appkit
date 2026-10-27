package tests

import (
	"strings"
	"testing"

	"fmt"
	"webtyp.com/appkit"
	"webtyp.com/mcp"
	"webtyp.com/model"
	"webtyp.com/router"
	"webtyp.com/router/mock"
	"webtyp.com/rpc"
)

type fakeOps struct{}

func (fakeOps) ModelName() string { return "fake" }
func (fakeOps) MountOperations(reg router.OperationRegistry) {
	reg.Operation("list", func(ctx router.Context) {
		ctx.Write([]byte("{}"))
	}).Requires("fake", model.Read)
}

func fakeAuthn(userID string) router.Middleware {
	return func(next router.HandlerFunc) router.HandlerFunc {
		return func(ctx router.Context) {
			ctx.SetUserID(userID)
			next(ctx)
		}
	}
}

func allowFakeRead(userID string, resource model.Resource, action model.Action) bool {
	return resource == "fake" && action == model.Read
}

func buildRouter(t *testing.T, cfg appkit.ServerConfig, authn router.Middleware) *mock.Router {
	t.Helper()
	if cfg.Logger == nil {
		cfg.Logger = t.Log
	}
	authz := cfg.Authorize
	if authz == nil {
		authz = appkit.DenyAll(cfg.Logger)
	}
	r := &mock.Router{}
	r.Configure(mock.Config{
		Authn:     authn,
		Authorize: authz,
	})
	if err := appkit.MountServer(r, cfg); err != nil {
		t.Fatalf("MountServer: %v", err)
	}
	if err := r.Verify(); err != nil {
		t.Fatalf("Verify: %v", err)
	}
	return r
}

func post(r *mock.Router, path, body string) (int, string) {
	ctx := &mock.Context{
		InMethod: "POST",
		InPath:   path,
		InBody:   []byte(body),
	}
	ctx.SetHeader("Content-Type", "application/json")
	r.Invoke("POST", path, ctx)
	status := ctx.Status
	if status == 0 {
		status = 200
	}
	return status, string(ctx.ResponseBody())
}

func get(r *mock.Router, path string) (int, string) {
	ctx := &mock.Context{
		InMethod: "GET",
		InPath:   path,
	}
	r.Invoke("GET", path, ctx)
	status := ctx.Status
	if status == 0 {
		status = 200
	}
	return status, string(ctx.ResponseBody())
}

func TestMCPClosedByDefault(t *testing.T) {
	r := buildRouter(t, appkit.ServerConfig{}, nil)
	status, body := post(r, mcp.MCPPath, `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`)
	if status != 200 {
		t.Fatalf("tools/list: status %d, body %s", status, body)
	}
	if !strings.Contains(body, `"tools":[]`) {
		t.Fatalf("tools/list must list zero tools, got %s", body)
	}
}

func TestMCPOpensExposedTools(t *testing.T) {
	r := buildRouter(t, appkit.ServerConfig{
		Operations: []router.OperationModule{fakeOps{}},
		MCPTools:   []mcp.ToolName{mcp.ToolNameOf("fake", "list")},
		Authorize:  allowFakeRead,
	}, fakeAuthn("test-user"))
	status, body := post(r, mcp.MCPPath, `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`)
	if status != 200 {
		t.Fatalf("tools/list: status %d, body %s", status, body)
	}
	if !strings.Contains(body, `"fake.list"`) {
		t.Fatalf("tools/list must contain fake.list, got %s", body)
	}
}

func TestRPCMounted(t *testing.T) {
	r := buildRouter(t, appkit.ServerConfig{
		Operations: []router.OperationModule{fakeOps{}},
		Authorize:  allowFakeRead,
	}, fakeAuthn("test-user"))
	path := rpc.DefaultPrefix + "/fake/list"
	status, _ := post(r, path, `{}`)
	if status != 200 {
		t.Fatalf("with session and authorize allowing fake:r, want 200, got %d", status)
	}
}

func TestAnonymousDenied(t *testing.T) {
	r := buildRouter(t, appkit.ServerConfig{
		Operations: []router.OperationModule{fakeOps{}},
		Authorize:  allowFakeRead,
	}, nil)
	path := rpc.DefaultPrefix + "/fake/list"
	status, body := post(r, path, `{}`)
	if status != 403 {
		t.Fatalf("without session, want 403, got %d (body: %s)", status, body)
	}
}

func TestDefaultDenyWithoutAuthorize(t *testing.T) {
	logger := func(a ...any) {
		line := fmt.Sprint(a...)
		t.Log(line)
	}

	r := buildRouter(t, appkit.ServerConfig{
		Operations: []router.OperationModule{fakeOps{}},
		Logger:     logger,
	}, fakeAuthn("test-user"))
	path := rpc.DefaultPrefix + "/fake/list"
	status, _ := post(r, path, `{}`)
	if status != 403 {
		t.Fatalf("without authorize, want 403, got %d", status)
	}
}

func TestIntrospectionRequiresAPIExplorer(t *testing.T) {
	r := buildRouter(t, appkit.ServerConfig{
		Operations: []router.OperationModule{fakeOps{}},
		Authorize:  allowFakeRead,
	}, nil)
	status, _ := get(r, router.IntrospectionPath)
	if status != 403 {
		t.Fatalf("GET %s without router.ResourceIntrospection read permission: want 403, got %d", router.IntrospectionPath, status)
	}
}

func TestIntrospectionOpensWithItsPermission(t *testing.T) {
	r := buildRouter(t, appkit.ServerConfig{
		Operations: []router.OperationModule{fakeOps{}},
		Authorize: func(userID string, res model.Resource, a model.Action) bool {
			return res == router.ResourceIntrospection && a == model.Read
		},
	}, fakeAuthn("test-user"))
	status, _ := get(r, router.IntrospectionPath)
	if status != 200 {
		t.Fatalf("GET %s with router.ResourceIntrospection read permission: want 200, got %d", router.IntrospectionPath, status)
	}
}

func TestRegisterMountsAppRoutes(t *testing.T) {
	r := buildRouter(t, appkit.ServerConfig{
		Authorize: allowFakeRead,
		Register: func(r router.Router) {
			r.Get("/ping", func(ctx router.Context) {
				ctx.Write([]byte("ok"))
			}).Public()
		},
	}, nil)
	status, body := get(r, "/ping")
	if status != 200 {
		t.Fatalf("GET /ping: status %d, body %s", status, body)
	}
	if body != "ok" {
		t.Fatalf("GET /ping body: want 'ok', got %q", body)
	}
}
