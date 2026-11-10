//go:build !wasm

package tests

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"webtyp.com/appkit"
	"webtyp.com/crud"
	"webtyp.com/mcp"
	"webtyp.com/model"
	"webtyp.com/router"
	"webtyp.com/router/mock"
	"webtyp.com/rpc"
	"webtyp.com/server/httpd"
)

type fakeModel struct{}

func (fakeModel) ModelName() string { return "fake" }
func (fakeModel) Schema() []model.Field {
	return []model.Field{
		{Name: "id", Type: model.Text(), DB: &model.FieldDB{PK: true}},
	}
}
func (fakeModel) Pointers() []any { return []any{} }
func (fakeModel) IsNil() bool { return false }
func (fakeModel) EncodeFields(w model.FieldWriter) {}
func (fakeModel) DecodeFields(r model.FieldReader) {}

type fakeModelList []*fakeModel

func (l *fakeModelList) Append() model.Fielder {
	m := &fakeModel{}
	*l = append(*l, m)
	return m
}
func (l *fakeModelList) At(i int) model.Fielder { return (*l)[i] }
func (l *fakeModelList) Len() int { return len(*l) }
func (l *fakeModelList) IsNil() bool { return l == nil }
func (l *fakeModelList) EncodeFields(w model.FieldWriter) {}
func (l *fakeModelList) DecodeFields(r model.FieldReader) {}

type fakeOps struct{}

func (fakeOps) ModelName() string { return "fake" }

func (fakeOps) Definition() model.ModuleDefinition {
	return model.ModuleDefinition{
		Name: "fake",
		Entities: []model.Definition{{
			Name:  "fake",
			Wire:  true,
			Allow: model.Read,
			Rules: model.Rules{Read: model.AddMyBusinessRule},
		}},
	}
}

func (fakeOps) MountOperations(reg crud.Registry) {
	reg.AddReadOnlyEntity(&fakeModel{}, nil, func(c crud.CurrentUser, args crud.ReadArgs, filter model.Model) (model.ModelSlice, error) {
		return &fakeModelList{}, nil
	})
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
	op := router.Operation{Module: "fake", Entity: "fake", Action: model.Read}
	r := buildRouter(t, appkit.ServerConfig{
		Operations: []crud.Module{fakeOps{}},
		MCPTools:   []mcp.ToolName{mcp.ToolNameOf(op)},
		Authorize:  allowFakeRead,
	}, fakeAuthn("test-user"))
	status, body := post(r, mcp.MCPPath, `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`)
	if status != 200 {
		t.Fatalf("tools/list: status %d, body %s", status, body)
	}
	if !strings.Contains(body, op.String()) {
		t.Fatalf("tools/list must contain %s, got %s", op.String(), body)
	}
}

func TestRPCMounted(t *testing.T) {
	r := buildRouter(t, appkit.ServerConfig{
		Operations: []crud.Module{fakeOps{}},
		Authorize:  allowFakeRead,
	}, fakeAuthn("test-user"))
	path := rpc.DefaultPrefix + "/fake/fake"
	status, _ := get(r, path)
	if status != 200 {
		t.Fatalf("with session and authorize allowing fake:r, want 200, got %d", status)
	}
}

