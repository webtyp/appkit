---
PLAN: "feat: appkit server (deny by default, rpc, closed MCP, /_routes) and menu from view.Screen (permission per screen, groups only with a visible child)"
EXECUTOR: jules
REVIEWER: none
---

> Read AGENTS.md at the repo root before any change.

# PLAN — `appkit`, slice 1: the server and the menu from `view.Screen`

## Why

Every WebTyp app writes the same server composition by hand: `veltylabs/mjosefa-cms/config/server.go`
(240 lines) and `veltylabs/iam/config/server.go`. `appkit` is the piece that joins the existing
libraries once; an app only configures it. This slice moves **only the server** (login session,
permissions and first admin come in later plans). The source is in `_temp/` (reference only):
`_temp/mjosefa-cms/config/{server.go,dev.go,prod.go,mcp.go,routes.go}`,
`_temp/mjosefa-cms/tests/{api_surface_test.go,setup_test.go,app_security_test.go}`,
`_temp/iam/config/server.go`, `_temp/device_manager/ui/browser.go`.

## Tools

```
go install webtyp.com/devflow/cmd/gotest@latest
go get webtyp.com/env@v0.1.0 webtyp.com/events@v0.1.0 webtyp.com/fmt@v1.0.0 webtyp.com/mcp@v0.3.4 webtyp.com/model@v0.2.6 webtyp.com/orm@v0.12.10 webtyp.com/postgres@v0.4.16 webtyp.com/router@v0.4.6 webtyp.com/rpc@v0.2.3 webtyp.com/server@v0.2.86 webtyp.com/unixid@v0.3.1 webtyp.com/view@v0.6.44 webtyp.com/layout@v0.3.70 webtyp.com/dom@v0.13.29 webtyp.com/lang@v0.1.9
```

## Ecosystem search

Searched `webtyp/*` for `BuildServer`, `RunServer`, `denyAll`, `MountIntrospection`, `HarvestOps`
callers: only the two apps above compose these pieces; `webtyp/server` is the HTTP server itself
(`httpd`), `webtyp/app` is the dev environment. **New** repo (`appkit`), importing the owners.

## Design gate

1. **Prior art:** Django `INSTALLED_APPS` + `get_wsgi_application()`, Rails `Rails.application`,
   Uber fx `fx.New(...).Run()`: the app declares parts, the framework assembles and runs them.
2. **Novice-name test:** `appkit.ServerConfig`, `appkit.BuildServer(cfg)`, `appkit.RunServer(cfg)`.
3. **Complexity ledger:** concepts +1 (`appkit`); lines per app −~200 when adopted; ways to build a
   server −1 per app that adopts it (the app deletes its copy in its own plan).
4. **Where it belongs:** composition across `server`, `router`, `rpc`, `mcp` belongs to none of them.
5. **Deletes:** here, the `gonew` stub `appkit.go` (already removed) and `_temp/` (last stage).
   Apps delete `config/server.go` when they adopt it (not in this plan).

## APIs used

Read the exact signatures from the module cache after `go get` (e.g. `go doc webtyp.com/mcp HarvestOps`)
— the source in `_temp/mjosefa-cms/config/server.go` already calls every one of them with the
pinned versions above and compiles. Symbols: `httpd.New`, `httpd.Config`, `httpd.DefaultPort`,
`httpd.DefaultPublicDir`, `(*httpd.Server).Router/Handler/ListenAndServe`, `rpc.Mount`,
`rpc.DefaultPrefix`, `router.MountIntrospection`, `router.IntrospectionPath`, `mcp.HarvestOps`,
`mcp.NewServer`, `mcp.Config`, `mcp.ToolName`, `postgres.Open`, `orm.New`, `unixid.NewUnixID`,
`eventsmock.Broker` (`webtyp.com/events/mock`), `env.Arg`, `env.Get`.
If a signature differs from how `_temp` calls it, stop and name the symbol in the PR; never adapt
it by guessing.

## Stage 1 — `server.go` (`//go:build !wasm`, package `appkit`)

Port `_temp/mjosefa-cms/config/server.go` with exactly these changes:

```go
// ServerConfig is everything an app tells appkit about its server.
type ServerConfig struct {
	Name       string                     // the app's name; MCP server name
	Port       string                     // "" = httpd.DefaultPort
	PublicDir  string                     // "" = httpd.DefaultPublicDir
	NoCache    bool
	DSN        string                     // DATABASE_URL; "" = no database
	Logger     func(...any)
	OpenDB     func(dsn string) (*orm.DB, error) // nil = postgres.Open
	Operations []router.OperationModule   // published at POST /api/<module>/<op>
	MCPTools   []mcp.ToolName             // operations an agent may call; empty = MCP closed
	Authn      router.Middleware          // nil = anonymous
	Authorize  model.Authorizer           // nil = deny everything (and log it)
	APIModules []router.APIModule
	Register   func(r router.Router)      // the app's own routes; nil = none
	IDs        model.IDGenerator          // nil = unixid
	Events     events.Publisher           // nil = in-process broker
}

func BuildServer(cfg ServerConfig) (*httpd.Server, error)
func RunServer(cfg ServerConfig) error   // BuildServer + ListenAndServe
func ReadEnv(cfg ServerConfig) ServerConfig // fills Port (-server_port arg, else PORT env), PublicDir (-server_public_dir), NoCache (-server_no_cache=true), DSN (DATABASE_URL) when empty
```

