//go:build !wasm

package tests

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"fmt"
	"webtyp.com/appkit"
	"webtyp.com/mcp"
	"webtyp.com/model"
	"webtyp.com/router"
	"webtyp.com/rpc"
	"webtyp.com/server/httpd"
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

func buildServer(t *testing.T, cfg appkit.ServerConfig, authn router.Middleware) *httptest.Server {
	t.Helper()
	if cfg.Logger == nil {
		cfg.Logger = t.Log
	}
	authz := cfg.Authorize
	if authz == nil {
		authz = appkit.DenyAll(cfg.Logger)
	}
	srv := httpd.New(httpd.Config{
		Port:      "0",
		Health:    true,
		Authn:     authn,
		Authorize: authz,
	})
	if err := appkit.MountServer(srv.Router(), cfg); err != nil {
		t.Fatalf("MountServer: %v", err)
	}
	h, err := srv.Handler()
	if err != nil {
		t.Fatalf("Handler: %v", err)
	}
	ts := httptest.NewServer(h)
	t.Cleanup(ts.Close)
	return ts
}

func post(t *testing.T, url, body string) (int, string) {
	t.Helper()
	req, _ := http.NewRequest("POST", url, bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST %s: %v", url, err)
	}
	defer resp.Body.Close()
	out, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(out)
}

func TestMCPClosedByDefault(t *testing.T) {
	ts := buildServer(t, appkit.ServerConfig{}, nil)
	status, body := post(t, ts.URL+mcp.MCPPath, `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`)
	if status != http.StatusOK {
		t.Fatalf("tools/list: status %d, body %s", status, body)
	}
	if !strings.Contains(body, `"tools":[]`) {
		t.Fatalf("tools/list must list zero tools, got %s", body)
	}
}

func TestMCPOpensExposedTools(t *testing.T) {
	ts := buildServer(t, appkit.ServerConfig{
		Operations: []router.OperationModule{fakeOps{}},
		MCPTools:   []mcp.ToolName{mcp.ToolNameOf("fake", "list")},
		Authorize:  allowFakeRead,
	}, fakeAuthn("test-user"))
	status, body := post(t, ts.URL+mcp.MCPPath, `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`)
	if status != http.StatusOK {
		t.Fatalf("tools/list: status %d, body %s", status, body)
	}
	if !strings.Contains(body, `"fake.list"`) {
		t.Fatalf("tools/list must contain fake.list, got %s", body)
	}
}

func TestRPCMounted(t *testing.T) {
	ts := buildServer(t, appkit.ServerConfig{
		Operations: []router.OperationModule{fakeOps{}},
		Authorize:  allowFakeRead,
	}, fakeAuthn("test-user"))
	path := ts.URL + rpc.DefaultPrefix + "/fake/list"
	status, _ := post(t, path, `{}`)
	if status != http.StatusOK {
		t.Fatalf("with session and authorize allowing fake:r, want 200, got %d", status)
	}
}

func TestAnonymousDenied(t *testing.T) {
	ts := buildServer(t, appkit.ServerConfig{
		Operations: []router.OperationModule{fakeOps{}},
		Authorize:  allowFakeRead,
	}, nil)
	path := ts.URL + rpc.DefaultPrefix + "/fake/list"
	status, body := post(t, path, `{}`)
	if status != http.StatusForbidden {
		t.Fatalf("without session, want 403, got %d (body: %s)", status, body)
	}
}

func TestDefaultDenyWithoutAuthorize(t *testing.T) {
	logger := func(a ...any) {
		line := fmt.Sprint(a...)
		t.Log(line)
	}

	ts := buildServer(t, appkit.ServerConfig{
		Operations: []router.OperationModule{fakeOps{}},
		Logger:     logger,
	}, fakeAuthn("test-user"))
	path := ts.URL + rpc.DefaultPrefix + "/fake/list"
	status, _ := post(t, path, `{}`)
	if status != http.StatusForbidden {
		t.Fatalf("without authorize, want 403, got %d", status)
	}
}

func TestIntrospectionRequiresAPIExplorer(t *testing.T) {
	ts := buildServer(t, appkit.ServerConfig{
		Operations: []router.OperationModule{fakeOps{}},
		Authorize:  allowFakeRead,
	}, nil)
	resp, err := http.Get(ts.URL + router.IntrospectionPath)
	if err != nil {
		t.Fatalf("GET %s: %v", router.IntrospectionPath, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("GET %s without api_explorer:r permission: want 403, got %d", router.IntrospectionPath, resp.StatusCode)
	}
}

func TestRegisterMountsAppRoutes(t *testing.T) {
	ts := buildServer(t, appkit.ServerConfig{
		Authorize: allowFakeRead,
		Register: func(r router.Router) {
			r.Get("/ping", func(ctx router.Context) {
				ctx.Write([]byte("ok"))
			}).Public()
		},
	}, nil)
	resp, err := http.Get(ts.URL + "/ping")
	if err != nil {
		t.Fatalf("GET /ping: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /ping: status %d, body %s", resp.StatusCode, body)
	}
	if string(body) != "ok" {
		t.Fatalf("GET /ping body: want 'ok', got %q", string(body))
	}
}
