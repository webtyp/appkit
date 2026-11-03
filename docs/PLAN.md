---
PLAN: "feat: appkit.BuildModules — build the app's modules and people modules on a given database"
EXECUTOR: jules
REVIEWER: none
---

> Read AGENTS.md at the repo root before any change.
> **Finish by yourself:** when the delivery check passes, commit, push and open the pull request. Never ask whether to
> proceed or finalize — nobody answers, and a session that waits ends without your work.

# Plan — `appkit.BuildModules`: the app's modules on any `*orm.DB` (a transaction for an import)

## Tools

```bash
go install webtyp.com/devflow/cmd/gotest@latest
```

Environment: `GOPRIVATE=webtyp.com/*,github.com/veltylabs/*`, `GOFLAGS=-mod=mod`.

## Why

`webtyp.com/dataimport` runs an app's import steps on one transaction and needs the app's modules built **on that
transaction**. Today only `BuildServer` builds them (`serve.go`, loop over `app.Modules`) and the people modules
(`rbac`, `authority`, `access`) are built inside `LoginByRUT.build` (`login_rut_server.go`). An app would have to
re-create each module by hand and drift when a module is added. This plan exposes the one construction path.

## Ecosystem search

- `Deps`, `Find[T]`, `ModuleServer.Build` → **reuse** (the import step asks `appkit.Find[T](deps)` by interface).
- No existing "build modules on a given db" in `appkit`, `app`, or `dataimport` → **extend** `appkit`.

## Design gate

1. **Prior art.** Rails `Rails.application.eager_load!` + running code inside `ActiveRecord::Base.transaction`,
   Django `apps.get_app_config` inside `transaction.atomic()`, Spring `@Transactional` services — unverified links.
   Shared standard: application services are built once by the framework and run inside a transaction. Followed.
2. **Names.** `appkit.BuildModules(app, db)` — "build the modules on this db".
3. **Ledger.** Concepts +1; ways to build an app's modules: stays 1 (`BuildServer` uses the same unexported
   `buildModules`; `LoginByRUT.build` uses the same unexported `buildIdentity`).
4. **Where.** `appkit` is the composition root that already owns both blocks. Server-only: `//go:build !wasm`, like
   `serve.go` and `login_rut_server.go` — `ddl` (and the server-side people modules) must never reach the browser binary; `orm` may.
5. **Deletes.** The inline module loop in `BuildServer` and the inline rbac/authority/access block in
   `LoginByRUT.build` (moved, not copied).

## APIs used (all in this repo, read at HEAD)

| Symbol | Where |
|---|---|
| `type Deps struct { DB *orm.DB; IDs model.IDGenerator; Events events.Publisher; TenantID, Timezone string; built []model.ModuleNaming }` | `module_server.go` |
| `func Find[T any](d Deps) (T, error)` | `module_server.go` |
| `BuildServer` module loop | `serve.go:61-78` |
| `LoginByRUT.build`: rbac/idle-minutes/authority/access block | `login_rut_server.go:100-138` |
| `unixid.NewUnixID()`, `&eventsmock.Broker{}`, `loggerOf(app)` | `serve.go` |

## Stage 1 — `serve.go`: extract the module loop

```go
// buildModules builds app.Modules in order on d.DB, appending each to d.built (so later modules Find it).
// It returns the operation and API modules among them, for BuildServer to mount.
func buildModules(app App, d *Deps) (ops []router.OperationModule, apis []router.APIModule, err error) {
	for _, m := range app.Modules {
		if m.Server.Build == nil {
			continue
		}
		built, err := m.Server.Build(*d)
		if err != nil {
			return nil, nil, fmt.Errorf("%s: %w", m.Name, err)
		}
		d.built = append(d.built, built)
		if o, ok := built.(router.OperationModule); ok {
			ops = append(ops, o)
		}
		if a, ok := built.(router.APIModule); ok {
			apis = append(apis, a)
		}
	}
	return ops, apis, nil
}
```

In `BuildServer`, replace the loop (`if d.DB != nil { for _, m := range app.Modules { … } }`) with:

```go
	if d.DB != nil {
		if ops, apis, err = buildModules(app, &d); err != nil {
			return nil, err
		}
	}
```

## Stage 2 — `login_rut_server.go`: extract the people modules

```go
// identity is the people modules: roles, users and their administration.
type identity struct {
	roles  *rbac.Service
	users  *authority.Module
	access *access.Module
}

// buildIdentity builds rbac, authority and access on d.DB (the block that was inline in LoginByRUT.build,
// including the idle-minutes resolution and authMod.SetLog(logger)).
func (l LoginByRUT) buildIdentity(d Deps, logger func(...any)) (identity, error)
```

