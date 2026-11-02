---
PLAN: "feat: appkit.App, appkit.Module and the server/migrate entry points — an app is one declaration"
TAG: v0.1.0
EXECUTOR: jules
REVIEWER: none
STATUS: running
SESSION: 4522725314813341400
---

> Read AGENTS.md at the repo root before any change.
> **Finish by yourself:** when the delivery check passes, commit, push and open the pull request. Never ask whether to
> proceed or finalize — nobody answers, and a session that waits ends without your work.

Commit after every stage. Never run `git reset`, `git restore`, `git checkout -- <file>` or `git stash` on your own work.
Stuck for 30 minutes on one stage: open the PR anyway, listing in its description what fails.

## Why

Today every app hand-writes the same server composition: open the database, build an id generator and an event
broker, construct each domain module with its own `Deps`, wire modules to each other with adapters, collect their
operations, start `httpd`, and keep a separate hand-written migration list plus a `cmd/migrate` binary. This plan
makes that one declaration:

```go
// what an app will write (config/app.go) — NOT part of this repo, shown for context
var App = appkit.App{
	Name: "mjosefa-cms", Tenant: "mjosefa-cms", Timezone: "America/Santiago",
	Open:    postgres.Open,
	Modules: []appkit.Module{servicecatalog.Module, staffmanager.Module, roomlayout.Module},
	Auth:    auth,
}
// web/server/main.go:  func main() { if err := appkit.Server(config.App); err != nil { log.Fatal(err) } }
// migrate/main.go:     func main() { if err := appkit.Migrate(config.App); err != nil { log.Fatal(err) } }
```

## Tools

```bash
go install webtyp.com/devflow/cmd/gotest@latest
go get webtyp.com/server@v0.2.105 webtyp.com/orm@v0.12.20 webtyp.com/storage@v0.1.18 webtyp.com/events@v0.1.0 \
  webtyp.com/unixid@v0.3.1 webtyp.com/env@v0.1.3 webtyp.com/ddl@v0.0.37 webtyp.com/model@v0.2.13 \
  webtyp.com/mcp@v0.3.20 webtyp.com/router@v0.4.17
go mod tidy
```

Environment: `GOPRIVATE=webtyp.com/*,github.com/veltylabs/*`, `GOFLAGS=-mod=mod`.

## Ecosystem search

- Searched `webtyp/*` and `veltylabs/*` for `App`, `Module`, `Deps`, `Provider`, `Find`, `Migrate`, `BuildServer`.
- **Reuse**: `appkit.MountServer` (this repo, `server.go`) mounts rpc, MCP, `/_routes`, `/_schema`; `httpd.New`
  serves; `ddl.Step` is the migration unit; `orm.New`, `unixid.NewUnixID`, `events/mock.Broker` (in-process broker).
- **Extend**: this repo. Each app's `config/server.go` (`mjosefa-cms`, `iam`) holds the code being moved here.
- **New**: none outside `appkit`. No other repo has an "app" type (`layout/app.BuildMenu` is the browser menu only).

## Design gate

1. **Prior art** (unverified in this session): Django `INSTALLED_APPS` + `settings.py`, Rails `config/application.rb`
   with engines, Laravel service providers — an app is a list of pieces plus a few values. We follow that; the module
   list is also the migration order, like Django's app order.
2. **Novice-name test**: `appkit.App`, `appkit.Module`, `appkit.Deps`, `appkit.Find`, `appkit.Server`,
   `appkit.Migrate`, `appkit.BuildServer` read as plain sentences ("find the module that gives me categories").
3. **Complexity ledger**: concepts +3 (`App`, `Module`, `Find`) / −4 (per-app `ServerConfig`, `BuildServer`,
   `MigrationSteps`, adapters); files an app touches to add a module 3 → 1; lines at the call site −300 per app;
   ways to do the same thing: 0 (the per-app composition is deleted by the follow-up app plans).
