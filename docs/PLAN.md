---
PLAN: "feat!: appkit is production only — migration and dev tools leave; Server/BuildServer take mounts; RunBrowser → Browser(app, tools)"
EXECUTOR: jules
REVIEWER: none
STATUS: running
SESSION: 12412985251714415200
---

> Read AGENTS.md at the repo root before any change.
> **Finish by yourself:** when the delivery check passes, commit, push and open the pull request. Never ask whether to
> proceed or finalize — nobody answers, and a session that waits ends without your work.

# Plan — appkit keeps only what an app ships to production

## Tools

```bash
go install webtyp.com/devflow/cmd/gotest@latest
```

Environment: `GOPRIVATE=webtyp.com/*,github.com/veltylabs/*`, `GOFLAGS=-mod=mod`, `go mod tidy` at the end.

## Why

`appkit` is the production declaration of an app (rule in project-layout: development operations never live here).
Today it also (1) builds and runs migrations (`migrate.go`: `Steps`, `RunMigrations`, `Migrate`) — moving to the new
repo `webtyp.com/migrate`; and (2) mounts development tools on every server: the database diagram
(`server.go:55-61`, `erd`) and the API introspection (`server.go:53`, `router.MountIntrospection`), and always puts
«API explorer» and «Database» in the browser menu (`run_browser.go:139`). Those move to the new repo
`webtyp.com/devtools`, compiled only with `-tags dev`. appkit gets one explicit entry for them: the app's `main`
passes them in. The generated mains become (written by another plan, in `webtyp/server`):

```go
// web/server/main.go
func main() { if err := appkit.Server(config.App, devtools.Server(config.App)...); err != nil { log.Fatal(err) } }
// web/browser/main.go
func main() { appkit.Browser(config.App, devtools.Menu()) }
```

## Design gate

1. Prior art (unverified): Rails mounts `web-console` only in the development group; Django adds `debug_toolbar`
   only in development settings; Spring Boot DevTools is excluded from the packaged jar. The tool is left out of the
   deliverable, not hidden behind a permission.
2. Names: `appkit.Server` / `appkit.Browser` read like their folders (`web/server`, `web/browser`) and like
   `ModuleServer`/`ModuleBrowser`, `BrowserDeps`. `RunBrowser` → `Browser` («Run» is not in `Server` either).
3. Ledger: exported −6 (`Steps`, `RunMigrations`, `Migrate`, `ErrNoDatabaseURL`, `ErrNoCompiler`,
   `ServerConfig.Migrations`), renamed 1 (`RunBrowser`), +1 field (`ServerConfig.Mounts`), +1 parameter each on
   `Server`, `BuildServer`, `Browser`. Ways to mount a dev tool: 1 (the mounts).
4. Belongs here: the entry points are appkit's; what is mounted is decided by the app's main.
5. Deletes: the list in 3, `erd.Mount` and `router.MountIntrospection` calls, the fixed `appmenu.Tools{...}`, the
   `webtyp.com/erd` requirement in `go.mod`.

## The contract (exactly these signatures)

```go
// server side (files without the wasm tag, as today)
func Server(app App, mounts ...func(r router.Router, policy model.PolicyDescriber) error) error
func BuildServer(app App, se ServerEnv, mounts ...func(r router.Router, policy model.PolicyDescriber) error) (*httpd.Server, error)
type ServerConfig struct {
	// … every field of today except Migrations …
	Mounts []func(r router.Router, policy model.PolicyDescriber) error // extra routes the app's main passes (dev tools); nil = none
}
// browser side
func Browser(app App, tools appmenu.Tools) // appmenu = webtyp.com/layout/app; appmenu.Tools{} = no tools group
```

## Stage 1 — migration leaves (`migrate.go`)

- Delete `Steps`, `RunMigrations`, `Migrate`, `ErrNoDatabaseURL`, `ErrNoCompiler`.
- Keep `EnvDatabaseURL`, `ErrNoOpen`, `ErrNoDatabase`, `loggerOf` (the server and `BuildModules` use them): move them
  to a new `env.go` (`//go:build !wasm`) and delete `migrate.go`.

## Stage 2 — dev tools leave the server (`server.go`, `serve.go`)

