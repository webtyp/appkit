---
PLAN: "refactor!: appkit is isomorphic (server code out of the browser binary, D44), shows the app through view.Layout (D50), and MountServer hands the app's event broker to rpc and mcp"
EXECUTOR: antigravity
REVIEWER: none
---

> Read AGENTS.md at the repo root before any change.
> **Finish by yourself:** when the delivery check passes, commit after every stage and stop. Never ask whether to
> proceed. This plan runs **locally**: do not tag, push or publish anything; the owner reviews first.
> Never run `git reset`, `git restore`, `git checkout -- <file>` or `git stash` on your own work.

## Goal

`crud` now publishes the change of every stored write (after the commit) on the broker it is given, and `rpc`/`mcp` take that broker. Three changes in one delivery (one plan per library): **(a)** the broker reaches the transports (stages 1 and 2); **(b)** `appkit` stops dragging server code into the browser binary: `AppServer`, `ModuleServer`, `Deps` and `Find` are `!browser`, with empty stand-ins under `browser` (decision D44, stage 4); **(c)** `appkit` shows the app through the layout contract of `view` instead of the layout importing `appkit`: `appkit.Browser(app, view.Layout)` builds a `view.Session` and gives it to the layout, `App.Brand` is a `view.Brand` and `appkit.Brand` is deleted (decision D50, stage 5). `layout` stops importing `appkit` in its own plan.

`appkit` is the composition root: it already builds one broker
for the modules (`Deps.Events`, `serve.go:124`); this plan gives the same broker to the transports so a write arriving by HTTP or by an agent announces itself. It also fixes the one place that
looked for a route by the old suffix (`checkPublic`, `server.go:93`).

## Ecosystem search

Searched `ServerConfig`, `rpc.Mount`, `mcp.HarvestOps`, `checkPublic`, `crud.Op` over `webtyp/*`: only `appkit/server.go` mounts the transports; `appkit/serve.go` owns the broker (`eventsmock.Broker`, which is the in-process `events.Publisher`/`events.Subscriber`).
**Extend** `appkit`; reuse `rpc.MethodOf` (rpc plan). No new library.

## Design gate

1. **Prior art**: none new (composition root wiring).
2. **Novice-name test**: `ServerConfig.Events` — "the broker writes announce on".
3. **Ledger**: concepts +1 (`ServerConfig.Events`); the app wires one broker once, instead of modules each publishing by hand.
4. **Where**: `appkit` composes; `crud` publishes; `events` is the contract.
5. **Deletes**: the `crud.Op` suffix match in `checkPublic`; the stdlib `strings` import (use `webtyp.com/fmt`, already imported by `server.go`); `appkit.Brand` (the type is `view.Brand`); the server types from the browser build.

Added by (b) and (c): concepts +2 (`appkit.Browser(app, layout)`, `appkit.NewSession(app, caller)`); the way an app is drawn: the layout imported `appkit` and rebuilt the session itself → one `view.Session` built once by `appkit`.

## Tools

```bash
go install webtyp.com/devflow/cmd/gotest@latest
```

Dependencies (all published, no `replace`): `go get webtyp.com/model@v0.2.18 webtyp.com/router@v0.4.21 webtyp.com/crud@v0.0.10 webtyp.com/storage@v0.2.1 webtyp.com/mcp@v0.3.24 webtyp.com/view@v0.6.71 webtyp.com/rpc@v0.2.21 && go mod tidy`. Prerequisite: `rpc` published (its REST plan).

## APIs used

