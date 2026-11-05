---
PLAN: "feat!: appkit knows no login and no migration — entry is a module (access/module), migrations listed in the generated config/migrations.go"
EXECUTOR: antigravity (local, multi-repo with replace)
REVIEWER: none
---

> Read AGENTS.md at the root of every repo you touch before changing it. Follow the skills **api-design**,
> **api-harness**, **module-layout**, **project-layout**, **zero-debt**. Decision: P43-A of
> `veltylabs/mjosefa-cms/docs/DATA_IMPORT_MASTER_PLAN.md`.

# Plan — appkit only assembles; the entry and the migrations live in the pieces that own them

Local execution across repos: point every consumer at the local copies with `replace` in `go.mod`; publish nothing
until `gotest` passes in every repo **and** `mjosefa-cms` starts with `webtyp dev`. Then publish bottom-up with
`gopush` (order at the end), which drops each `replace`.

## Tools

```bash
go install webtyp.com/devflow/cmd/gotest@latest
go install webtyp.com/lang/cmd/langc@latest
```

## Why

`appkit` is production-only and assembles an app; it must not implement a feature nor know about migration. Today:

1. **Login inside appkit.** `App.Login` is a closed interface (`app.go:25-26`, unexported `loginMethod()`), and
   `login_rut_server.go` builds RUT + trusted IP + rate limit + first admin + roles, so appkit imports `auth/authority`,
   `auth/trusted_ip`, `rbac`, `access`. `browser.go`/`brand.go` hold the RUT form and the setup form; `Screens` appends
   `accessui.Screens` when the login is `LoginByRUT` (`browser.go:260`); `BuildModules` builds the people modules
   (`build_modules.go:39`); `idleSeconds` reads `LoginByRUT.IdleMinutes` (`browser.go:84`).
2. **Migration inside appkit.** `ModuleServer.Migrate func(ddl.Execer, ddl.Compiler) error` (`module_server.go:21`);
   `webtyp/migrate` reads it and asks `case appkit.LoginByRUT` to add the login tables.

After: the entry is **one module** in `App.Modules` (`webtyp.com/access/module`), found by appkit through a contract
made only of `router`/`model` types; the list of migrations is **written** in a generated file of the app,
`config/migrations.go`, read by `migrate/main.go` and by the dev server's diagram.

## Ecosystem search

- Contracts that cross the boundary already exist in neutral pieces: `router.Middleware`, `model.Authorizer`,
  `model.PolicyDescriber`, `router.APIModule`, `router.OperationModule`, `model.ModuleNaming` → reuse.
- `webtyp/access` already depends on `auth` and `rbac` and owns people and roles administration and its screens
  (`access/ui`) → **extend**: the entry module is `webtyp.com/access/module` (same pattern as every veltylabs module:
  `module/module.go`, `module/server.go`, `module/browser.go`).
- The session contract the browser needs for any entry (`auth.ProfileDTO`, `auth.OpMe`, `auth.PathLogout`,
  `auth.PathSetup`) stays in `auth` and appkit keeps using it: it is common to every entry, not RUT-specific.
- `webtyp/migrate` v0.0.2 (`App`, `Run`, `Steps(app)`) → **change** to receive the steps, not the app.
- `server/maingen.go` already generates `web/server/main.go` and `migrate/main.go` → **extend** with
  `config/migrations.go` and `web/browser/main.go`.

## Design gate

