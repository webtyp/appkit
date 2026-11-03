# appkit
<img src="docs/img/badges.svg">

The application kit: one config turns WebTyp pieces (server, auth, permissions, session, menu) into a ready app

## Why This Exists

Without appkit, each app writes a complex `config/server.go` (database, ids, events, module constructors, adapters between modules, httpd, MountServer) plus `config/migrate.go` and `cmd/migrate/main.go`. With appkit, it's just one [`App`](app.go) literal; `web/server/main.go` and `migrate/main.go` are one line each. The dependency injection uses [`Find`](module.go) across each [`Module`](module.go).

| Without | With |
|---|---|
| Hand-written `config/server.go` | One `appkit.App` literal |
| Hand-written `config/migrate.go` | |
| `cmd/migrate/main.go` | One line `migrate/main.go` |
| Manual dependency injection | `appkit.Find` across `appkit.Module`s |

## Before You Start

- The database driver belongs to the app (`App.Open`), so `appkit` never imports a driver directly (e.g. `postgres`).
- Modules are built and migrated in exactly the order they appear in `App.Modules`.
- A module that needs another module must be listed after it in the `App.Modules` list so `Find` can see it.

## Step-by-Step Guide

1. Create a module exporting `var Module = appkit.Module{Name, Migrate, Build}` whose `Build` uses `appkit.Find[CategorySource](d)`.
2. Write `config/app.go` with the single `appkit.App{…}` declaration.
3. Write the server and migration entries:

```go
// web/server/main.go
func main() { if err := appkit.Server(config.App); err != nil { log.Fatal(err) } }
```

```go
// migrate/main.go
func main() { if err := appkit.Migrate(config.App); err != nil { log.Fatal(err) } }
```

| `ServerConfig` field | What |
|---|---|
| `Migrations []ddl.Step` | non-empty → `GET /_schema` (`db_schema:read`, never public) serves the app's database diagram ([`webtyp.com/erd`](https://github.com/webtyp/erd)), one band per migration step |

## Documentation Map

| File | What |
|---|---|
| [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md) | Design and decisions |
| [`module.go`](module.go) | Modules and dependencies |
| [`app.go`](app.go) | The Application declaration |
| [`serve.go`](serve.go) | Server build and environment |
| [`migrate.go`](migrate.go) | Migration execution |