Move lines 100-138 (from `rbacSvc, err := rbac.New(d.DB)` to the `access.New` error check) into it unchanged,
returning `identity{roles: rbacSvc, users: authMod, access: accessMod}`. `LoginByRUT.build` calls it right after the
security-event subscription and keeps using `id.roles`, `id.users`, `id.access` where it used the three locals.
Behaviour of `build` does not change.

## Stage 3 — `build_modules.go` (new, `//go:build !wasm`)

```go
//go:build !wasm

package appkit

import (
	"webtyp.com/orm"
	"webtyp.com/unixid"
	eventsmock "webtyp.com/events/mock" // same import path serve.go uses for eventsmock.Broker
)

// BuildModules builds the app's modules, in App.Modules order, and — with LoginByRUT — its people modules
// (authority, access), all on db. An import passes its transaction, so every write rolls back with it. Events go
// to a broker nobody listens to: importing history is not a business action.
//
// Example:
//
//	plan := dataimport.Plan[appkit.Deps]{
//		Build: func(tx *orm.DB) (appkit.Deps, error) { return appkit.BuildModules(config.App, tx) },
//	}
//	// in a step: users, err := appkit.Find[*authority.Module](s.Modules)
func BuildModules(app App, db *orm.DB) (Deps, error) {
	if db == nil {
		return Deps{}, ErrNoDatabase
	}
	ids, err := unixid.NewUnixID()
	if err != nil {
		return Deps{}, err
	}
	d := Deps{DB: db, IDs: ids, Events: &eventsmock.Broker{}, TenantID: app.TenantID, Timezone: app.Timezone}
	if _, _, err := buildModules(app, &d); err != nil {
		return Deps{}, err
	}
	if l, ok := app.Login.(LoginByRUT); ok {
		id, err := l.buildIdentity(d, loggerOf(app))
		if err != nil {
			return Deps{}, fmt.Errorf("login: %w", err)
		}
		d.built = append(d.built, id.users, id.access)
	}
	return d, nil
}
```

Add to the errors of the repo (where `ErrNoOpen` is declared, same style): `ErrNoDatabase = errors.New("appkit: BuildModules needs a database")`
(use the same error constructor the neighbouring `ErrNoOpen` uses). Use the import path of `eventsmock` and
`unixid` exactly as `serve.go` imports them.

## Stage 4 — tests (`tests/build_modules_test.go`, `//go:build !wasm`, package `tests`)

Reuse `mod`, `openMem`, `trustedIPMod` from `tests/app_test.go` / `tests/login_test.go`.

| Test | Setup → call | Expected |
|---|---|---|
| `TestBuildModules_NilDB` | `appkit.BuildModules(appkit.App{TenantID: "t"}, nil)` | `errors.Is(err, appkit.ErrNoDatabase)` |
| `TestBuildModules_FindsAppModules` | app with `mod("trusted", …trustedIPMod{})`, no Login; db `orm.New(mem.New())` | `appkit.Find[trustedIPMod](d)` no error |
| `TestBuildModules_LoginAddsPeople` | same plus `Login: appkit.LoginByRUT{}` | `appkit.Find[*authority.Module](d)` and `appkit.Find[*access.Module](d)` no error |
| `TestBuildModules_ModuleErrorNamed` | module `"bad"` whose Build returns `errors.New("boom")` | error text `bad: boom` |
| existing `TestBuildServer_*` | unchanged | still pass (proves the refactor kept behaviour) |

## Stage 5 — documentation

- `README.md`: in the server section, one paragraph + the Example above: "Import steps build the modules on their
  transaction with `BuildModules`". Keep the README ≤ 150 lines.
- `docs/ARCHITECTURE.md`: in Components, `BuildModules` next to `BuildServer`, and the decision "one construction
  path: `buildModules` and `buildIdentity` are shared by `BuildServer` and `BuildModules`; server-only files".

## Delivery check

1. `gotest` passes (a PR that does not compile, or whose tests could not run, is a failed delivery).
2. `grep -n "for _, m := range app.Modules" serve.go` → empty (the loop lives only in `buildModules`).
3. `grep -c "rbac.New(d.DB)" login_rut_server.go` → `1`.
4. `head -1 build_modules.go` → `//go:build !wasm`.

Commit after every stage. Never run `git reset`, `git restore`, `git checkout -- <file>` or `git stash` on your own
work. Stuck 30 minutes: open the PR anyway listing what fails.

## Stages

| Stage | Files | Done when |
|---|---|---|
| 1 | `serve.go` | `BuildServer` uses `buildModules`; existing tests pass |
| 2 | `login_rut_server.go` | `build` uses `buildIdentity`; existing tests pass |
| 3 | `build_modules.go`, errors file | compiles |
| 4 | `tests/build_modules_test.go` | four new tests pass |
| 5 | `README.md`, `docs/ARCHITECTURE.md` | written |