1. Prior art: Go's `database/sql` driver pattern and chi middlewares — the assembler holds a contract, the
   implementation is a separate package chosen by the app (unverified link: https://pkg.go.dev/database/sql/driver).
   Rails/Django: authentication is an app or engine (`devise`, `django.contrib.auth`) the app installs, not the
   framework core.
2. Names: `appkit.Entry` (how people enter: `Authn`, `Authorize`, `Policy`); `appkit.EntryScreens` (the screens
   before the menu); `config.Migrations`; `migrate.App(open, steps...)`. Read aloud without docs.
3. Ledger: appkit imports −4 (`auth/authority`, `auth/trusted_ip`, `rbac`, `access`) and −1 (`ddl`); exported
   −`Login`, −`LoginByRUT`, −`LoginForm`, −`LoginScreen`, −`FirstAdminRegistrar`, −`RoleAdmin`,
   −`EnvSessionIdleMinutes`, −`ModuleServer.Migrate`; +`Entry`, +`EntryScreens`, +`ModuleBrowser.Entry`,
   +`Parts` seam. Ways to enter: 1 (a module). Ways to list migrations: 1 (`config/migrations.go`).
4. Belongs where: entry → `access` (people, roles, first admin, their screens); migration list → the app (generated);
   running migrations → `migrate`; assembling → `appkit`.
5. Deletes: `appkit/login_rut_server.go`, the RUT/setup forms in `appkit/browser.go` and `brand.go`,
   `App.Login`, `ModuleServer.Migrate` in **every** module (`Migrate:` field of 8 `module/server.go`, list in Stage 5),
   `migrate.Steps(app)`'s login switch.

## The contracts (exact)

```go
// webtyp/appkit — app.go: App loses the Login field.
type App struct {
	TenantID string
	Timezone string
	Brand    Brand
	Modules  []Module // the entry module goes first: appkit.Find finds it among the built modules
	Server   AppServer
	Log      func(...any)
}

// webtyp/appkit — module_server.go
type ModuleServer struct {
	Build func(d Deps) (model.ModuleNaming, error) // Migrate is gone
}

// Entry is how people enter the app; one module of App.Modules provides it. Without it every operation is denied.
// Example: entry, err := appkit.Find[appkit.Entry](d)
type Entry interface {
	Authn() router.Middleware      // who the caller is (session)
	Authorize() model.Authorizer   // what the caller may do
	Policy() model.PolicyDescriber // which role opens each resource (API explorer)
}

// Parts lets one module build several values (people, roles, administration): appkit adds each part to the built
// list, so Find, operations and API routes see them as if they were modules. Optional, asserted at the seam.
type Parts interface {
	Parts() []model.ModuleNaming
}

// webtyp/appkit — module_browser.go
type ModuleBrowser struct {
	Screens func(d BrowserDeps) []view.Screen
	Entry   func(d BrowserDeps) EntryScreens // only the entry module sets it
}

// EntryScreens are what the browser shows before the menu: the sign-in and, while the server has no
// administrator, the first-administrator setup. IdleSeconds locks an idle session (0 = 30 minutes).
type EntryScreens struct {
	Login       *Element // webtyp.com/dom Element, rendered as today's loginScreen.Render()
	Setup       *Element
	IdleSeconds int
}
```

```go
// webtyp/migrate — migrate.go (replaces Steps/Run/App of v0.0.2)
func Run(conn ddl.Execer, compiler ddl.Compiler, log func(...any), steps ...ddl.Step) error
func App(open func(dsn string) (storage.Conn, error), log func(...any), steps ...ddl.Step) error // DATABASE_URL
```

```go
// webtyp/devtools — server.go: the diagram reads the same list
func Server(steps ...ddl.Step) []func(r router.Router, policy model.PolicyDescriber) error
```

```go
// app (generated by webtyp/server maingen; //go:build !wasm) — config/migrations.go
// Code generated by https://webtyp.com. DO NOT EDIT.
package config

var Migrations = []ddl.Step{
	{Name: "access", Run: accessmigrate.Migrate},             // webtyp.com/access/migrate
	{Name: "service_catalog", Run: catalogmigrate.Migrate},   // github.com/veltylabs/service_catalog/migrate
	// … one per App.Modules entry, same order
}
// migrate/main.go:   if err := migrate.App(config.App.Server.Database, log.Println, config.Migrations...); err != nil { log.Fatal(err) }
// web/server/main.go: if err := appkit.Server(config.App, devtools.Server(config.Migrations...)...); err != nil { log.Fatal(err) }
// web/browser/main.go: appkit.Browser(config.App, devtools.Menu())
```

Generator rule for each `App.Modules` element `X.Module`: take the import path of `X` (e.g.
`github.com/veltylabs/service_catalog/module`), drop the last element `/module`, append `/migrate`, call its
`Migrate`; `Name` = the module repo's last path element (`service_catalog`, `access`). If that package has no
`Migrate` function, skip the module (it has no tables).

## Stage 1 — `webtyp/access`: the entry module

- `access/migrate/migrate.go` `Migrate`: runs, in order, `authority.Migrate`, `rbac.Migrate`, then today's body
  (the entry owns its tables: users, roles, audit).
- New `access/module/` (copy the shape of `github.com/veltylabs/service_catalog/module`):
  - `module.go` (no tag): `var Module = appkit.Module{Name: "access", Server: server, Browser: browser}`.
  - `server.go` (`!browser`): move **verbatim** from `appkit/login_rut_server.go` the constants (`maxLoginAttempts`,
    `loginWindow`, `loginBlock`, `RoleAdmin`, `permWildcardAll`, `EnvSessionIdleMinutes`), `securityEventLogger`,
    `seedFirstAdmin`, `identity`, `buildIdentity` (idle minutes from `SESSION_IDLE_MINUTES` or 30; the `IdleMinutes`
    field disappears) and `build` (returns the `*entry` below instead of `serverAuth`). `Find`/`ErrNoProvider` →
    `appkit.Find`/`appkit.ErrNoProvider`; `isErr` → `errors.Is` (server-only file, stdlib allowed).
    ```go
    type entry struct {
    	users  *authority.Module
    	access *access.Module
    	authn  router.Middleware
    	authz  model.Authorizer
    	policy model.PolicyDescriber
    }
    func (e *entry) ModelName() string                  { return "access_entry" }
    func (e *entry) Authn() router.Middleware            { return e.authn }
    func (e *entry) Authorize() model.Authorizer         { return e.authz }
    func (e *entry) Policy() model.PolicyDescriber       { return e.policy }
    func (e *entry) Parts() []model.ModuleNaming         { return []model.ModuleNaming{e.users, e.access} }
    var server = appkit.ModuleServer{Build: func(d appkit.Deps) (model.ModuleNaming, error) { return build(d) }}
    ```
    `Policy`: the value today's `serverAuth.Policy` gets (if it is never set today, return the `rbac` service if it
    implements `model.PolicyDescriber`, else `nil`, and say which in the PR).
  - `interfaces.go`: move `FirstAdminRegistrar` here (staff_manager satisfies it structurally; nothing changes there).
  - `browser.go` (`browser`): move from `appkit/browser.go` + `appkit/brand.go` the RUT form (`LoginForm`,
    `LoginScreen`, `rutData`, `post(auth.PathLoginRUT…)`), the setup form (`setupData`, `post(auth.PathSetup…)`),
    their messages, and their `lang.json` keys (`langc sync .` in both repos). `Screens: accessui.Screens(d.Caller)`,
    `Entry: func(d appkit.BrowserDeps) appkit.EntryScreens { … }` returning both forms rendered and `IdleSeconds`.
    Brand name for the title: add `Brand appkit.Brand` to `appkit.BrowserDeps` (appkit fills it from `App.Brand`).