4. **Where it belongs**: `appkit` is the application kit (its README). The database driver stays with the app
   (`App.Open`), so `appkit` never imports `postgres`.
5. **What it deletes**: here, nothing (new capability). In the apps that adopt it: `config/server.go`,
   `config/migrate.go`, `cmd/migrate/`, `modules/server.go`, `modules/tenant.go`.

## APIs used

| Symbol | Exact signature | Tag | Notes |
|---|---|---|---|
| `httpd.New` | `func New(c httpd.Config) *httpd.Server` | server v0.2.105 | `webtyp.com/server/httpd` |
| `httpd.Config` | fields `Port, InternetPort, PublicDir string; Gzip, NoCache, Health bool; Logger func(...any); Authn router.Middleware; Authorize model.Authorizer; RoutesEndpoint bool; Policy model.PolicyDescriber` | server v0.2.105 | |
| `httpd.DefaultPort`, `httpd.DefaultPublicDir` | untyped string constants | server v0.2.105 | |
| `(*httpd.Server).Router` | `func (s *Server) Router() router.Router` | server v0.2.105 | |
| `(*httpd.Server).Handler` | `func (s *Server) Handler() (http.Handler, error)` | server v0.2.105 | validates RBAC: call once in BuildServer |
| `(*httpd.Server).ListenAndServe` | `func (s *Server) ListenAndServe() error` | server v0.2.105 | |
| `storage.Conn` | `interface { Executor; Compiler }` | storage v0.1.18 | satisfies `ddl.Execer` |
| `mem.New` | `func New() storage.Conn` | storage v0.1.18 | `webtyp.com/storage/mem`, tests only |
| `orm.New` | `func New(conn storage.Conn) *orm.DB` | orm v0.12.20 | |
| `unixid.NewUnixID` | `func NewUnixID() (*unixid.UnixID, error)` | unixid v0.3.1 | satisfies `model.IDGenerator` |
| `mock.Broker` | `type Broker struct{…}`; `(*Broker).Publish(events.Event)` | events v0.1.0 | `webtyp.com/events/mock`, in-process; use `&mock.Broker{}` |
| `events.Publisher` | `interface { Publish(e events.Event) }` | events v0.1.0 | |
| `env.Get` | `func Get(key string) string` | env v0.1.3 | |
| `env.Arg` | `func Arg(key string) string` | env v0.1.3 | `-key=value` from os.Args |
| `ddl.Step` | `struct { Name string; Run func(conn ddl.Execer, compiler ddl.Compiler) error }` | ddl v0.0.37 | |
| `ddl.Execer` / `ddl.Compiler` | `Exec(query string, args ...any) error` / `CompileDDL(s ddl.Stmt, m model.Model) (string, []any, error)` | ddl v0.0.37 | a postgres or mem `storage.Conn` also implements `ddl.Compiler`: check with a type assertion |
| `model.ModuleNaming` | `interface { ModelName() string }` | model v0.2.13 | |
| `model.IDGenerator` | `interface { NewID() string }` | model v0.2.13 | |
| `router.OperationModule` | `interface { model.ModuleNaming; MountOperations(reg router.OperationRegistry) }` | router v0.4.17 | |
| `router.APIModule` | `interface { model.ModuleNaming; MountAPI(r router.Router) }` | router v0.4.17 | |
| `appkit.MountServer`, `appkit.ServerConfig`, `appkit.DenyAll` | this repo, `server.go` | — | unchanged |

If a signature in the published tag differs from this table, stop and name the symbol in the PR; never adapt it by guessing.

## Stage 1 — `module.go`: `Module`, `Deps`, `Find`

New file `module.go`, exactly:

```go
//go:build !wasm

package appkit

import (
	"errors"
	"fmt"

	"webtyp.com/events"
	"webtyp.com/model"
	"webtyp.com/orm"
)

// ErrNoProvider and ErrManyProviders are what Find returns when the modules
// built so far hold zero or more than one value of the requested type.
var (
	ErrNoProvider    = errors.New("appkit: no module provides it — list the module that does before the one that needs it in App.Modules")
	ErrManyProviders = errors.New("appkit: more than one module provides it — keep one in App.Modules")
)

// Module is one domain module as an app lists it in App.Modules. appkit builds
// it with the shared Deps and migrates it, both in App.Modules order.
//
//	var Module = appkit.Module{Name: "staff_manager", Migrate: migrate.Migrate, Build: build}
type Module struct {
	Name    string                                             // identity: migration step name and error prefix
	Migrate func(conn ddl.Execer, compiler ddl.Compiler) error // nil: the module has no tables
	Build   func(d Deps) (model.ModuleNaming, error)           // nil: migrations only, nothing to serve
}

// Deps is what every module gets from appkit: the database, ids, events, the
// installation's tenant and time zone, and Find over the modules built before it.
type Deps struct {
	DB       *orm.DB
	IDs      model.IDGenerator
	Events   events.Publisher
	Tenant   string
	Timezone string

	built []model.ModuleNaming
}

// Find returns the one module built so far that satisfies T, so a module asks
// for what it needs by interface and never imports the module that has it.
//
//	cats, err := appkit.Find[roomlayout.CategorySource](d)
func Find[T any](d Deps) (T, error) {
	var found, zero T
	n := 0
	for _, m := range d.built {
		if v, ok := m.(T); ok {
			found = v
			n++
		}
	}
	switch n {
	case 0:
		return zero, fmt.Errorf("%w: %T", ErrNoProvider, &zero)
	case 1:
		return found, nil
	default:
		return zero, fmt.Errorf("%w: %T", ErrManyProviders, &zero)
	}
}
```

Add `"webtyp.com/ddl"` to the import block (it is used by `Module.Migrate`).

## Stage 2 — `app.go`: `App`, `Auth`

New file `app.go`:

```go
//go:build !wasm

package appkit

import (
	"webtyp.com/mcp"
	"webtyp.com/model"
	"webtyp.com/router"
	"webtyp.com/storage"
)

// App is the whole server side of an application: its name, its installation
// values, its database driver and the list of its modules, in order.
type App struct {
	Name     string                                 // MCP server name
	Tenant   string                                 // Deps.Tenant for every module
	Timezone string                                 // Deps.Timezone, e.g. "America/Santiago"
	Open     func(dsn string) (storage.Conn, error) // the app's database driver (postgres.Open); nil = no database
	Modules  []Module                               // build and migration order
	Auth     func(d Deps) (Auth, error)             // runs after every module is built; nil = anonymous, deny all
	MCPTools []mcp.ToolName                         // operations an agent may call; empty = MCP closed
	Register func(r router.Router)                  // the app's own routes; nil = none
	Logger   func(...any)                           // nil = discard
}

// Auth is what App.Auth returns: who the caller is and what they may do.
type Auth struct {
	Authn      router.Middleware
	Authorize  model.Authorizer // nil = deny all
	Policy     model.PolicyDescriber
	APIModules []router.APIModule // auth's own endpoints (login, me, logout)
}
```

## Stage 3 — `migrate.go`: `Steps`, `RunMigrations`, `Migrate`

New file `migrate.go`:

```go
//go:build !wasm

package appkit

import (
	"errors"
	"fmt"

	"webtyp.com/ddl"
	"webtyp.com/env"
)

// EnvDatabaseURL is the environment variable that holds the database address.
const EnvDatabaseURL = "DATABASE_URL"

var (
	ErrNoDatabaseURL = errors.New("appkit: DATABASE_URL is not set")
	ErrNoOpen        = errors.New("appkit: DATABASE_URL is set but App.Open is nil — give App the database driver (postgres.Open)")
	ErrNoCompiler    = errors.New("appkit: the database connection does not implement ddl.Compiler")
)

// Steps are the migrations of App.Modules, in that order; a module without
// Migrate has none.
func Steps(app App) []ddl.Step {
	var out []ddl.Step
	for _, m := range app.Modules {
		if m.Migrate != nil {
			out = append(out, ddl.Step{Name: m.Name, Run: m.Migrate})
		}
	}
	return out
}

// RunMigrations runs Steps(app) against conn, stopping at the first error.
func RunMigrations(app App, conn ddl.Execer, compiler ddl.Compiler) error {
	logger := loggerOf(app)
	for _, step := range Steps(app) {
		logger("migrate:", step.Name)
		if err := step.Run(conn, compiler); err != nil {
			return fmt.Errorf("migrate %s: %w", step.Name, err)
		}
	}
	return nil
}

// Migrate opens DATABASE_URL with App.Open and runs every module's migration.
// It is the whole body of an app's migrate/main.go.
func Migrate(app App) error {
	dsn := env.Get(EnvDatabaseURL)
	if dsn == "" {
		return ErrNoDatabaseURL
	}
	if app.Open == nil {
		return ErrNoOpen
	}
	conn, err := app.Open(dsn)
	if err != nil {
		return err
	}
	compiler, ok := conn.(ddl.Compiler)
	if !ok {
		return ErrNoCompiler
	}
	return RunMigrations(app, conn, compiler)
}

func loggerOf(app App) func(...any) {
	if app.Logger != nil {
		return app.Logger
	}
	return func(...any) {}
}
```

## Stage 4 — `serve.go`: `ServerEnv`, `BuildServer`, `Server`

New file `serve.go`:

```go
//go:build !wasm

package appkit

import (
	"fmt"

	eventsmock "webtyp.com/events/mock"
	"webtyp.com/env"
	"webtyp.com/orm"
	"webtyp.com/router"
	"webtyp.com/server/httpd"
	"webtyp.com/unixid"
)

// Process arguments the dev environment and the self-host supervisor pass to web/server.
const (
	ArgPort      = "server_port"
	ArgPublicDir = "server_public_dir"
	ArgNoCache   = "server_no_cache"
)

const msgNoDatabase = "appkit: DATABASE_URL is not set — starting without a database; modules are not built"

// ServerEnv is what the process tells the server: where to listen, what to
// serve and which database to open ("" = none).
type ServerEnv struct {
	Port      string
	PublicDir string
	NoCache   bool
	DSN       string
}

// BuildServer builds the whole server without listening: database, modules in
// App.Modules order, auth, and every contract MountServer mounts.
//
//	srv, err := appkit.BuildServer(config.App, appkit.ServerEnv{Port: "8080"})
func BuildServer(app App, se ServerEnv) (*httpd.Server, error) {
	logger := loggerOf(app)
	ids, err := unixid.NewUnixID()
	if err != nil {
		return nil, err
	}
	d := Deps{IDs: ids, Events: &eventsmock.Broker{}, Tenant: app.Tenant, Timezone: app.Timezone}

	if se.DSN == "" {
		logger(msgNoDatabase)
	} else {
		if app.Open == nil {
			return nil, ErrNoOpen
		}
		conn, err := app.Open(se.DSN)
		if err != nil {
			return nil, err
		}
		d.DB = orm.New(conn)
	}

	var ops []router.OperationModule
	var apis []router.APIModule
	if d.DB != nil {
		for _, m := range app.Modules {
			if m.Build == nil {
				continue
			}
			built, err := m.Build(d)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", m.Name, err)
			}
			d.built = append(d.built, built)
			if o, ok := built.(router.OperationModule); ok {
				ops = append(ops, o)
			}
			if a, ok := built.(router.APIModule); ok {
				apis = append(apis, a)
			}
		}
	}

	var au Auth
	if app.Auth != nil && d.DB != nil {
		if au, err = app.Auth(d); err != nil {
			return nil, fmt.Errorf("auth: %w", err)
		}
	}
	authorize := au.Authorize
	if authorize == nil {
		authorize = DenyAll(logger)
	}

	srv := httpd.New(httpd.Config{
		Port: se.Port, PublicDir: se.PublicDir, Gzip: true, NoCache: se.NoCache, Health: true,
		Logger: logger, Authn: au.Authn, Authorize: authorize, Policy: au.Policy,
	})
	if err := MountServer(srv.Router(), ServerConfig{
		Name:       app.Name,
		Authorize:  authorize,
		Policy:     au.Policy,
		Operations: ops,
		MCPTools:   app.MCPTools,
		APIModules: append(apis, au.APIModules...),
		Register:   app.Register,
		Migrations: Steps(app),
		Logger:     logger,
	}); err != nil {
		return nil, err
	}
	if _, err := srv.Handler(); err != nil {
		return nil, err
	}
	return srv, nil
}

// Server reads the process arguments and DATABASE_URL, builds the server and
// listens. It is the whole body of an app's web/server/main.go.
func Server(app App) error {
	srv, err := BuildServer(app, processEnv())
	if err != nil {
		return err
	}
	return srv.ListenAndServe()
}

func processEnv() ServerEnv {
	se := ServerEnv{
		Port:      env.Arg(ArgPort),
		PublicDir: env.Arg(ArgPublicDir),
		NoCache:   env.Arg(ArgNoCache) == "true",
		DSN:       env.Get(EnvDatabaseURL),
	}
	if se.Port == "" {
		se.Port = httpd.DefaultPort
	}
	if se.PublicDir == "" {
		se.PublicDir = httpd.DefaultPublicDir
	}
	return se
}
```

