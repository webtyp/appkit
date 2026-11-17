//go:build !wasm && !browser

package tests

import (
	"errors"
	"strings"
	"testing"

	"webtyp.com/appkit"
	"webtyp.com/auth"
	"webtyp.com/crud"
	"webtyp.com/ddl"
	"webtyp.com/events/inprocess"
	"webtyp.com/mcp"
	"webtyp.com/model"
	"webtyp.com/orm"
	"webtyp.com/rbac"
	"webtyp.com/router"
	"webtyp.com/router/mock"
	"webtyp.com/rpc"
	"webtyp.com/storage"
	"webtyp.com/storage/mem"
	"webtyp.com/unixid"
)

func openMem(string) (storage.Conn, error) { return mem.New(), nil }

func newIDs() (model.IDGenerator, error) { return unixid.NewUnixID() }

// part is how the backend builds one module of the app.
func part(name string, build func(d appkit.Deps) (model.ModuleNaming, error)) appkit.BackendModule {
	return appkit.BackendModule{Name: name, Build: build}
}

func returns(v model.ModuleNaming) func(appkit.Deps) (model.ModuleNaming, error) {
	return func(appkit.Deps) (model.ModuleNaming, error) { return v, nil }
}

// kit is an app, its backend and its server, the way an app's config/ declares them.
func kit(parts ...appkit.BackendModule) (appkit.App, appkit.Backend, appkit.Server) {
	app := appkit.App{TenantID: "t1", Timezone: "America/Santiago", Log: func(...any) {}}
	for _, p := range parts {
		app.Modules = append(app.Modules, model.ModuleDefinition{Name: p.Name})
	}
	b := appkit.Backend{Events: &inprocess.Broker{}, API: rpc.JSON{}, Modules: parts}
	return app, b, appkit.Server{Database: openMem, NewIDs: newIDs}
}

func build(t *testing.T, app appkit.App, b appkit.Backend, s appkit.Server) *appkit.Built {
	t.Helper()
	built, err := appkit.Build(app, b, s, "mem", ddl.Schema{})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	return built
}

type namedMod struct{ name string }

func (m namedMod) ModelName() string                  { return m.name }
func (m namedMod) Definition() model.ModuleDefinition { return model.ModuleDefinition{Name: m.name} }
func (namedMod) MountOperations(crud.Registry)        {}

type categorySource interface{ Categories() []string }

type providerMod struct{ namedMod }

func (providerMod) Categories() []string { return []string{"a"} }

func TestFindBuiltModule_ReturnsTheModuleBuiltBefore(t *testing.T) {
	var got categorySource
	app, b, s := kit(
		part("provider", returns(providerMod{namedMod{"provider"}})),
		part("consumer", func(d appkit.Deps) (model.ModuleNaming, error) {
			m, err := appkit.FindBuiltModule(d, "provider")
			if err != nil {
				return nil, err
			}
			src, ok := m.(categorySource)
			if !ok {
				t.Fatalf("provider is %T", m)
			}
			got = src
			return namedMod{"consumer"}, nil
		}),
	)
	build(t, app, b, s)
	if got == nil || got.Categories()[0] != "a" {
		t.Fatalf("consumer did not get the provider's module, got %v", got)
	}
}

func TestFindBuiltModule_ProviderAfterConsumerNamesTheMissingOne(t *testing.T) {
	app, b, s := kit(
		part("consumer", func(d appkit.Deps) (model.ModuleNaming, error) {
			_, err := appkit.FindBuiltModule(d, "provider")
			return nil, err
		}),
		part("provider", returns(providerMod{namedMod{"provider"}})),
	)
	_, err := appkit.Build(app, b, s, "mem", ddl.Schema{})
	if !errors.Is(err, appkit.ErrNoModuleBuilt) || !strings.Contains(err.Error(), "provider") || !strings.HasPrefix(err.Error(), "consumer: ") {
		t.Fatalf("want ErrNoModuleBuilt naming provider, prefixed by consumer, got %v", err)
	}
}