## Stage 2 — `webtyp/appkit`: assemble only

- Delete `login_rut_server.go`, `App.Login`, `Login`, `LoginByRUT`, `defaultIdleMinutes` uses, `LoginForm`,
  `LoginScreen`, `RoleAdmin`, `EnvSessionIdleMinutes`, `FirstAdminRegistrar`, `ModuleServer.Migrate`.
- `buildModules`: after `Build`, if the value implements `Parts`, append each part to `d.built` (and to `ops`/`apis`
  when they implement `router.OperationModule`/`router.APIModule`). Same in `BuildModules`.
- `BuildServer`: replace the `switch app.Login.(type)` block with
  `entry, err := Find[Entry](d)`; `ErrNoProvider` → no entry: deny all (today's nil-login behaviour, logged once);
  otherwise `Authn: entry.Authn()`, `authorize = entry.Authorize()`, `Policy: entry.Policy()`.
- Browser (`browser.go`): find the one module with `Browser.Entry != nil` (more than one → panic with a clear
  message: two entry modules); use its `EntryScreens` where today `loginScreen`/`setupForm` render; idle lock from
  `IdleSeconds`; `Screens` no longer appends `accessui.Screens`. Keep the session contract (`auth.ProfileDTO`,
  `OpMe`, `PathLogout`, `PathSetup`).
- `go mod tidy`: `grep -n "auth/authority\|trusted_ip\|webtyp.com/rbac\|webtyp.com/access\|webtyp.com/ddl" go.mod` → only
  what the session contract still needs (`webtyp.com/auth`).

## Stage 3 — `webtyp/migrate`: receive the steps

Replace `Steps`, `Run`, `App` with the contract above (no `appkit` import; `EnvDatabaseURL` becomes a local const
`"DATABASE_URL"`; `ErrNoOpen` local). Tests: order, stop at first error, no URL, no open, no compiler.

## Stage 4 — `webtyp/devtools`: `Server(steps ...ddl.Step)`

Use `steps` instead of `migrate.Steps(app)`; drop the `appkit` and `migrate` imports from `server.go`/`server_off.go`.
Tests: build the app's steps by hand.

## Stage 5 — every module drops `Migrate:` from `module/server.go`

`var server = appkit.ModuleServer{Migrate: migrate.Migrate, Build: build}` → `appkit.ModuleServer{Build: build}`
and drop the now-unused `migrate` import in:
`veltylabs/modules/{staff_manager,device_manager,clinical_encounter,appointment_booking,patient_directory,service_catalog,room_layout,business_calendar}/module/server.go`.
Each still exports `migrate.Migrate` in its own `migrate/` package (the generator calls it).

## Stage 6 — `webtyp/server` maingen

- New template `config/migrations.go` (rule above), written by `writeGenerated` when `HasApp`. Parse `config/*.go`
  with `go/parser` (no build): find `var App = appkit.App{…}`, its `Modules` composite literal, each element
  `<ident>.Module`, map `<ident>` to its import path from the file's imports.
- `migrateTemplateText`, `appServerTemplateText` and a new `web/browser/main.go` template (`//go:build wasm`) as in
  the contract. `writeGenerated` keeps refusing to overwrite a hand-written file.
- The `docs/PLAN.md` running in this repo (session 5353264496857617316) is discarded: do not merge its PR.
- Tests in `server/tests/maingen_test.go`: an app with two modules gets `config/migrations.go` with both, in order;
  a module whose repo has no `migrate` package is skipped; the three mains contain the exact calls.

## Stage 7 — `veltylabs/mjosefa-cms`

- `config/app.go`: delete `Login: appkit.LoginByRUT{}`; `Modules` starts with `access.Module`
  (`webtyp.com/access/module`).
- Delete the hand-written `web/browser/main.go`; run the generator (`webtyp dev` or the maingen entry) to write the
  three mains and `config/migrations.go`.
- `migrate/pastdb/staff.go:55,58`: `appkit.Find[*authority.Module]` / `[*access.Module]` keep working through `Parts`.
- Tests: `appkit.Steps(config.App)` → `config.Migrations` (`tests/migrate_legacy_test.go`, `tests/app_schema_test.go`,
  `tests/setup_test.go`); `appkit.EnvSessionIdleMinutes` → the constant's new home or the literal env name
  `"SESSION_IDLE_MINUTES"` via the access module's exported const.
- `gotest`, then `webtyp dev`: the TUI shows «Migrating database» → «Database up to date», the login screen appears,
  «Mi agenda» opens without `hold_until` errors.

## Delivery check

1. `gotest` passes in: `access`, `appkit`, `migrate`, `devtools`, `server`, the 8 modules, `mjosefa-cms`.
2. `grep -rn "LoginByRUT\|loginMethod\|Migrate:\|appkit.Steps\|migrate.Steps" --include=*.go webtyp veltylabs` → empty
   (outside `docs/`).
3. `grep -n "webtyp.com/ddl\|webtyp.com/rbac\|webtyp.com/access\|auth/authority\|trusted_ip" webtyp/appkit/go.mod` → empty.
4. `langc sync .` in `access` and `appkit`: 0 untranslated, 0 reuse warnings.
5. `mjosefa-cms` running under `webtyp dev`: sign in, open «Mi agenda», «Personal», «Pacientes» — no server error in
   the log.

Never `init()`. Never edit `*_orm.go` by hand. Commit per stage in each repo; no `git reset`/`restore`/`stash` on
your own work.

## Stages and publish order

| Stage | Repo | Files | Publish order |
|---|---|---|---|
| 3 | `webtyp/migrate` | `migrate.go`, `tests/*`, docs | 1 |
| 2 | `webtyp/appkit` | `app.go`, `module_server.go`, `module_browser.go`, `serve.go`, `build_modules.go`, `browser.go`, `brand.go`, `login_rut_server.go` (deleted), `tests/*`, docs, `lang.json` | 2 |
| 1 | `webtyp/access` | `migrate/migrate.go`, `module/*` (new), `lang.json`, tests, docs | 3 |
| 4 | `webtyp/devtools` | `server.go`, `server_off.go`, `tests/*`, docs | 3 |
| 5 | 8 modules | `module/server.go` | 4 |
| 6 | `webtyp/server` | `maingen.go`, `tests/maingen_test.go`, docs | 4 |
| 7 | `mjosefa-cms` | `config/*`, `web/*/main.go`, `migrate/main.go`, `tests/*` | 5 |
