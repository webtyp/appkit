---
PLAN: "refactor!: AppServer.Public replaces PublicReads; modules' DefaultGrants applied to the authenticated role at boot"
EXECUTOR: antigravity
REVIEWER: none
---

> Read AGENTS.md at the repo root before any change.
> **Finish by yourself:** when the delivery check passes, commit after every stage and stop. Never ask whether to
> proceed. This plan runs **locally** and is **published as soon as the delivery check passes** (no `replace` anywhere).
> Never run `git reset`, `git restore`, `git checkout -- <file>` or `git stash` on your own work.

# Plan: `AppServer.Public` and default grants

## Tools

```bash
go install webtyp.com/devflow/cmd/gotest@latest
go install webtyp.com/lang/cmd/langc@latest
go get webtyp.com/crud@latest webtyp.com/rpc@latest webtyp.com/rbac@latest
```
These must contain `crud.Public` (A6.2), `rpc.Mount(…, []crud.Public, …)` (A6.7 rpc) and `rbac.RoleAuthenticated` / `EnsureGrants`
(A6.8); if `go doc webtyp.com/rbac EnsureGrants` fails, stop. No `replace`.

## Why

Decisions D23 and D24 of `webtyp/docs/ARCHITECTURE_MASTER_PLAN.md`. The app — never a module — opens actions to visitors
(`Public`, reads and writes). And what any signed-in person may do is a grant to `rbac.RoleAuthenticated` that each module declares
(`DefaultGrants`) and the app applies once at boot.

## Stage 1 — `app_server.go`, `server.go`, `serve.go`, `module_server.go`

- `AppServer.PublicReads []model.ModuleNaming` → `Public []crud.Public // actions visitors may call without a session; nil = none`.
- `checkPublicReads` → `checkPublic(public []crud.Public, ops []crud.Module) error`: for each entry and each action bit, the route
  `crud.Op(p.Model, action)` must exist with `Access == model.AccessPublic`; else `ErrPublicNotMounted = errors.New("appkit:
  AppServer.Public names an action no module offers")` (rename `ErrPublicReadNotMounted`).
- `rpc.Mount(r, rpc.DefaultPrefix, cfg.Logger, cfg.Public, cfg.Operations...)`; `serve.go:182` passes `app.Server.Public`.
- `ModuleServer` gains `DefaultGrants []model.Grant // what any signed-in person may do with this module (granted to rbac.RoleAuthenticated)`.
- `AppServer` gains `Grants interface{ EnsureGrants(projectID string, role model.RoleCode, grants []model.Grant) error } // the app's rbac
  service; required when any module declares DefaultGrants`. In `BuildServer`, after the modules are built: merge every module's
  `DefaultGrants`; none → nothing; some and `Grants == nil` → `ErrNoGrants = errors.New("appkit: a module declares DefaultGrants but
  AppServer.Grants is nil; set it to the app's rbac service")`; else `Grants.EnsureGrants(app.TenantID, rbac.RoleAuthenticated, merged)`
  (the rbac project is `App.TenantID`, `app.go:7`).

## Stage 2 — tests (rows in `docs/TESTS.md`)

| Test | Expected |
|---|---|
| `TestPublic_CreateMounted` | module with a table allowing Create; `Public: [{Model, model.Create}]` → server boots, route public |
| `TestPublic_UnknownIsError` | `Public` names an action no module offers → `ErrPublicNotMounted` (replaces `TestPublicReads_UnknownIsError`) |
| `TestGrants_AppliedAtBoot` | module with `DefaultGrants`; a fake `Grants` records the call → called once with `app.TenantID`, `rbac.RoleAuthenticated`, the grants |
| `TestGrants_MissingGranterIsError` | `DefaultGrants` set, `Grants` nil → `ErrNoGrants` |

## Stage 3 — documentation

`README.md`, `docs/ARCHITECTURE.md`: `PublicReads` → `Public` with an example of a public create; «DefaultGrants and the authenticated
role». `langc sync .` if any text changed.

## Delivery check

`gotest`; `grep -rn "PublicReads" .` → empty. A tree that does not compile or whose tests did not run is a failed delivery.

| Stage | Files |
|---|---|
| 1 | `app_server.go`, `server.go`, `serve.go`, `module_server.go` |
| 2 | `tests/*`, `docs/TESTS.md` |
| 3 | `README.md`, `docs/ARCHITECTURE.md`, `lang.json` |