func TestBuild_ModulesAreBuiltInAppOrder(t *testing.T) {
	var order []string
	mk := func(n string) appkit.BackendModule {
		return part(n, func(appkit.Deps) (model.ModuleNaming, error) { order = append(order, n); return namedMod{n}, nil })
	}
	app, b, s := kit(mk("a"), mk("b"), mk("c"))
	b.Modules = []appkit.BackendModule{mk("c"), mk("a"), mk("b")} // the list order of the parts does not decide
	build(t, app, b, s)
	if strings.Join(order, "") != "abc" {
		t.Fatalf("modules must be built in App.Modules order, got %v", order)
	}
}

func TestBuild_ModuleWithoutItsPartIsAnError(t *testing.T) {
	app, b, s := kit(part("a", returns(namedMod{"a"})))
	app.Modules = append(app.Modules, model.ModuleDefinition{Name: "orphan"})
	if _, err := appkit.Build(app, b, s, "mem", ddl.Schema{}); !errors.Is(err, appkit.ErrNoPart) || !strings.Contains(err.Error(), "orphan") {
		t.Fatalf("want ErrNoPart naming orphan, got %v", err)
	}
}

func TestBuild_PartOfAnUnlistedModuleIsAnError(t *testing.T) {
	app, b, s := kit(part("a", returns(namedMod{"a"})))
	b.Modules = append(b.Modules, part("stray", returns(namedMod{"stray"})))
	if _, err := appkit.Build(app, b, s, "mem", ddl.Schema{}); !errors.Is(err, appkit.ErrUnlistedPart) || !strings.Contains(err.Error(), "stray") {
		t.Fatalf("want ErrUnlistedPart naming stray, got %v", err)
	}
}

func TestBuild_PartWithoutBuildIsAnError(t *testing.T) {
	app, b, s := kit(appkit.BackendModule{Name: "a"})
	if _, err := appkit.Build(app, b, s, "mem", ddl.Schema{}); !errors.Is(err, appkit.ErrNoBuildFunc) {
		t.Fatalf("want ErrNoBuildFunc, got %v", err)
	}
}

func TestBuild_ModuleErrorIsNamed(t *testing.T) {
	app, b, s := kit(part("bad", func(appkit.Deps) (model.ModuleNaming, error) { return nil, errors.New("boom") }))
	if _, err := appkit.Build(app, b, s, "mem", ddl.Schema{}); err == nil || !strings.Contains(err.Error(), "bad: boom") {
		t.Fatalf("want an error containing 'bad: boom', got %v", err)
	}
}

type notCrud struct{}

func (notCrud) ModelName() string { return "x" }

func TestBuild_NeitherCrudNorPartsIsAnError(t *testing.T) {
	app, b, s := kit(part("x", returns(notCrud{})))
	if _, err := appkit.Build(app, b, s, "mem", ddl.Schema{}); !errors.Is(err, appkit.ErrNotCrudModule) {
		t.Fatalf("want ErrNotCrudModule, got %v", err)
	}
}

type withParts struct {
	namedMod
	parts []model.ModuleNaming
}

func (w withParts) Parts() []model.ModuleNaming { return w.parts }

func TestBuild_PartsAreFoundLikeModules(t *testing.T) {
	var seen model.ModuleNaming
	app, b, s := kit(
		part("people", returns(withParts{namedMod{"people"}, []model.ModuleNaming{namedMod{"users"}, namedMod{"roles"}}})),
		part("after", func(d appkit.Deps) (model.ModuleNaming, error) {
			var err error
			seen, err = appkit.FindBuiltModule(d, "roles")
			return namedMod{"after"}, err
		}),
	)
	build(t, app, b, s)
	if seen == nil || seen.ModelName() != "roles" {
		t.Fatalf("a part must be found by its name, got %v", seen)
	}
}

