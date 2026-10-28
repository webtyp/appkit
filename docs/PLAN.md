---
PLAN: "feat: ServerConfig.Migrations — GET /_schema serves the app's database diagram"
TAG: v0.0.7
EXECUTOR: jules
REVIEWER: none
STATUS: review
SESSION: 13470001113254648269
PR: https://github.com/webtyp/appkit/pull/3
REVIEWED_COMMIT: 49fca9246a3f1a7529e8eb94d73c2dbe54e8d099
---

> Read AGENTS.md at the repo root before any change.

<!-- Dispatch after webtyp.com/ddl v0.0.32 (ddl.Step, ddl.Capture) is published. webtyp.com/erd v0.0.2 is already published. -->

# Plan — `webtyp.com/appkit`: the app's database diagram

## Tools

```bash
go install webtyp.com/devflow/cmd/gotest@latest
go get webtyp.com/ddl@v0.0.32 webtyp.com/erd@v0.0.2
```

## Why

Every webtyp app with a database gets, without writing it, a «Developer tools → Database» screen. The server
half lives here, next to `/_routes`: the app passes its migration list; appkit captures the tables each step
creates (no database touched), reads them as one diagram grouped by module, and serves it at `GET /_schema`
guarded by `db_schema:read`. An app without migrations gets no route.

## Ecosystem search

`ddl.Capture` (tables per step, no database), `erd.ReadApp` / `erd.Mount` (diagram, findings, route) exist. Reuse
both; appkit only wires them, once, for every app (api-design gate 4: «the glue is written once»).

## Design gate

1. **Prior art:** Rails `db/schema.rb`, Django `inspectdb`/`sqlmigrate`, Prisma Studio: the schema is derived from
   the migration source and shown by the framework, not by each app.
2. **Name:** `ServerConfig.Migrations` — «the app's migrations», the word apps already use (`MigrationSteps()`).
3. **Ledger:** concepts +1 (`Migrations`) / −0; files an app touches to show its schema 1 (`config/server.go`) / −0;
   ways +0.
4. **Belongs:** appkit already mounts `/_routes` for every app; `/_schema` is its sibling.
5. **Deletes:** nothing here; new capability.

## APIs used

| Symbol | Signature | Tag | File |
|---|---|---|---|
| `ddl.Step` | `type Step struct { Name string; Run func(conn Execer, compiler Compiler) error }` | ddl v0.0.32 | `capture.go` |
| `ddl.Table` | `type Table struct { Step string; Model model.Model }` | ddl v0.0.32 | `capture.go` |
| `ddl.Capture` | `func Capture(steps []Step) ([]Table, error)` | ddl v0.0.32 | `capture.go` |
| `erd.Group` | `type Group struct { Name string; Models []model.Model }` | erd v0.0.2 | `read.go` |
| `erd.ReadApp` | `func ReadApp(groups ...Group) Diagram` | erd v0.0.2 | `read.go` |
| `erd.Mount` | `func Mount(r router.Router, path string, d Diagram) router.Route` | erd v0.0.2 | `mount.go` |
| `erd.SchemaPath`, `erd.ResourceSchema` | `const SchemaPath = "/_schema"`; `const ResourceSchema model.Resource = "db_schema"` | erd v0.0.2 | `erd.go` |
| `erd.Report` | `type Report struct { SVG string; Findings []Finding }`, decodable with `webtyp.com/json.Decode` | erd v0.0.2 | `erd.go` |

If a signature in the published tag differs from this table, stop and name the symbol in the PR; never adapt it by
guessing.

## Stage 1 — `server.go`

Add the field to `ServerConfig`, after `Register`:

```go
	Migrations []ddl.Step // the app's migrations, in order; non-empty → GET /_schema (db_schema:read) serves its database diagram
```

In `MountServer`, right after the `router.MountIntrospection(...)` line:

```go
	if len(cfg.Migrations) > 0 {
		tables, err := ddl.Capture(cfg.Migrations)
		if err != nil {
			return err
		}
		erd.Mount(r, erd.SchemaPath, erd.ReadApp(groupsOf(tables)...)).Requires(erd.ResourceSchema, model.Read)
	}
```

New file `schema.go` (no build tag: a library file carries no destination tag):

```go
package appkit

import (
	"webtyp.com/ddl"
	"webtyp.com/erd"
)

// groupsOf turns captured tables into one diagram group per migration step, in step order.
func groupsOf(tables []ddl.Table) []erd.Group {
	var groups []erd.Group
	for _, t := range tables {
		if n := len(groups); n > 0 && groups[n-1].Name == t.Step {
			groups[n-1].Models = append(groups[n-1].Models, t.Model)
			continue
		}
		groups = append(groups, erd.Group{Name: t.Step, Models: []model.Model{t.Model}})
	}
	return groups
}
```