Anti-footgun: `server.go` (MountServer) keeps its `//go:build !browser` tag; the new files use `!wasm` because they open a
database and listen. Do not change `server.go`.

## Stage 5 — tests (`tests/app_test.go`, external package `tests`)

Fakes (write them in the test file):

```go
type providerMod struct{}                       // provides categories
func (providerMod) ModelName() string           { return "provider" }
func (providerMod) Categories() []string        { return []string{"a"} }

type CategorySource interface{ Categories() []string }

type consumerMod struct{ src CategorySource }
func (consumerMod) ModelName() string { return "consumer" }

var calls []string // order of Build/Migrate calls, reset per test
func mod(name string, build func(appkit.Deps) (model.ModuleNaming, error)) appkit.Module {
	return appkit.Module{
		Name:    name,
		Migrate: func(ddl.Execer, ddl.Compiler) error { calls = append(calls, "migrate:"+name); return nil },
		Build:   build,
	}
}
func openMem(string) (storage.Conn, error) { return mem.New(), nil }
```

| Test | Setup | Call | Expected |
|---|---|---|---|
| `TestFind_OneProvider` | consumer's Build does `src, err := appkit.Find[CategorySource](d)` and returns `consumerMod{src}`; `App{Open: openMem, Modules: [provider, consumer]}` | `appkit.BuildServer(app, appkit.ServerEnv{DSN: "mem"})` | `err == nil`; the consumer got `src.Categories()[0] == "a"` (capture it in a test variable) |
| `TestFind_NoProvider` | same, Modules `[consumer]` only | `BuildServer` | `errors.Is(err, appkit.ErrNoProvider)`; `err.Error()` starts with `consumer: ` |
| `TestFind_ProviderAfterConsumer` | Modules `[consumer, provider]` | `BuildServer` | `errors.Is(err, appkit.ErrNoProvider)` (order matters) |
| `TestFind_TwoProviders` | Modules `[provider, provider2, consumer]` (`provider2` another type with `Categories()`) | `BuildServer` | `errors.Is(err, appkit.ErrManyProviders)` |
| `TestBuildServer_NoDSN` | Modules `[provider]` with a Build that appends `"build:provider"` to `calls` | `BuildServer(app, ServerEnv{})` | `err == nil`; `calls` is empty (no module built without a database) |
| `TestBuildServer_DSNWithoutOpen` | `App{}` | `BuildServer(app, ServerEnv{DSN: "x"})` | `errors.Is(err, appkit.ErrNoOpen)` |
| `TestBuildServer_AuthSeesModules` | `Auth` does `appkit.Find[CategorySource](d)` and returns `appkit.Auth{}` | `BuildServer` with Modules `[provider]` | `err == nil` and Auth found the provider |
| `TestSteps_Order` | Modules `[a, b]` built with `mod`, plus `appkit.Module{Name: "c"}` (no Migrate) | `appkit.Steps(app)` | names `["a", "b"]` in that order |
| `TestRunMigrations_Order` | Modules `[a, b]` | `appkit.RunMigrations(app, mem.New(), nil)` | `calls == ["migrate:a", "migrate:b"]` |
| `TestRunMigrations_StopsAtError` | `a`'s Migrate returns `errors.New("boom")`, then `b` | `RunMigrations` | error text `migrate a: boom`; `calls` has no `migrate:b` |
| `TestMigrate_NoDatabaseURL` | `t.Setenv("DATABASE_URL", "")` | `appkit.Migrate(app)` | `errors.Is(err, appkit.ErrNoDatabaseURL)` |
| `TestMigrate_NoOpen` | `t.Setenv("DATABASE_URL", "x")`, `App{}` | `appkit.Migrate(app)` | `errors.Is(err, appkit.ErrNoOpen)` |