- `ServerConfig`: delete `Migrations`; add `Mounts` (contract above).
- `MountServer`: delete the `router.MountIntrospection(...)` line (`server.go:53`) and the `if len(cfg.Migrations) > 0 {…}`
  block (`server.go:55-61`). After mounting `cfg.APIModules` and before `cfg.Register`, add:
  ```go
	for _, mount := range cfg.Mounts {
		if err := mount(r, cfg.Policy); err != nil {
			return err
		}
	}
  ```
- `BuildServer(app, se, mounts...)`: in its `ServerConfig{…}` delete `Migrations: Steps(app)` and set `Mounts: mounts`.
- `Server(app, mounts...)`: `srv, err := BuildServer(app, processEnv(), mounts...)`.
- Update the doc comments: «It is the whole body of an app's web/server/main.go» with the new call.

## Stage 3 — the browser entry (`run_browser.go`)

- `RunBrowser(app App)` → `Browser(app App, tools appmenu.Tools)`; `buildSession(app, caller)` →
  `buildSession(app, caller, tools)`; line 139 passes `tools` instead of `appmenu.Tools{API: apiexplorer.FromServer(),
  Database: schemaview.FromServer()}`; drop the `apiexplorer` and `schemaview` imports.
- Every other call of `buildSession` (tests included) passes `appmenu.Tools{}`.

## Stage 4 — tests

- `tests/app_test.go`, `tests/login_test.go`: delete the tests of `Steps`, `RunMigrations`, `Migrate` (they move to
  `webtyp/migrate`).
- `tests/server_test.go`: delete `TestIntrospectionRequiresAPIExplorer`, `TestIntrospectionOpensWithItsPermission` and
  the schema test that uses `ServerConfig{Migrations: …}` (`~line 241`) — they move to `webtyp/devtools`. Add:

| Test | Setup → call | Expected |
|---|---|---|
| `TestMountsReceiveRouterAndPolicy` | `ServerConfig{Policy: p, Mounts: {func(r, pol) error { r.GET("/x", …); got = pol; return nil }}}` → `buildRouter` + `get(r, "/x")` (use the router method the other tests use to add a route; if none, assert only `got == p`) | mount called once; `got == p` |
| `TestMountErrorStopsServer` | `Mounts: {func(router.Router, model.PolicyDescriber) error { return errors.New("boom") }}` → `MountServer` | error contains `"boom"` |
| `TestNoDevToolsByDefault` | `ServerConfig{Authorize: allow-all}` → `get(r, router.IntrospectionPath)`, `get(r, "/_schema")` | both 404 |

## Stage 5 — docs

`README.md` and `docs/ARCHITECTURE.md`: appkit is production only; `Server(app, mounts...)`, `Browser(app, tools)`,
`ServerConfig.Mounts`; migrations are `webtyp.com/migrate`, dev tools are `webtyp.com/devtools`; the two generated
mains above. Remove every mention of `Steps`, `RunMigrations`, `Migrate`, `RunBrowser`, `/_schema` served by appkit.

## Delivery check

1. `gotest` passes. A PR that does not compile, or whose tests could not run, is a failed delivery.
2. `grep -rn "RunBrowser\|RunMigrations\|func Steps\|func Migrate\|ErrNoDatabaseURL\|ErrNoCompiler\|Migrations\|erd\.\|MountIntrospection\|apiexplorer\|schemaview" --include=*.go --include=*.md .` → empty.
3. `grep -n "webtyp.com/erd" go.mod` → empty.

Commit after every stage. Never run `git reset`, `git restore`, `git checkout -- <file>` or `git stash` on your own
work. Stuck 30 minutes: open the PR anyway listing what fails. If a symbol named here differs in the code, stop and
name it in the PR; never adapt it by guessing.

## Stages

| Stage | Files | Done when |
|---|---|---|
| 1 | `migrate.go` (deleted), `env.go` | compiles |
| 2 | `server.go`, `serve.go` | compiles |
| 3 | `run_browser.go` | compiles for wasm |
| 4 | `tests/*` | tests pass |
| 5 | `README.md`, `docs/ARCHITECTURE.md` | done |