(add `"webtyp.com/model"` to its imports.)

## Stage 2 — tests (`tests/server_test.go`)

Use the file's own `buildRouter`, `get` and `fakeAuthn`. A step that creates two tables, the second referencing
a table nobody creates:

```go
type table struct {
	name string
	ref  string // foreign key column "ref_id" → ref, when not empty
}

func (t *table) ModelName() string { return t.name }
func (t *table) Schema() []model.Field {
	f := []model.Field{{Name: "id", Type: model.Text(), DB: &model.FieldDB{PK: true}}}
	if t.ref != "" {
		f = append(f, model.Field{Name: "ref_id", Type: model.Text()})
	}
	return f
}
func (t *table) SchemaExt() []model.FieldExt {
	if t.ref == "" {
		return nil
	}
	return []model.FieldExt{{Field: t.Schema()[1], Ref: t.ref}}
}
func (t *table) Pointers() []any                  { return nil }
func (t *table) IsNil() bool                      { return t == nil }
func (t *table) EncodeFields(_ model.FieldWriter) {}
func (t *table) DecodeFields(_ model.FieldReader) {}

var shopStep = ddl.Step{Name: "shop", Run: func(conn ddl.Execer, c ddl.Compiler) error {
	d := ddl.New(conn, c)
	if err := d.CreateTable(&table{name: "customer"}); err != nil {
		return err
	}
	return d.CreateTable(&table{name: "invoice", ref: "tenant"})
}}

func allowSchema(userID string, res model.Resource, a model.Action) bool {
	return res == erd.ResourceSchema && a == model.Read
}

func TestSchemaServedWithItsPermission(t *testing.T) {
	r := buildRouter(t, appkit.ServerConfig{Migrations: []ddl.Step{shopStep}, Authorize: allowSchema}, fakeAuthn("admin"))
	status, body := get(r, erd.SchemaPath)
	if status != 200 {
		t.Fatalf("GET %s = %d, want 200", erd.SchemaPath, status)
	}
	var rep erd.Report
	if err := json.Decode([]byte(body), &rep); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(rep.SVG, `id="erd-table-invoice"`) || !strings.Contains(rep.SVG, ">shop<") {
		t.Error("the diagram must draw the step's tables in a band named after the step")
	}
	if len(rep.Findings) != 1 || rep.Findings[0].Rule != erd.RuleMissingTable || rep.Findings[0].Target != "tenant" {
		t.Fatalf("findings = %+v, want one missing_table → tenant", rep.Findings)
	}
}

func TestSchemaRequiresDBSchemaPermission(t *testing.T) {
	r := buildRouter(t, appkit.ServerConfig{Migrations: []ddl.Step{shopStep}, Authorize: allowFakeRead}, fakeAuthn("clerk"))
	if status, _ := get(r, erd.SchemaPath); status != 403 {
		t.Fatalf("GET %s without db_schema:read = %d, want 403", erd.SchemaPath, status)
	}
}

func TestNoMigrationsNoSchemaRoute(t *testing.T) {
	r := buildRouter(t, appkit.ServerConfig{Authorize: allowSchema}, fakeAuthn("admin"))
	if status, _ := get(r, erd.SchemaPath); status == 200 {
		t.Fatal("an app without migrations must not serve /_schema")
	}
}
```

(imports: `webtyp.com/ddl`, `webtyp.com/erd`, `webtyp.com/json`.)

## Stage 3 — docs

`README.md`: in the `ServerConfig` table/list add `Migrations []ddl.Step` — «non-empty → `GET /_schema`
(`db_schema:read`) serves the database diagram (`webtyp.com/erd`), grouped by step». Never `Public()`.

## Delivery

- `gotest` green. Environment: `GOPRIVATE=webtyp.com/*,github.com/veltylabs/*`, `GOFLAGS=-mod=mod`. A PR that does
  not compile, or whose tests could not run, is a failed delivery.
- `git diff --stat main` lists exactly: `go.mod`, `go.sum`, `server.go`, `schema.go`, `tests/server_test.go`,
  `README.md`.
- Never pause to ask permission or report progress: when the check passes, commit, push and open the PR. Stuck 30
  minutes → open the PR listing what fails. Commit after every stage; never `git reset`, `git restore`,
  `git checkout -- <file>` or `git stash` on your own work.

| Stage | File | Action |
|---|---|---|
| 1 | `server.go`, `schema.go` | `Migrations`; capture → `erd.Mount` with `db_schema:read` |
| 2 | `tests/server_test.go` | served with permission, 403 without, absent without migrations |
| 3 | `README.md` | one entry |
