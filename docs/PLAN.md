---
PLAN: "refactor: guard /_routes with router.ResourceIntrospection; delete appkit.ResourceAPIExplorer"
TAG: v0.0.5
EXECUTOR: jules
REVIEWER: none
STATUS: running
SESSION: 9680790118507378019
---

> Read AGENTS.md at the repo root before any change.

# Plan — `webtyp.com/appkit`: one name for the permission of `/_routes`

## Why

`/_routes` and the API explorer screen share one RBAC resource, `"api_explorer"`, today written in three repos.
`webtyp/router` v0.4.9 names it once, `router.ResourceIntrospection`, next to `router.IntrospectionPath`. This repo
uses it and deletes its own copy.

## Tools

```
go install webtyp.com/devflow/cmd/gotest@latest
go get webtyp.com/router@v0.4.9
```

`webtyp.com/router@v0.4.9` is the first tag with `ResourceIntrospection`. If `go get` fails or the constant is
missing, stop and say so in the PR.

## Ecosystem search

`router.ResourceIntrospection` (`webtyp.com/router` v0.4.9, `introspection.go`) → **reuse**. Nothing new is created.

## Design gate

1. **Prior art.** Spring Boot Actuator, Laravel Telescope (`viewTelescope`) and Django `admindocs` name the
   permission where the endpoint lives; consumers import it.
2. **Novice-name test.** Callers read `router.ResourceIntrospection` beside `router.IntrospectionPath`.
3. **Complexity ledger.** Concepts −1 (`appkit.ResourceAPIExplorer`); ways to name the permission −1.
4. **Where it belongs.** `router`, the owner of `/_routes`.
5. **What it deletes.** `appkit.ResourceAPIExplorer`.

## APIs used

| Symbol | Exact signature | Tag | File |
|---|---|---|---|
| `router.ResourceIntrospection` | `const ResourceIntrospection model.Resource = "api_explorer"` | router v0.4.9 | `introspection.go` |
| `router.MountIntrospection` | `func MountIntrospection(r Router, path string, policy model.PolicyDescriber) Route` | router v0.4.9 | `introspection.go` |

If a signature in the published tag differs from this table, stop and name the symbol in the PR; never adapt it by guessing.

## Stage 1 — `server.go`

- Delete `const ResourceAPIExplorer model.Resource = "api_explorer"` (line 24).
- Line 52 becomes
  `router.MountIntrospection(r, router.IntrospectionPath, cfg.Policy).Requires(router.ResourceIntrospection, model.Read)`.

Check: `grep -rn "ResourceAPIExplorer\|\"api_explorer\"" --include=*.go .` → empty.

## Stage 2 — tests (`tests/server_test.go`)

- Keep `TestIntrospectionRequiresAPIExplorer` (403 without the permission). Change its failure message to
  `"GET %s without router.ResourceIntrospection read permission: want 403, got %d"`.
- Add `TestIntrospectionOpensWithItsPermission`: `buildRouter(t, appkit.ServerConfig{Operations:
  []router.OperationModule{fakeOps{}}, Authorize: func(userID string, res model.Resource, a model.Action) bool {
  return res == router.ResourceIntrospection && a == model.Read }}, fakeAuthn("test-user"))`; `get(r,
  router.IntrospectionPath)` → status 200. (Use the existing helpers `buildRouter`, `fakeAuthn`, `get` of this file.)

## Stage 3 — docs

`docs/ARCHITECTURE.md`, «Server half»: write `Requires(router.ResourceIntrospection, model.Read)` instead of
`Requires(<introspection resource>, model.Read)`. `README.md`: no change unless it names `ResourceAPIExplorer`
(then replace it).

## Delivery

- `gotest` green. A PR that does not compile, or whose tests could not run, is a failed delivery.
  Environment: `GOPRIVATE=webtyp.com/*,github.com/veltylabs/*`, `GOFLAGS=-mod=mod`.
- Never pause to ask permission or report progress: when the check passes, commit, push and open the PR. Stuck 30
  minutes → open the PR listing what fails. Commit after every stage; never `git reset`, `git restore`,
  `git checkout -- <file>` or `git stash` on your own work.

| Stage | File | Action |
|---|---|---|
| 1 | `server.go`, `go.mod`, `go.sum` | use `router.ResourceIntrospection`; delete own constant |
| 2 | `tests/server_test.go` | 403 without, 200 with |
| 3 | `docs/ARCHITECTURE.md` | name the constant |
