---
PLAN: "feat!: modules mount only as crud.Module; MountAPI only through AppServer.Infra; AppServer.PublicReads"
EXECUTOR: antigravity
REVIEWER: none
---

> Read AGENTS.md at the repo root before any change.
> **Finish by yourself:** when the delivery check passes, commit after every stage and stop. Never ask whether to
> proceed. This plan runs **locally**: do not tag, push or publish anything; the owner reviews first.
> Never run `git reset`, `git restore`, `git checkout -- <file>` or `git stash` on your own work.

# Plan: `appkit` mounts `crud.Module`, `Infra` and `PublicReads`

Context: master `webtyp/docs/ARCHITECTURE_MASTER_PLAN.md` D8, D10, D14. Today `buildModules` (`serve.go:51-62`)
type-asserts every built value: any module with `MountAPI` gets the full `router.Router` and can register
`r.Post(...).Public()`. After this plan a domain module can only be mounted as `crud.Module`; routes with the full
router come only from `AppServer.Infra`, which the **app** writes in `config/`; a read opens to visitors only if the
app lists it in `AppServer.PublicReads`. Runs after the rpc and mcp plans.

## Tools

```bash
go install webtyp.com/devflow/cmd/gotest@latest
```
`go.mod`: `replace` for `webtyp.com/model`, `webtyp.com/router`, `webtyp.com/crud`, `webtyp.com/rpc`, `webtyp.com/mcp` to the sibling folders; `go get webtyp.com/crud`; `go mod tidy`.

## APIs used

| Symbol | Signature |
|---|---|
| `crud.Module` | `interface{ model.ModuleNaming; MountOperations(reg crud.Registry) }` |
| `crud.Op` | `func Op(m model.ModuleNaming, action model.Action) string` |
| `rpc.Mount` | `func Mount(r router.Router, prefix string, log func(...any), public []model.ModuleNaming, modules ...crud.Module) error` |
| `rpc.Routes` | `func Routes(prefix string, public []model.ModuleNaming, modules ...crud.Module) ([]router.RouteInfo, error)` |
| `mcp.HarvestOps` | `func HarvestOps(expose []mcp.ToolName, modules ...crud.Module) (mcp.ToolProvider, error)` |
| `router.RouteInfo` | fields `Path string`, `Access model.Access` (`model.AccessPublic`) |

## Stage 1 — types (`app_server.go`, `module_server.go`)

```go
// AppServer is what only the server binary needs.
type AppServer struct {
	Database    func(dsn string) (storage.Conn, error) // the driver (postgres.Open); nil = no database
	MCPTools    []mcp.ToolName                         // operations an agent may call; empty = MCP closed
	Routes      func(r router.Router)                  // the app's own routes; nil = none
	Infra       []Infra                                // the only modules that get the whole Router (sessions, files); written by the app
	PublicReads []model.ModuleNaming                   // entities whose read visitors may call without a session; nil = none
}

// Infra is a module with its own HTTP routes (login, logout, file upload), mounted only
// because the app lists it. It is built after App.Modules, so it can Find what they built.
//
//	Infra: []appkit.Infra{access.Infra}
type Infra struct {
	Name  string                                  // identity in error messages
	Build func(d Deps) (router.APIModule, error)
}
```
Errors (`module_server.go`, next to the existing ones):
```go
ErrNotCrudModule       = errors.New("appkit: a module builds neither a crud.Module nor Parts — its operations would never be mounted")
ErrPublicReadNotMounted = errors.New("appkit: AppServer.PublicReads names an entity no module reads")
```

## Stage 2 — `serve.go`

`buildModules(app App, d *Deps) (ops []crud.Module, err error)`:
- `appendBuilt` appends to `d.built` and, `if o, ok := v.(crud.Module); ok { ops = append(ops, o) }`. **Delete the `router.APIModule` assertion.**
- After `Build`: a top-level value that is neither `crud.Module` nor `Parts` → `fmt.ErrType(fmt.Err(m.Name), ErrNotCrudModule)`.