func TestBuild_DepsCarryTheInstallation(t *testing.T) {
	var d appkit.Deps
	app, b, s := kit(part("a", func(got appkit.Deps) (model.ModuleNaming, error) { d = got; return namedMod{"a"}, nil }))
	build(t, app, b, s)
	if d.Zone.Name() != "America/Santiago" || d.TenantID != "t1" || d.DB == nil || d.IDs == nil || d.Events == nil || d.Queue == nil || d.Log == nil {
		t.Fatalf("Deps incomplete: %+v", d)
	}
}

func TestBuild_WhatTheAppForgotIsNamed(t *testing.T) {
	cases := []struct {
		name   string
		change func(*appkit.App, *appkit.Backend, *appkit.Server)
		want   error
	}{
		{"no log", func(a *appkit.App, _ *appkit.Backend, _ *appkit.Server) { a.Log = nil }, appkit.ErrNoLog},
		{"no events", func(_ *appkit.App, b *appkit.Backend, _ *appkit.Server) { b.Events = nil }, appkit.ErrNoEvents},
		{"no api", func(_ *appkit.App, b *appkit.Backend, _ *appkit.Server) { b.API = nil }, appkit.ErrNoAPI},
		{"no ids", func(_ *appkit.App, _ *appkit.Backend, s *appkit.Server) { s.NewIDs = nil }, appkit.ErrNoIDs},
		{"no database driver", func(_ *appkit.App, _ *appkit.Backend, s *appkit.Server) { s.Database = nil }, appkit.ErrNoOpen},
		{"bad timezone", func(a *appkit.App, _ *appkit.Backend, _ *appkit.Server) { a.Timezone = "Mars/Base" }, appkit.ErrNoTimezone},
		{"unknown sign-in", func(a *appkit.App, _ *appkit.Backend, _ *appkit.Server) { a.SignIn = 99 }, appkit.ErrUnknownSignIn},
	}
	for _, c := range cases {
		app, b, s := kit(part("a", returns(namedMod{"a"})))
		c.change(&app, &b, &s)
		if _, err := appkit.Build(app, b, s, "mem", ddl.Schema{}); !errors.Is(err, c.want) {
			t.Errorf("%s: want %v, got %v", c.name, c.want, err)
		}
	}
}

func TestBuild_WithoutAddressBuildsNothingAndDeniesAll(t *testing.T) {
	built := false
	app, b, s := kit(part("a", func(appkit.Deps) (model.ModuleNaming, error) { built = true; return namedMod{"a"}, nil }))
	got, err := appkit.Build(app, b, s, "", ddl.Schema{})
	if err != nil {
		t.Fatal(err)
	}
	if built {
		t.Fatal("without a database address no module is built")
	}
	if got.Authorize("u", "fake", model.Read) {
		t.Fatal("without sign-in everything is denied")
	}
}

func TestBuild_NoSignInSaysSo(t *testing.T) {
	var lines []string
	app, b, s := kit(part("a", returns(namedMod{"a"})))
	app.Log = func(a ...any) { lines = append(lines, a[0].(string)) }
	got := build(t, app, b, s)
	if got.Authn != nil || got.Authorize("u", "fake", model.Read) {
		t.Fatal("no SignIn: no session middleware and every operation denied")
	}
	if len(linesWith(lines, "App.SignIn is empty")) != 1 {
		t.Fatalf("want one line saying nobody can enter, got %q", lines)
	}
}

func TestBuild_SignInByTrustedIPGivesTheSessionAndTheRoles(t *testing.T) {
	app, b, s := kit(part("a", returns(namedMod{"a"})))
	app.SignIn = appkit.SignInByTrustedIP
	got := build(t, app, b, s)
	if got.Authn == nil || got.Authorize == nil {
		t.Fatalf("sign-in must give Authn and Authorize: %+v", got)
	}
	if got.Authorize("nobody", "fake", model.Read) {
		t.Fatal("a person with no role may do nothing")
	}
}