| symbol | exact signature | file |
|---|---|---|
| `rpc.Mount` | `func Mount(r router.Router, prefix string, log func(...any), public []crud.Public, pub events.Publisher, modules ...crud.Module) error` | `rpc/mount.go` (rpc plan) |
| `rpc.Routes` | `func Routes(prefix string, public []crud.Public, modules ...crud.Module) ([]router.RouteInfo, error)` (unchanged) | `rpc/mount.go` |
| `rpc.MethodOf` | `func MethodOf(a model.Action) string` | `rpc/request.go` (rpc plan) |
| `mcp.HarvestOps` | `func HarvestOps(pub events.Publisher, expose []ToolName, modules ...crud.Module) (ToolProvider, error)` | `mcp/harvest.go` (mcp plan) |
| `crud.Public` | `struct{ Model model.ModuleNaming; Actions model.Action }` | `crud/contract.go` |
| `appkit.Deps.Events` | `events.Publisher` | `module_server.go:49` |
| `view.Brand` | `struct{ Name, LogoMark string }` | `view/session.go` |
| `view.Session` | `struct{ Brand Brand; Caller router.Caller; Zone time.Zone; Screens []Screen; Entry EntryScreens; Log func(...any) }` | `view/session.go` |
| `view.Layout` | `interface{ Show(Session) }` | `view/session.go` |
| `view.NewRecords` | `func NewRecords() *Records`; `(*Records).Caller(c router.Caller) router.Caller` | `view/records.go:34,37` |
| `rpc.NewCaller` | `func NewCaller(origin string) *Caller` (`origin == ""` = the page's own origin) | `rpc/caller.go:21` |
| `time.LoadZone` | `func LoadZone(name string) (Zone, error)` | `time/zone.go:36` |

If a signature differs from this table, stop and name the symbol in the PR; never adapt it by guessing.

## Test fixtures against `crud` v0.0.10 (a module fixture that implements `crud.Module` or `crud.Registry`)

`crud` v0.0.8, v0.0.9 and v0.0.10 changed the shapes below; every fixture and fake in this repo's tests follows them. The reference is `crud`'s own `tests/stored_entity_test.go` (`testModule`, `declareRules`): it is in the module cache at `webtyp.com/crud@v0.0.10/tests/` (`go list -m -f '{{.Dir}}' webtyp.com/crud`).

| Old | New (`crud` v0.0.10) |
|---|---|
| `crud.Caller` (hook and read arguments) | `crud.CurrentUser` (same field `UserID`) |
| `reg.Read(&Rec{}, nil, func(c crud.Caller, args crud.ReadArgs, f model.Model) (model.ModelSlice, error) {…})` | `reg.AddReadOnlyEntity(&Rec{}, nil, func(c crud.CurrentUser, args crud.ReadArgs, f model.Model) (model.ModelSlice, error) {…})` (same parameters, new name) |
| `reg.Table(crud.Table{…})` | `reg.AddStoredEntity(crud.StoredEntity{…})`; its hooks return `(crud.RuleAnswer, error)`: `return crud.ApplyAndPublish, nil` (an error: `return 0, err`); `Rules: model.Rules{…}` is required (`HaveNoBusinessRule` where a hook is nil, `AddMyBusinessRule` where it is set) |
| a `crud.Module` has `ModelName()` and `MountOperations(reg crud.Registry)` | it also has `Definition() model.ModuleDefinition`, listing **exactly** the entities it mounts, or `crud.Mount` refuses it (`ErrGeneratedOutOfDate`) |
| `crud.Op(&Rec{}, model.Read)` (a string) | `router.Operation{Module: "<ModelName()>", Entity: "<entity ModelName()>", Action: model.Read}` |
| `crud.Mount(reg, public, m)` | `crud.Mount(reg, public, nil, m)` (the new third argument is the `events.Publisher`; `nil` = nobody listens) |
| `crud.Loopback(m...)`, `crud.LoopbackActingAs(id, m...)` | `crud.Loopback(nil, m...)`, `crud.LoopbackActingAs(nil, id, m...)` |

`Definition()` for the two usual fixtures:

```go
// a read-only entity mounted with AddReadOnlyEntity (a Wire model, no table)
func (m myModule) Definition() model.ModuleDefinition {
	return model.ModuleDefinition{Name: m.ModelName(), Entities: []model.Definition{{
		Name: (&Rec{}).ModelName(), Wire: true, Allow: model.Read, Rules: model.Rules{Read: model.AddMyBusinessRule}}}}
}

// a stored entity mounted with AddStoredEntity: copy what the entity says about itself
func (m myModule) Definition() model.ModuleDefinition {
	return model.ModuleDefinition{Name: m.ModelName(), Entities: []model.Definition{{
		Name: m.entity.Model.ModelName(), Allow: m.entity.Model.(model.Allower).Allow(), Rules: m.entity.Rules}}}
}
```

There is no `model.BaseData` and no `crud.Table`/`crud.Op` any more: if a name is not in `crud` v0.0.10, `go doc webtyp.com/crud` lists what is.

## Stage 1 — tests first (red)

Edits in `tests/` (find the sites with `grep -rn "crud\.Operation\|crud\.Caller\|MountOperations" tests`):

| Where | Old | New |
|---|---|---|
| `server_test.go:50` | `reg.Read(&fakeModel{}, nil, func(c crud.Caller, args crud.ReadArgs, filter model.Model) …)` | `func(c crud.CurrentUser, …)` |
| fixtures `fakeModel`, `fakeTableModel` and any other module type in `tests/` | no `Definition()` | add it: `ModuleDefinition{Name: <ModelName()>, Entities: [<one entry per entity it mounts>]}` — `Wire: true, Allow: model.Read, Rules: model.Rules{Read: model.AddMyBusinessRule}` for a `reg.Read` entity; `Allow: <entity Allow()>, Rules: <HaveNoBusinessRule for each allowed action>` for a `reg.Table` entity (which also carries `Rules:`) |
| `infra_test.go:189` | `expectedSuffix := "/" + crud.Op(&fakeTableModel{}, model.Create)` | `expectedMethod := rpc.MethodOf(model.Create)` and `expectedSuffix := "/" + (&fakeTableModel{}).ModelName()`; the assertion compares route `Method` and the path suffix |

New tests:

| Test | setup → call → expected |
|---|---|
| `TestMountServer_PassesTheBrokerToRPC` | `ServerConfig{Operations: [tableModule], Events: rec}` with a `HaveNoBusinessRule` `Create` table over an in-memory store; mount on an `httpd` test router and `POST /api/<module>/<entity>` a record → `rec` received one event whose topic is `crud.Topic(<module>, <entity>, model.Create)` |
| `TestMountServer_PassesTheBrokerToMCP` | same config, `MCPTools: [router.Operation{…Create}.String() as mcp.ToolName]` → running the tool announces the same event |
| `TestMountServer_NilBrokerIsFine` | `Events: nil` → the write succeeds, nothing is published, no panic |
| `TestMountServer_RestEndToEndOverHTTP` | the end-to-end proof that `rpc` and the server compose, which `rpc`'s own tests no longer do (decision D44: `rpc` depends on no server): `MountServer` on an `httpd` server (this repo's `tests/` module, which may import `httpd`), a table module over an in-memory store, and an `rpc.Caller` pointed at it → create, read by id (`GET` with the query), update (`PATCH`) and delete (`DELETE`) through the real HTTP stack all succeed, and a `GET` for an entity the module does not offer answers 404 |
| `TestCheckPublic_FindsTheRouteByMethodAndPath` | `Public: [{Model: &fakeTableModel{}, Actions: model.Read}]` → `MountServer` returns nil; with `Public` naming an entity no module offers → `ErrPublicNotMounted` |
| `TestCheckPublic_AWrongMethodIsNotAMatch` | a module whose only public-looking route for the entity is a `POST` while `Public` asks for `Read` (`GET`) → `ErrPublicNotMounted` |

