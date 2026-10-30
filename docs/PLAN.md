---
PLAN: "refactor: the schema diagram through erd.ReadSteps; groupsOf deleted"
EXECUTOR: jules
REVIEWER: none
---

> Read AGENTS.md at the repo root before any change.
> **Finish by yourself:** when the delivery check passes, commit, push and open the pull request. Never ask whether to
> proceed or finalize — nobody answers, and a session that waits ends without your work.

# Schema through `erd.ReadSteps`

## Context

`MountServer` builds the database diagram it serves at `erd.SchemaPath` with `ddl.Capture`, a private `groupsOf`
(`schema.go`) and `erd.ReadApp` (`server.go:55-61`). That same path now lives in `erd` as `erd.ReadSteps(steps...)`
(`webtyp.com/erd` v0.0.14), so a module's demo can draw the diagram in the browser with exactly the server's logic.
This plan makes `appkit` call it and deletes its private copy: one way from migration steps to a diagram.

## Tools

```bash
go install webtyp.com/devflow/cmd/gotest@latest
go get webtyp.com/erd@v0.0.14
go mod tidy
```

Environment: `GOPRIVATE=webtyp.com/*,github.com/veltylabs/*`, `GOFLAGS=-mod=mod`.

## APIs used

| Symbol | Exact signature | Tag |
|---|---|---|
| `erd.ReadSteps` | `func ReadSteps(steps ...ddl.Step) (Diagram, error)` — `ddl.Capture`, one group per step in order, `ReadApp`; returns the capture error | `webtyp.com/erd` v0.0.14, `steps.go` |
| `erd.Mount` | unchanged, as used in `server.go:60` | v0.0.14 |
| `ddl.Step` | `struct { Name string; Run func(conn Execer, compiler Compiler) error }` | `webtyp.com/ddl` |

If a signature differs, stop and name the symbol in the PR; never adapt it by guessing.

## Stage 1 — test first: add to `tests/server_test.go`

`TestSchemaStepFails`: `appkit.MountServer` (through the same helpers as `TestSchemaServedWithItsPermission`, or
calling `appkit.MountServer` directly on `mock.New()` if `buildRouter` fails the test on error) with
`Migrations: []ddl.Step{{Name: "broken", Run: func(ddl.Execer, ddl.Compiler) error { return errors.New("boom") }}}`
→ `MountServer` returns an error whose text contains `"broken"` and `"boom"`, and `erd.SchemaPath` is not mounted.
The existing `TestSchemaServedWithItsPermission` and the 403 test stay unchanged (they prove the served diagram did not
change). Commit after this stage (this test may already pass: the behaviour is preserved, not new).

## Stage 2 — code

- `server.go`: replace

```go
		tables, err := ddl.Capture(cfg.Migrations)
		if err != nil {
			return err
		}
		erd.Mount(r, erd.SchemaPath, erd.ReadApp(groupsOf(tables)...)).Requires(erd.ResourceSchema, model.Read)
```

  with

```go
		diagram, err := erd.ReadSteps(cfg.Migrations...)
		if err != nil {
			return err
		}
		erd.Mount(r, erd.SchemaPath, diagram).Requires(erd.ResourceSchema, model.Read)
```

  and drop the `webtyp.com/ddl` import from `server.go` if it becomes unused there (it stays wherever `ServerConfig`
  declares `Migrations []ddl.Step`).
- Delete `schema.go`. `grep -rn "groupsOf" --include=*.go .` → empty.

Run `gotest`: green. Commit after this stage.

## Delivery check

1. `gotest` passes, existing schema tests unchanged. A PR that does not compile, or whose tests could not run, is a failed delivery.
2. `test ! -e schema.go` and `grep -rn "groupsOf\|ddl.Capture" --include=*.go . | grep -v _test` → empty.
3. `git diff --stat main` lists only `server.go`, `schema.go` (deleted), `tests/server_test.go`, `go.mod`, `go.sum`.

## Delivery rules

- Commit after every stage. Never run `git reset`, `git restore`, `git checkout -- <file>` or `git stash` on your own work.
- Stuck for 30 minutes: open the PR anyway, listing what fails.

## Stages

| # | Files | Result |
|---|---|---|
| 1 | `tests/server_test.go` | A failing migration step stops the mount |
| 2 | `server.go`, delete `schema.go` | One path from steps to diagram; green |
