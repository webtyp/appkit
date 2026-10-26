---
PLAN: "feat: appkit server — BuildServer/RunServer joining httpd, rpc, MCP (closed by default) and /_routes, deny by default"
EXECUTOR: jules
REVIEWER: none
STATUS: review
SESSION: 13875819739320607912
PR: https://github.com/webtyp/appkit/pull/1
REVIEWED_COMMIT: 607f724196e64a1776614fffe7cdc5b3d04025f1
---

> Read AGENTS.md at the repo root before any change.

# PLAN — `appkit`, slice 1: the server

## Why

Every WebTyp app writes the same server composition by hand: `veltylabs/mjosefa-cms/config/server.go`
(240 lines) and `veltylabs/iam/config/server.go`. `appkit` is the piece that joins the existing
libraries once; an app only configures it. This slice moves **only the server** (login session,
permissions and first admin come in later plans). The source is in `_temp/` (reference only):
`_temp/mjosefa-cms/config/{server.go,dev.go,prod.go,mcp.go,routes.go}`,
`_temp/mjosefa-cms/tests/{api_surface_test.go,setup_test.go,app_security_test.go}`,
`_temp/iam/config/server.go`. (`_temp/device_manager/` is for the next plan: leave it.)

## Tools

```
go install webtyp.com/devflow/cmd/gotest@latest
go get webtyp.com/env@v0.1.0 webtyp.com/events@v0.1.0 webtyp.com/fmt@v1.0.0 webtyp.com/mcp@v0.3.4 webtyp.com/model@v0.2.6 webtyp.com/orm@v0.12.10 webtyp.com/postgres@v0.4.16 webtyp.com/router@v0.4.6 webtyp.com/rpc@v0.2.3 webtyp.com/server@v0.2.86 webtyp.com/unixid@v0.3.1
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
5. **Deletes:** here, the `gonew` stub `appkit.go` (already removed). `_temp/` is deleted by the menu plan (the last of the two).

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

## Stage 4 — docs

- `README.md`: what `appkit` is (one sentence), the `ServerConfig` table, and this example:

```go
// web/server/main.go of an app
func main() {
	cfg := appkit.ReadEnv(appkit.ServerConfig{Name: "MJosefa CMS", Operations: ops, Authorize: authz})
	if err := appkit.RunServer(cfg); err != nil {
		log.Fatal(err)
	}
}
```
- Do **not** delete `_temp/` in this plan (the menu plan still needs `_temp/device_manager/`).

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
| 4 | `README.md` | server section written |

## Executor notes
- `cfg.IDs` is validated but its initialized fallback was removed since it is currently not used natively by `BuildServer`. Same with `cfg.Events`. The variables were unused, preventing the build.