func TestAnonymousDenied(t *testing.T) {
	r := buildRouter(t, appkit.ServerConfig{
		Operations: []crud.Module{fakeOps{}},
		Authorize:  allowFakeRead,
	}, nil)
	path := rpc.DefaultPrefix + "/fake/fake"
	status, body := get(r, path)
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
		Operations: []crud.Module{fakeOps{}},
		Logger:     logger,
	}, fakeAuthn("test-user"))
	path := rpc.DefaultPrefix + "/fake/fake"
	status, _ := get(r, path)
	if status != 403 {
		t.Fatalf("without authorize, want 403, got %d", status)
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

type fakePolicy struct{}

func (fakePolicy) Grants() []model.RoleGrant { return nil }

func allowAll(string, model.Resource, model.Action) bool { return true }

func TestMountsReceiveRouterAndPolicy(t *testing.T) {
	p := fakePolicy{}
	var got model.PolicyDescriber
	calls := 0
	r := buildRouter(t, appkit.ServerConfig{
		Policy: p,
		Mounts: []func(r router.Router, policy model.PolicyDescriber) error{
			func(r router.Router, pol model.PolicyDescriber) error {
				calls++
				got = pol
				r.Get("/x", func(ctx router.Context) {
					ctx.Write([]byte("ok"))
				}).Public()
				return nil
			},
		},
	}, nil)

	status, body := get(r, "/x")
	if status != 200 || body != "ok" {
		t.Fatalf("GET /x = %d, %s; want 200, ok", status, body)
	}
	if calls != 1 {
		t.Errorf("mount called %d times, want 1", calls)
	}
	if got != p {
		t.Errorf("got policy %v, want %v", got, p)
	}
}

func TestMountErrorStopsServer(t *testing.T) {
	r := &mock.Router{}
	err := appkit.MountServer(r, appkit.ServerConfig{
		Mounts: []func(r router.Router, policy model.PolicyDescriber) error{
			func(router.Router, model.PolicyDescriber) error {
				return errors.New("boom")
			},
		},
	})
	if err == nil {
		t.Fatal("MountServer on broken mount: want error, got nil")
	}
	if !strings.Contains(err.Error(), "boom") {
		t.Errorf("error = %q, want 'boom'", err.Error())
	}
}

func TestNoDevToolsByDefault(t *testing.T) {
	r := buildRouter(t, appkit.ServerConfig{Authorize: allowAll}, fakeAuthn("admin"))
	if status, _ := get(r, router.IntrospectionPath); status != 404 {
		t.Fatalf("GET %s = %d, want 404", router.IntrospectionPath, status)
	}
	if status, _ := get(r, "/_schema"); status != 404 {
		t.Fatalf("GET /_schema = %d, want 404", status)
	}
}

func TestMountServer_PassesTheBrokerToRPC(t *testing.T) {
	store := newMemStore()
	mod := tableModule{store: store}
	rec := &recorder{}
	mux := http.NewServeMux()
	r := httpd.NewRouter(mux)
	cfg := appkit.ServerConfig{
		Operations: []crud.Module{mod},
		Events:     rec,
		Public: []crud.Public{{
			Model:   &tableModel{},
			Actions: model.Create,
		}},
	}
	if err := appkit.MountServer(r, cfg); err != nil {
		t.Fatalf("MountServer: %v", err)
	}
	srv := httptest.NewServer(mux)
	defer srv.Close()

	body := `{"records":[{"name":"item1"}]}`
	resp, err := http.Post(srv.URL+rpc.DefaultPrefix+"/tmod/rec", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		t.Fatalf("POST status %d: %s", resp.StatusCode, respBody)
	}

	evs := rec.events()
	if len(evs) != 1 {
		t.Fatalf("expected 1 event, got %d", len(evs))
	}
	expectedTopic := crud.Topic("tmod", "rec", model.Create)
	if evs[0].Topic != expectedTopic {
		t.Fatalf("expected topic %s, got %s", expectedTopic, evs[0].Topic)
	}
}

func TestMountServer_PassesTheBrokerToMCP(t *testing.T) {
	store := newMemStore()
	mod := tableModule{store: store}
	rec := &recorder{}
	op := router.Operation{Module: "tmod", Entity: "rec", Action: model.Create}
	r := buildRouter(t, appkit.ServerConfig{
		Name:       "test-mcp",
		Operations: []crud.Module{mod},
		Events:     rec,
		MCPTools:   []mcp.ToolName{mcp.ToolNameOf(op)},
		Authorize:  mcp.AllowAll,
	}, fakeAuthn("admin"))

	toolCall := fmt.Sprintf(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":%q,"arguments":{"records":[{"name":"bar"}]}}}`, op.String())
	status, body := post(r, mcp.MCPPath, toolCall)
	if status != 200 {
		t.Fatalf("tool call status %d: %s", status, body)
	}

	evs := rec.events()
	if len(evs) != 1 {
		t.Fatalf("expected 1 event, got %d", len(evs))
	}
	expectedTopic := crud.Topic("tmod", "rec", model.Create)
	if evs[0].Topic != expectedTopic {
		t.Fatalf("expected topic %s, got %s", expectedTopic, evs[0].Topic)
	}
}

func TestMountServer_NilBrokerIsFine(t *testing.T) {
	store := newMemStore()
	mod := tableModule{store: store}
	mux := http.NewServeMux()
	r := httpd.NewRouter(mux)
	cfg := appkit.ServerConfig{
		Operations: []crud.Module{mod},
		Events:     nil,
		Public: []crud.Public{{
			Model:   &tableModel{},
			Actions: model.Create,
		}},
	}
	if err := appkit.MountServer(r, cfg); err != nil {
		t.Fatalf("MountServer: %v", err)
	}
	srv := httptest.NewServer(mux)
	defer srv.Close()

	body := `{"records":[{"name":"item1"}]}`
	resp, err := http.Post(srv.URL+rpc.DefaultPrefix+"/tmod/rec", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("POST status %d", resp.StatusCode)
	}
}

func TestMountServer_RestEndToEndOverHTTP(t *testing.T) {
	store := newMemStore()
	mod := tableModule{store: store}
	mux := http.NewServeMux()
	r := httpd.NewRouter(mux)
	cfg := appkit.ServerConfig{
		Operations: []crud.Module{mod},
		Public: []crud.Public{{
			Model:   &tableModel{},
			Actions: model.AllActions,
		}},
	}
	if err := appkit.MountServer(r, cfg); err != nil {
		t.Fatalf("MountServer: %v", err)
	}
	srv := httptest.NewServer(mux)
	defer srv.Close()

	caller := rpc.NewCaller(srv.URL)

	// Create
	var saved tableModel
	var createErr error
	caller.Call(router.Operation{Module: "tmod", Entity: "rec", Action: model.Create},
		&crud.CreateArgs{Records: []model.Model{&tableModel{Name: "item1"}}},
		&saved,
		func(err error) { createErr = err })
	if createErr != nil {
		t.Fatalf("Create: %v", createErr)
	}
	if saved.ID == "" || saved.Name != "item1" {
		t.Fatalf("saved record invalid: %+v", saved)
	}

	// Read by ID
	var list tableModelList
	var readErr error
	caller.Call(router.Operation{Module: "tmod", Entity: "rec", Action: model.Read},
		&crud.ReadArgs{ID: saved.ID},
		&list,
		func(err error) { readErr = err })
	if readErr != nil {
		t.Fatalf("Read: %v", readErr)
	}
	if len(list) != 1 || list[0].ID != saved.ID {
		t.Fatalf("expected 1 record with id %s, got: %+v", saved.ID, list)
	}

	// Update (PATCH)
	var updateErr error
	caller.Call(router.Operation{Module: "tmod", Entity: "rec", Action: model.Update},
		&crud.UpdateArgs{IDs: []string{saved.ID}, Fields: []string{"name"}, Record: &tableModel{Name: "item1-updated"}},
		nil,
		func(err error) { updateErr = err })
	if updateErr != nil {
		t.Fatalf("Update: %v", updateErr)
	}

	// Verify update
	var listAfterUpdate tableModelList
	caller.Call(router.Operation{Module: "tmod", Entity: "rec", Action: model.Read},
		&crud.ReadArgs{ID: saved.ID},
		&listAfterUpdate,
		func(err error) { readErr = err })
	if readErr != nil || len(listAfterUpdate) != 1 || listAfterUpdate[0].Name != "item1-updated" {
		t.Fatalf("update verification failed: %+v", listAfterUpdate)
	}

	// Delete
	var deleteErr error
	caller.Call(router.Operation{Module: "tmod", Entity: "rec", Action: model.Delete},
		&crud.DeleteArgs{IDs: []string{saved.ID}},
		nil,
		func(err error) { deleteErr = err })
	if deleteErr != nil {
		t.Fatalf("Delete: %v", deleteErr)
	}

	// Verify deleted
	var listAfterDelete tableModelList
	var notFoundErr error
	caller.Call(router.Operation{Module: "tmod", Entity: "rec", Action: model.Read},
		&crud.ReadArgs{ID: saved.ID},
		&listAfterDelete,
		func(err error) { notFoundErr = err })
	if notFoundErr == nil {
		t.Fatalf("expected 404 or not found, got nil error (list: %+v)", listAfterDelete)
	}

	// GET for an entity the module does not offer answers 404
	var nonExistentList tableModelList
	var nonExistentErr error
	caller.Call(router.Operation{Module: "tmod", Entity: "nonexistent", Action: model.Read},
		&crud.ReadArgs{},
		&nonExistentList,
		func(err error) { nonExistentErr = err })
	if nonExistentErr == nil {
		t.Fatal("expected 404 for nonexistent entity, got nil error")
	}
}

func TestCheckPublic_FindsTheRouteByMethodAndPath(t *testing.T) {
	store := newMemStore()
	mod := tableModule{store: store}
	r := &mock.Router{}
	cfg := appkit.ServerConfig{
		Operations: []crud.Module{mod},
		Public: []crud.Public{{
			Model:   &tableModel{},
			Actions: model.Read,
		}},
	}
	if err := appkit.MountServer(r, cfg); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	rBad := &mock.Router{}
	cfgBad := appkit.ServerConfig{
		Operations: []crud.Module{mod},
		Public: []crud.Public{{
			Model:   &fakeModel{},
			Actions: model.Read,
		}},
	}
	err := appkit.MountServer(rBad, cfgBad)
	if !errors.Is(err, appkit.ErrPublicNotMounted) {
		t.Fatalf("expected ErrPublicNotMounted, got %v", err)
	}
}

type createOnlyModel struct {
	ID string
}

func (m *createOnlyModel) ModelName() string   { return "cr" }
func (m *createOnlyModel) Allow() model.Action { return model.Create }
func (m *createOnlyModel) IsNil() bool         { return m == nil }
func (m *createOnlyModel) Schema() []model.Field {
	return []model.Field{{Name: "id", Type: model.Text(), DB: &model.FieldDB{PK: true}}}
}
func (m *createOnlyModel) Pointers() []any                  { return []any{&m.ID} }
func (m *createOnlyModel) EncodeFields(w model.FieldWriter) { w.String("id", m.ID) }
func (m *createOnlyModel) DecodeFields(r model.FieldReader) {
	if v, ok := r.String("id"); ok {
		m.ID = v
	}
}

type createOnlyModule struct {
	store crud.Store
}

func (m createOnlyModule) ModelName() string { return "crmod" }
func (m createOnlyModule) Definition() model.ModuleDefinition {
	return model.ModuleDefinition{
		Name: "crmod",
		Entities: []model.Definition{{
			Name:  "cr",
			Allow: model.Create,
			Rules: model.Rules{Create: model.HaveNoBusinessRule},
		}},
	}
}
func (m createOnlyModule) MountOperations(reg crud.Registry) {
	reg.AddStoredEntity(crud.StoredEntity{
		Model:   &createOnlyModel{},
		NewList: func() model.ModelSlice { return &tableModelList{} },
		Store:   m.store,
		Rules:   model.Rules{Create: model.HaveNoBusinessRule},
	})
}

func TestCheckPublic_AWrongMethodIsNotAMatch(t *testing.T) {
	store := newMemStore()
	mod := createOnlyModule{store: store}
	r := &mock.Router{}
	cfg := appkit.ServerConfig{
		Operations: []crud.Module{mod},
		Public: []crud.Public{{
			Model:   &createOnlyModel{},
			Actions: model.Read,
		}},
	}
	err := appkit.MountServer(r, cfg)
	if !errors.Is(err, appkit.ErrPublicNotMounted) {
		t.Fatalf("expected ErrPublicNotMounted, got %v", err)
	}
}