The `mem` connection passed to `RunMigrations` is only received by the fake `Migrate`, which ignores it.

## Stage 6 — documentation

`README.md` (≤ 150 lines), these four sections in this order:

1. `## Why This Exists` — two lines, then a Without/With table: *Without*: each app writes `config/server.go`
   (database, ids, events, module constructors, adapters between modules, httpd, MountServer) plus
   `config/migrate.go` and `cmd/migrate/main.go`. *With*: one `appkit.App` literal; `web/server/main.go` and
   `migrate/main.go` are one line each. Link [`Module`](module.go), [`App`](app.go), [`Find`](module.go).
2. `## Before You Start` — the database driver is the app's (`App.Open`); modules are built and migrated in
   `App.Modules` order; a module that needs another lists it after it.
3. `## Step-by-Step Guide` — complete code: (a) a module exporting `var Module = appkit.Module{Name, Migrate, Build}`
   whose `Build` uses `appkit.Find[CategorySource](d)`; (b) `config/app.go` with `appkit.App{…}`; (c) the two
   one-line mains of the Why section. Keep the existing `ServerConfig` table below as `MountServer` reference.
4. `## Documentation Map` — table: `docs/ARCHITECTURE.md` (design), `module.go`, `app.go`, `serve.go`, `migrate.go`.

`docs/ARCHITECTURE.md`: add a section `## App and Module` stating: build order = `App.Modules` order; `Find` sees only
modules built before; no database → no module built (the server still serves static files and `/_routes`);
`App.Auth` runs after every module so it can `Find` them; migrations are `Steps(app)`, the same list `/_schema` draws.

## Delivery check

```bash
gotest
grep -n "postgres" *.go go.mod   # empty: appkit never imports a database driver
```

A PR that does not compile, or whose tests could not run, is a failed delivery.

## Stages

| # | Stage | Files |
|---|---|---|
| 1 | Module, Deps, Find | `module.go` |
| 2 | App, Auth | `app.go` |
| 3 | Steps, RunMigrations, Migrate | `migrate.go` |
| 4 | ServerEnv, BuildServer, Server | `serve.go` |
| 5 | Tests | `tests/app_test.go` |
| 6 | Documentation | `README.md`, `docs/ARCHITECTURE.md` |