func TestBuild_DefaultGrantsGoToTheAuthenticatedRole(t *testing.T) {
	conn := mem.New()
	app, b, s := kit(appkit.BackendModule{Name: "a", Build: returns(namedMod{"a"}), DefaultGrants: []model.Grant{{Resource: "fake", Actions: model.Read}}})
	s.Database = func(string) (storage.Conn, error) { return conn, nil }
	build(t, app, b, s)
	svc, err := rbac.New(orm.New(conn))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.GetRoleByCode("t1", rbac.RoleAuthenticated); err != nil {
		t.Fatalf("the authenticated role must exist with the module's grants: %v", err)
	}
}

func TestBuildModules_NeedsADatabase(t *testing.T) {
	app, b, s := kit()
	if _, err := appkit.BuildModules(app, b, s, nil); !errors.Is(err, appkit.ErrNoDatabase) {
		t.Fatalf("want ErrNoDatabase, got %v", err)
	}
}

func TestBuildModules_BuildsOnTheGivenDatabaseInOrder(t *testing.T) {
	var db *orm.DB
	app, b, s := kit(
		part("a", func(d appkit.Deps) (model.ModuleNaming, error) { db = d.DB; return namedMod{"a"}, nil }),
		part("b", func(d appkit.Deps) (model.ModuleNaming, error) {
			_, err := appkit.FindBuiltModule(d, "a")
			return namedMod{"b"}, err
		}),
	)
	tx := orm.New(mem.New())
	d, err := appkit.BuildModules(app, b, s, tx)
	if err != nil {
		t.Fatal(err)
	}
	if db != tx || d.DB != tx {
		t.Fatal("modules must be built on the transaction the import passes")
	}
	if _, err := appkit.FindBuiltModule(d, "b"); err != nil {
		t.Fatalf("the returned Deps must find the built modules: %v", err)
	}
}

func TestBuildModules_ErrorsAreNamed(t *testing.T) {
	app, b, s := kit(part("bad", func(appkit.Deps) (model.ModuleNaming, error) { return nil, errors.New("boom") }))
	if _, err := appkit.BuildModules(app, b, s, orm.New(mem.New())); err == nil || !strings.Contains(err.Error(), "bad: boom") {
		t.Fatalf("want 'bad: boom', got %v", err)
	}
	app.Timezone = "Mars/Base"
	if _, err := appkit.BuildModules(app, b, s, orm.New(mem.New())); !errors.Is(err, appkit.ErrNoTimezone) {
		t.Fatalf("want ErrNoTimezone, got %v", err)
	}
}

func TestApp_RenderHTMLNeedsAName(t *testing.T) {
	html := appkit.App{Name: "Acme"}.RenderHTML()
	if !strings.Contains(html, "login__card") {
		t.Fatalf("expected login__card, got %q", html)
	}
	if got := (appkit.App{}).RenderHTML(); got != "" {
		t.Fatalf("an app without a name draws nothing, got %q", got)
	}
}

// ---- Mount ----

// mounted builds the app and mounts it on a mock router that authenticates as `test-user` and allows only fake:read.
func mounted(t *testing.T, app appkit.App, b appkit.Backend, s appkit.Server, authn router.Middleware, mounts ...func(router.Router, model.PolicyDescriber) error) (*mock.Router, error) {
	t.Helper()
	built := build(t, app, b, s)
	r := &mock.Router{}
	r.Configure(mock.Config{Authn: authn, Authorize: allowFakeRead})
	err := built.Mount(r, mounts...)
	return r, err
}

func fakeKit() (appkit.App, appkit.Backend, appkit.Server) {
	return kit(part("fake", returns(fakeOps{})))
}

func TestMount_PublishesTheOperationsThroughTheAPI(t *testing.T) {
	app, b, s := fakeKit()
	r, err := mounted(t, app, b, s, fakeAuthn("test-user"))
	if err != nil {
		t.Fatal(err)
	}
	if status, _ := get(r, rpc.DefaultPrefix+"/fake/fake"); status != 200 {
		t.Fatalf("with a session and fake:r allowed, want 200, got %d", status)
	}
}