Added by (b) and (c) (`tests/session_test.go`, `tests/browser_build_test.go`):

| Test | setup → call → expected |
|---|---|
| `TestNewSession_CarriesWhatTheAppIs` | an `App{Timezone: "America/Santiago", Brand: view.Brand{Name: "acme"}, Log: l, Modules: [one with Browser.Screens and one with Browser.Entry]}` and a fake `router.Caller` → `NewSession(app, caller)` returns a `view.Session` with `Brand.Name == "acme"`, the zone `America/Santiago`, one screen per module screen in order, the `Entry` of the entry module and `Log` set |
| `TestNewSession_NoTimezoneIsAnError` | `App{}` without `Timezone` → `ErrNoTimezone`, never a default zone |
| `TestNewSession_ScreensShareOnePageOfRecords` | two screens that list through `Session.Caller`-based listers: what one saves the other sees (the caller handed to `Browser.Screens` is `view.NewRecords().Caller(caller)`) |
| `TestNewSession_TwoEntryModulesIsAnError` | two modules set `Browser.Entry` → `ErrTwoEntryModules` |
| `TestLayout_ShowReceivesTheSession` | a fake type `recordingLayout{got view.Session}` used through a `view.Layout` variable: `l.Show(session)` stores it (proves the contract an app and a layout exchange compiles against `appkit.NewSession`'s result) |
| `TestBrowserBuild_LeavesServerCodeOut` (`//go:build !wasm`) | runs `GOOS=js GOARCH=wasm go list -deps -tags browser -f '{{.ImportPath}}' ./...` in the module root → the output contains none of `webtyp.com/orm`, `webtyp.com/mcp`, `webtyp.com/rbac`, `webtyp.com/ddl`, `webtyp.com/server/httpd`, `webtyp.com/storage` |
| `TestApp_BrandIsViewBrand` | `var _ view.Brand = App{}.Brand` compiles and `grep -rn "type Brand" appkit/*.go` is empty (checked by the delivery check) |

`rec` is a `recorder` (`type recorder struct{ got []events.Event }`, `Publish` appends) in `tests/helpers_test.go`. Run `gotest`: it does not compile. Commit: `test: broker reaches the transports (red)`.

## Stage 2 — the code

`server.go`: add the field to `ServerConfig` (import `webtyp.com/events`):

```go
	Events      events.Publisher // the broker writes announce on (crud publishes after each commit); nil = nobody listens
```

and use it:

```go
	if len(cfg.Operations) > 0 {
		if err := rpc.Mount(r, rpc.DefaultPrefix, cfg.Logger, cfg.Public, cfg.Events, cfg.Operations...); err != nil {
			return err
		}
	}
	// …
	provider, err := mcp.HarvestOps(cfg.Events, cfg.MCPTools, cfg.Operations...)
```

Replace the body of `checkPublic` from `for _, p := range public {` on:

```go
	for _, p := range public {
		suffix := "/" + p.Model.ModelName()
		for _, act := range actions {
			if !p.Actions.Has(act) {
				continue
			}
			method := rpc.MethodOf(act)
			found := false
			for _, r := range routes {
				if r.Access == model.AccessPublic && r.Method == method && fmt.HasSuffix(r.Path, suffix) {
					found = true
					break
				}
			}
			if !found {
				return fmt.ErrType(fmt.Err(p.Model.ModelName()), ErrPublicNotMounted)
			}
		}
	}
	return nil
```

`serve.go` (`BuildServer`, the `MountServer(srv.Router(), ServerConfig{…})` call): add `Events: d.Events,`. `build_modules.go` (history import) keeps its own throwaway broker.
`screens.go` and the other `router.Caller` users do not change (the interface keeps its name). `gotest` passes. Commit: `refactor!: the broker reaches rpc and mcp`.

## Stage 4 — isomorphic: the server types leave the browser binary (D44)

1. `app_server.go` → first line `//go:build !browser` (it imports `crud`, `mcp`, `storage`, `router`, `model`). `module_server.go` → `//go:build !browser` (imports `orm`, `events`, `router`; it holds `ModuleServer`, `Entry`, `Parts`, `Deps`, `Find`).
2. New `app_server_browser.go` (`//go:build browser`): `// AppServer is empty in the browser: what only the server needs lives in app_server.go.` `type AppServer struct{}`; new `module_server_browser.go` (`//go:build browser`): `type ModuleServer struct{}`. `App.Server` and `Module.Server` keep compiling in both binaries and an app's `config/` fills them from a `!browser` file, as `App` already documents.
3. `serve.go` is `!wasm` and `server.go` is `!browser` already; `screens.go`, `app.go` stay untagged (they import only `router`, `time`, `view`).
4. `go vet` for both: `GOOS=js GOARCH=wasm go vet -tags browser ./...` and `go vet ./...` pass (`gotest` runs both). `TestBrowserBuild_LeavesServerCodeOut` is the proof.

Commit: `refactor!: server types are !browser (D44)`.

## Stage 5 — the app is shown through `view.Layout` (D50)

1. `app.go`: `Brand view.Brand` replaces `Brand Brand`; the comment example reads `view.Brand{…}`. `brand.go`: delete `type Brand` (keep the texts and `loadingHTML`); `tests/login_test.go` uses `view.Brand`.
2. New `session.go` (untagged; imports `router`, `view`):

```go
// NewSession builds what a layout draws: the app's brand, the connection to the server, the zone, the menu screens and the entry screens.
//
//	session, err := appkit.NewSession(config.App, rpc.NewCaller(""))
func NewSession(app App, caller router.Caller) (view.Session, error) {
	records := view.NewRecords().Caller(caller) // one page of records: what one screen saves, the others see
	screens, err := Screens(app, records)
	if err != nil {
		return view.Session{}, err
	}
	entry, err := EntryScreens(app, records)
	if err != nil {
		return view.Session{}, err
	}
	zone, err := time.LoadZone(app.Timezone)
	if err != nil {
		return view.Session{}, ErrNoTimezone
	}
	return view.Session{Brand: app.Brand, Caller: caller, Zone: zone, Screens: screens, Entry: entry, Log: app.Log}, nil
}
```

3. New `browser.go` (`//go:build browser`; imports `rpc`, `view`):

```go
// Browser is the whole body of an app's web/browser/main.go: it builds the session and gives it to the layout the app chose; it never returns.
//
//	func main() { appkit.Browser(config.App, layoutapp.New(devtools.Menu())) }
func Browser(app App, layout view.Layout) {
	session, err := NewSession(app, rpc.NewCaller(""))
	if err != nil {
		app.Log("appkit:", err) // App.Log is required (ErrNoLog), so this never panics
		return
	}
	layout.Show(session)
	select {}
}
```

`appkit` imports no layout and `layout` imports no `appkit`: `grep -rn "webtyp.com/layout" .` → empty, `go.mod` included.
Commit: `refactor!: Browser(app, view.Layout); App.Brand is a view.Brand (D50)`.

## Stage 6 — documentation

- `README.md` (≤ 150 lines, four ordered sections): in the server section state that `App.Server` events reach every write; show `ServerConfig.Events`; in the browser section show `appkit.Browser(config.App, layout)` and `view.Brand`; say which files are `!browser`.
- `docs/ARCHITECTURE.md` (four sections) and the diagram that shows `MountServer`: the broker arrow to `rpc` and `mcp`.
- `docs/TESTS.md`: one row per new test. Do not link `docs/PLAN.md` from permanent docs.

Commit: `docs: broker wiring`.

## Delivery check

1. `gotest` (builds, vets, runs; a PR that does not compile, or whose tests could not run, is a failed delivery).
2. `grep -rn "crud\.Operation(" --include=*.go .` → empty.
3. `grep -rn "type Brand\|webtyp.com/layout" --include=*.go --include=go.mod .` → empty.
4. `git diff --stat main` lists only `server.go`, `serve.go`, `app.go`, `brand.go`, `session.go`, `browser.go`, `app_server.go`, `module_server.go`, `app_server_browser.go`, `module_server_browser.go`, `go.mod`, `go.sum`, `tests/*`, `README.md`, `docs/*`.

| Stage | Files | Commit |
|---|---|---|
| 1 | `tests/*` | `test: … (red)` |
| 2 | `server.go`, `serve.go` | `refactor!: …` |
| 4 | `app_server.go`, `module_server.go`, `app_server_browser.go`, `module_server_browser.go` | `refactor!: server types are !browser (D44)` |
| 5 | `app.go`, `brand.go`, `session.go`, `browser.go`, `tests/*` | `refactor!: Browser(app, view.Layout) (D50)` |
| 6 | `README.md`, `docs/*` | `docs: …` |