`buildInfra(app App, d Deps) ([]router.APIModule, error)`: for each `app.Server.Infra` entry with non-nil `Build`, build it
(error wrapped with `fmt.Err(i.Name)`), skip nil results. Called in `BuildServer` right after `buildModules` (only when `d.DB != nil`, like modules).

`BuildServer` passes `Operations: ops`, `APIModules: apis` (from `buildInfra`) and `PublicReads: app.Server.PublicReads` to `MountServer`.

## Stage 3 — `server.go`

`ServerConfig`: `Operations []crud.Module`, new `PublicReads []model.ModuleNaming`, `APIModules` comment becomes "built from AppServer.Infra".
`MountServer`: `rpc.Mount(r, rpc.DefaultPrefix, cfg.Logger, cfg.PublicReads, cfg.Operations...)`; `mcp.HarvestOps(cfg.MCPTools, cfg.Operations...)`.
Then check every public read was mounted:
```go
func checkPublicReads(public []model.ModuleNaming, ops []crud.Module) error {
	if len(public) == 0 {
		return nil
	}
	routes, err := rpc.Routes(rpc.DefaultPrefix, public, ops...)
	if err != nil {
		return err
	}
	for _, p := range public {
		suffix := "/" + crud.Op(p, model.Read)
		found := false
		for _, r := range routes {
			if r.Access == model.AccessPublic && strings.HasSuffix(r.Path, suffix) {
				found = true
				break
			}
		}
		if !found {
			return fmt.ErrType(fmt.Err(p.ModelName()), ErrPublicReadNotMounted)
		}
	}
	return nil
}
```
(`server.go` is `!browser`; `strings` from stdlib is acceptable there only if `webtyp.com/fmt` has no suffix helper — check `fmt.Convert(...).HasSuffix` first and prefer it.)

## Stage 4 — tests

`tests/server_test.go`, `tests/build_modules_test.go`: fakes become `crud.Module`. New (rows in `docs/TESTS.md`):

| Test | Setup | Expected |
|---|---|---|
| `TestBuildModules_APIModuleIsNotMounted` (`tests/infra_test.go`) | a module whose built value is a `crud.Module` **and** has `MountAPI` registering `GET /x` | after `BuildServer`, `/x` is not among `srv.Router().Routes()` |
| `TestInfra_Mounted` | `AppServer.Infra = []appkit.Infra{{Name: "auth", Build: …returns fake with MountAPI GET /x…}}` | `/x` is registered |
| `TestBuildModules_NeitherCrudNorPartsIsError` | module whose `Build` returns a plain `model.ModuleNaming` | `errors.Is(err, appkit.ErrNotCrudModule)` |
| `TestPublicReads_Mounted` | crud module with entity `rec`, `PublicReads: {&rec{}}` | route `/api/<mod>/rec_r` has `Access == model.AccessPublic` |
| `TestPublicReads_UnknownIsError` | `PublicReads: {&other{}}` with no module reading `other` | `errors.Is(err, appkit.ErrPublicReadNotMounted)` |

## Stage 5 — documentation

`README.md` (4 mandatory sections, ≤150 lines) with a complete `config/app.go`: `Modules`, `Server.Infra`, `Server.PublicReads`.
`docs/ARCHITECTURE.md`: why domain modules never get `router.Router`; build order Modules → Infra → mount.

## Delivery check

`gotest` passes; `grep -rn "OperationModule\|v.(router.APIModule)" --include=*.go .` → empty.

| Stage | Files |
|---|---|
| 1 | `app_server.go`, `module_server.go` |
| 2 | `serve.go` |
| 3 | `server.go` |
| 4 | `tests/*.go`, `docs/TESTS.md` |
| 5 | `README.md`, `docs/ARCHITECTURE.md` |