func TestMount_AnonymousIsDenied(t *testing.T) {
	app, b, s := fakeKit()
	r, err := mounted(t, app, b, s, nil)
	if err != nil {
		t.Fatal(err)
	}
	if status, body := get(r, rpc.DefaultPrefix+"/fake/fake"); status != 403 {
		t.Fatalf("without a session want 403, got %d (%s)", status, body)
	}
}

func TestMount_NoAgentsNoMCPEndpoint(t *testing.T) {
	app, b, s := fakeKit()
	r, err := mounted(t, app, b, s, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, route := range r.Routes() {
		if route.Path == mcp.MCPPath {
			t.Fatal("Backend.Agents is nil: no MCP endpoint must be mounted")
		}
	}
}

func TestMount_AgentsOpenOnlyTheAllowedTools(t *testing.T) {
	op := router.Operation{Module: "fake", Entity: "fake", Action: model.Read}
	app, b, s := fakeKit()
	b.Agents = mcp.Tools{Allowed: []mcp.ToolName{mcp.ToolNameOf(op)}}
	r, err := mounted(t, app, b, s, fakeAuthn("test-user"))
	if err != nil {
		t.Fatal(err)
	}
	status, body := post(r, mcp.MCPPath, `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`)
	if status != 200 || !strings.Contains(body, op.String()) {
		t.Fatalf("tools/list must contain %s, got %d %s", op.String(), status, body)
	}
}

func TestMount_AgentsWithNothingAllowedListNothing(t *testing.T) {
	app, b, s := fakeKit()
	b.Agents = mcp.Tools{}
	r, err := mounted(t, app, b, s, nil)
	if err != nil {
		t.Fatal(err)
	}
	status, body := post(r, mcp.MCPPath, `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`)
	if status != 200 || !strings.Contains(body, `"tools":[]`) {
		t.Fatalf("zero tools expected, got %d %s", status, body)
	}
}

func TestMount_PublicActionIsMountedPublic(t *testing.T) {
	app, b, s := kit(part("fake", returns(fakeTableOps{})))
	b.Public = []crud.Public{{Model: &fakeTableModel{}, Actions: model.Create}}
	r, err := mounted(t, app, b, s, nil)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, route := range r.Routes() {
		if route.Method == rpc.MethodOf(model.Create) && strings.HasSuffix(route.Path, "/"+(&fakeTableModel{}).ModelName()) {
			found = route.Access == model.AccessPublic
		}
	}
	if !found {
		t.Fatal("the public create must be mounted with AccessPublic")
	}
}

type otherModel struct{}

func (otherModel) ModelName() string { return "other" }

func TestMount_PublicActionNoModuleOffersIsAnError(t *testing.T) {
	app, b, s := fakeKit()
	b.Public = []crud.Public{{Model: otherModel{}, Actions: model.Read}}
	if _, err := mounted(t, app, b, s, nil); !errors.Is(err, rpc.ErrPublicNotMounted) {
		t.Fatalf("want rpc.ErrPublicNotMounted, got %v", err)
	}
}

func TestMount_TheAppsOwnRoutesAndExtraMountsAreMounted(t *testing.T) {
	app, b, s := fakeKit()
	b.Routes = func(r router.Router) {
		r.Get("/ping", func(ctx router.Context) { ctx.Write([]byte("ok")) }).Public()
	}
	calls := 0
	r, err := mounted(t, app, b, s, nil, func(r router.Router, _ model.PolicyDescriber) error {
		calls++
		r.Get("/x", func(ctx router.Context) { ctx.Write([]byte("x")) }).Public()
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if status, body := get(r, "/ping"); status != 200 || body != "ok" {
		t.Fatalf("GET /ping = %d %q", status, body)
	}
	if status, body := get(r, "/x"); status != 200 || body != "x" || calls != 1 {
		t.Fatalf("GET /x = %d %q, mount called %d times", status, body, calls)
	}
}

func TestMount_AnExtraMountErrorStopsTheMount(t *testing.T) {
	app, b, s := fakeKit()
	boom := errors.New("boom")
	_, err := mounted(t, app, b, s, nil, func(router.Router, model.PolicyDescriber) error { return boom })
	if !errors.Is(err, boom) {
		t.Fatalf("want boom, got %v", err)
	}
}

func linesWith(lines []string, part string) []string {
	var out []string
	for _, l := range lines {
		if strings.Contains(l, part) {
			out = append(out, l)
		}
	}
	return out
}

func TestMount_SignInBringsItsOwnRoutes(t *testing.T) {
	app, b, s := fakeKit()
	app.SignIn = appkit.SignInByTrustedIP
	r, err := mounted(t, app, b, s, nil)
	if err != nil {
		t.Fatal(err)
	}
	if status, _ := get(r, auth.PathMe); status == 404 {
		t.Fatalf("GET %s: the sign-in must mount it, got 404", auth.PathMe)
	}
	if !hasRoute(r, "POST", auth.PathLoginRUT) || !hasRoute(r, "POST", auth.PathLogout) {
		t.Fatalf("the sign-in must mount %s and %s", auth.PathLoginRUT, auth.PathLogout)
	}
}

func TestMount_WithoutSignInThereAreNoSessionRoutes(t *testing.T) {
	app, b, s := fakeKit()
	r, err := mounted(t, app, b, s, nil)
	if err != nil {
		t.Fatal(err)
	}
	if hasRoute(r, "POST", auth.PathLoginRUT) {
		t.Fatalf("an app without SignIn must not offer %s", auth.PathLoginRUT)
	}
}

// hasRoute says whether the router registered a route with that method and path.
func hasRoute(r *mock.Router, method, path string) bool {
	for _, info := range r.Routes() {
		if info.Method == method && info.Path == path {
			return true
		}
	}
	return false
}

// staffRegister is the staff register: the built module that says from which IP a person may enter and receives the
// first administrator's staff record.
type staffRegister struct {
	namedMod
	asked      []string // the IPs it was asked about
	registered []string // "code name ip" of every first administrator it received
}

func (s *staffRegister) IsTrustedIP(userID, ip string) bool {
	s.asked = append(s.asked, ip)
	return false
}

func (s *staffRegister) RegisterFirstAdmin(userID, code, name, ip string) error {
	s.registered = append(s.registered, code+" "+name+" "+ip)
	return nil
}

// fromLAN posts body to path as a request that comes from the local network at ip.
func fromLAN(r *mock.Router, path, ip, body string) int {
	ctx := &mock.Context{InMethod: "POST", InPath: path, InBody: []byte(body)}
	ctx.SetHeader("Content-Type", "application/json")
	ctx.SetValue(router.ContextKeyNetwork, router.NetworkLocal)
	ctx.SetValue(router.ContextKeyRemoteAddr, ip+":50000")
	r.Invoke("POST", path, ctx)
	return ctx.Status
}

func TestBuild_SignInUsesTheStaffRegister(t *testing.T) {
	staff := &staffRegister{namedMod: namedMod{"staff"}}
	app, b, s := kit(part("staff", returns(staff)))
	app.SignIn = appkit.SignInByTrustedIP
	r, err := mounted(t, app, b, s, nil)
	if err != nil {
		t.Fatal(err)
	}
	if status := fromLAN(r, auth.PathSetup, "10.0.0.5", `{"code":"12.345.678-5","name":"Ana"}`); status >= 400 {
		t.Fatalf("the first-run setup from the LAN: status %d", status)
	}
	if len(staff.registered) != 1 || staff.registered[0] != "12345678-5 Ana 10.0.0.5" {
		t.Fatalf("the first administrator must reach the staff register with the IP it set up from, got %v", staff.registered)
	}
	fromLAN(r, auth.PathLoginRUT, "10.0.0.9", `{"code":"12.345.678-5"}`)
	if len(staff.asked) == 0 || staff.asked[len(staff.asked)-1] != "10.0.0.9" {
		t.Fatalf("a login must ask the staff register about the caller's IP, asked %v", staff.asked)
	}
}