- **Deleted vs the source:** `Providers` (the MCP provider is always `mcp.HarvestOps(cfg.MCPTools, cfg.Operations...)`:
  one path); `Router network.Gateway` and `defaultOperations` (the app passes `Operations`; nil =
  none); `DefaultServerConfig` (opening Postgres for the app's own deps stays in the app);
  `modules.TenantID` (becomes `cfg.Name`). `Register` is called where the source calls
  `Register(srv.Router())`.
- Keep the source's order inside `BuildServer`: validate Authorize → open DB → IDs/Events →
  MCP provider → `denyAll` when `Authorize` is nil → `mcp.NewServer` → `httpd.New` → `rpc.Mount` →
  `introspectionAccess(router.MountIntrospection(...))` → MCP `MountAPI` → `APIModules` →
  `Register` → `srv.Handler()` (fail fast on RBAC).
- Keep `denyAll` unexported. Log text stays as in the source but starts with `"appkit: "` instead
  of `"app: "`.

## Stage 2 — `dev.go` / `prod.go`

Port `_temp/mjosefa-cms/config/dev.go` (`//go:build !wasm && !prod`) and `prod.go`
(`//go:build !wasm && prod`) into package `appkit`, unchanged in behaviour: `validateAuthorize`,
`introspectionAccess`, exported `DevAllowAll()` (dev only). Add in `server.go`:
`const ResourceAPIExplorer model.Resource = "api_explorer"` (production `/_routes` requires
`ResourceAPIExplorer` read; the value equals `webtyp.com/layout/apiexplorer`'s `ModelName()`).

## Stage 3 — tests (`tests/`, package `tests`, `//go:build !wasm`)

Port from `_temp/mjosefa-cms/tests/` the cases below, using a fake `router.OperationModule` with
one read operation (`fake.list`, `Requires("fake", model.Read)`) instead of the clinic's modules,
and `httptest.NewServer(handler)` from `srv.Handler()`:

| Test | Setup | Expect |
|---|---|---|
| `TestMCPClosedByDefault` | `MCPTools` empty | `tools/list` on `POST /mcp` returns zero tools |
| `TestRPCMounted` | `Authorize` allows `fake:r`, `Authn` sets a user | `POST /api/fake/list` → 200 |
| `TestAnonymousDenied` | no `Authn` | `POST /api/fake/list` → 403 |
| `TestDefaultDenyWithoutAuthorize` | `Authorize` nil, `Authn` sets a user | → 403 and the logger received a line containing `DENY` |
| `TestIntrospectionListsRPCRoutes` | dev build | `GET /_routes` → 200, lists a route starting with `/api/` |
| `TestRegisterMountsAppRoutes` | `Register` adds `GET /ping` `.Public()` | `GET /ping` → 200 |

## Stage 4 — `menu.go` (`//go:build browser`): screens → menu

Modules now declare their screens as data with `webtyp.com/view` v0.6.44 (`view.Screen`,
`view.Lookup`, `Screen.Validate()` — read them with `go doc webtyp.com/view Screen`). `appkit`
turns them into the menu modules `platformd` draws. Reference for the leaf construction and the
lookup loading: `_temp/device_manager/ui/browser.go` (`onFirstActivation`, `fillOptions`).

```go
// Menu is what appkit hands to platformd: the modules and the visibility rule.
type Menu struct {
	Modules []platformd.UIModule
	CanView func(id string) bool // for platformd.Platform.CanView
}

// BuildMenu validates every screen and builds its platformd module. allows answers whether the
// signed-in user holds a permission (in an app: func(r model.Resource) bool { return profile.Allows(string(r)) }).
func BuildMenu(screens []view.Screen, ids model.IDGenerator, allows func(model.Resource) bool) (Menu, error)
```

Rules (write them exactly):

1. Call `s.Validate()` on every screen first; return its error unchanged.
2. **Leaf** → `v, err := crudview.New(crudview.Config{ParentID: s.ID, Presenter: s.Presenter, IDs: ids})`;
   module `platformd.NewUIModule(s.ID, lang.Translate(s.Label).String(), svg.Icon(s.Icon), v, activate)`
   where `activate` is `onFirstActivation(v, load)` copied from the reference, and `load` runs
   `fillOptions(v, l.Field, l.Source)` for each `l` in `s.Lookups`. If
   `s.Presenter.Description() != ""`, wrap with `platformd.Describe(m, s.Presenter.Description())`.
3. **Group** → `platformd.Describe(platformd.NewGroupUIModule(s.ID, lang.Translate(s.Label).String(), svg.Icon(s.Icon), children), s.Help)`
   with `children` built by rule 2.
4. **CanView**: a closure over a slice of `{id string; permission model.Resource}` pairs (no `map`:
   this code ships to WASM) for every leaf: returns `allows(permission)` for a leaf id; returns
   `false` for a group id and for unknown ids. `platformd` already shows a group only when one of its
   children is viewable (`platformd.go` `isViewable`), so a group with no viewable child is hidden.

## Stage 5 — menu tests (`tests/menu_test.go`, `//go:build browser`)

Use `view.New(&conformance.FakeLister{}, &conformance.MockRecord{})` (`webtyp.com/view/conformance`)
as presenters and `unixid`/a counter as `ids`.

| Test | Setup | Expect |
|---|---|---|
| `TestBuildMenu_InvalidScreenIsAnError` | a leaf without `Permission` | error containing `needs a Permission` |
| `TestBuildMenu_LeafVisibleByItsPermission` | leaf `x` with `Permission: "service"`; `allows` true only for `"service"` | `CanView("x")` true; with `allows` always false → false |
| `TestBuildMenu_GroupHasNoOwnPermission` | group `g` with leaf `g.a` | `CanView("g")` false; one module returned, it implements `platformd.GroupModule` with 1 child |
| `TestBuildMenu_DescriptionFromPresenter` | leaf whose presenter uses `view.WithDescription("Help")` | module implements `platformd.Described`, `Description() == "Help"` |

## Stage 6 — docs and cleanup

- `README.md`: what `appkit` is (one sentence), the `ServerConfig` table, a `BuildMenu` section (screens from modules → menu, permission per screen, groups hidden without a visible child), and this example:

```go
// web/server/main.go of an app
func main() {
	cfg := appkit.ReadEnv(appkit.ServerConfig{Name: "MJosefa CMS", Operations: ops, Authorize: authz})
	if err := appkit.RunServer(cfg); err != nil {
		log.Fatal(err)
	}
}
```
- Delete `_temp/`. Check: `test ! -e _temp`.

## Delivery check

```
GOPRIVATE=webtyp.com/*,github.com/veltylabs/* GOFLAGS=-mod=mod gotest
```

- A PR that does not compile, or whose tests could not run, is a failed delivery.
- Never pause to ask permission or report progress: when the check passes, commit, push and open
  the PR. Stuck for 30 minutes: open the PR anyway listing what fails.
- Commit after every stage (`git add -A` then `git commit -m "stage N: …"`); never `git reset`,
  `git restore`, `git checkout -- <file>` or `git stash` on your own work.

## Stages

| # | Files | Done when |
|---|---|---|
| 1 | `server.go` | compiles; no `Providers`, no `network` |
| 2 | `dev.go`, `prod.go` | `go vet -tags prod ./...` and default build pass |
| 3 | `tests/*_test.go` | six tests pass |
| 4 | `menu.go` | `BuildMenu` builds leaves, groups and `CanView` |
| 5 | `tests/menu_test.go` | four tests pass in the browser suite |
| 6 | `README.md`; delete `_temp/` | `test ! -e _temp` |
