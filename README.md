# appkit
<img src="docs/img/badges.svg">

The application kit: one value turns WebTyp pieces (server, login, permissions, session, menu) into a ready app, for both binaries.

## Why This Exists

Without appkit, each app writes its server wiring, its login wiring (users, roles, first administrator, rate limit),
its browser session, its migrate command, and lists every module twice — once for the server, once for the menu.
With appkit, the app declares one [`App`](app.go); each module is one [`Module`](app.go) value; `web/server/main.go`
and `migrate/main.go` are generated, and `web/browser/main.go` is one line.

| Without | With |
|---|---|
| `config/server.go`, `config/auth.go`, `config/client.go` | One `appkit.App` |
| Every module listed for the server and again for the menu | `App.Modules`, once |
| `cmd/migrate/main.go` | Generated `migrate/main.go` = `appkit.Migrate(config.App)` |
| Adapters written by hand between modules | [`Find`](module_server.go) by interface |

## Before You Start

- **appkit's types carry no destination tag**: `App`, `Module`, `AppServer`, `ModuleServer`, `ModuleBrowser`
  exist in every binary, so gopls never reports an undefined symbol. What keeps server code out of the wasm is the
  **values**: a module (and the app) fills each part from a tagged file and leaves it zero in the other binary.
- Modules are built, migrated and shown in `App.Modules` order. A module that `Find`s another goes after it.
  The first screen of the list is the one opened first.
- `Login: appkit.LoginByRUT{}` brings users, roles (`access`), rate limit, idle lock and first-administrator setup.
  A module that implements [`FirstAdminRegistrar`](module_server.go) completes that administrator (its device, its record).
- appkit never imports a database driver: the app passes it in `AppServer.Database`.

## Step-by-Step Guide

1. A module exports one value from its `module/` package:

```go
// module/module.go (no tag)
var Module = appkit.Module{Name: "service_catalog", Server: server, Browser: browser}

// module/server.go (//go:build !browser)
var server = appkit.ModuleServer{Migrate: migrate.Migrate, Build: build}
var browser appkit.ModuleBrowser

func build(d appkit.Deps) (model.ModuleNaming, error) {
	return servicecatalog.New(d.DB, servicecatalog.Deps{IDs: d.IDs, Publisher: d.Events, TenantID: d.TenantID})
}

// module/browser.go (//go:build browser)
var server appkit.ModuleServer
var browser = appkit.ModuleBrowser{Screens: ui.Screens}
```

2. The app declares itself:

```go
// config/app.go (no tag)
var App = appkit.App{
	TenantID: "acme",
	Timezone: "America/Santiago",
	Brand:    Brand, // config/brand.go: appkit.Brand{Name, LogoMark}; sitec renders its access page
	Modules:  []appkit.Module{catalog.Module},
	Login:    appkit.LoginByRUT{},
	Server:   server,
	Log:      fmt.Println,
}

// config/server.go (//go:build !browser)
var server = appkit.AppServer{Database: postgres.Open}

// config/browser.go (//go:build browser)
var server appkit.AppServer
```

`Timezone` is **required**: an IANA name such as `America/Santiago`. appkit loads it once and every module receives
it already loaded — `Deps.Zone` on the server, `BrowserDeps.Zone` in the browser — so every date and hour is shown on
the installation's clock. Empty or unknown → the app does not start (`ErrNoTimezone`), never UTC in silence.

```go
// a module's screens (browser)
var browser = appkit.ModuleBrowser{Screens: func(d appkit.BrowserDeps) []view.Screen {
	return ui.Screens(d.Caller, d.Zone)
}}
```

3. The browser main, the only one the app writes:

```go
// web/browser/main.go
func main() { appkit.RunBrowser(config.App) }
```

Import steps build the modules on their transaction with `BuildModules`:

```go
	plan := dataimport.Plan[appkit.Deps]{
		Build: func(tx *orm.DB) (appkit.Deps, error) { return appkit.BuildModules(config.App, tx) },
	}
	// in a step: users, err := appkit.Find[*authority.Module](s.Modules)
```

## Documentation Map

| File | What |
|---|---|
| [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md) | Design and decisions |
| [`app.go`](app.go) | `App`, `Module`, `Login`, `LoginByRUT` |
| [`module_server.go`](module_server.go) | `ModuleServer`, `Deps`, `Find`, `FirstAdminRegistrar` |
| [`login_rut_server.go`](login_rut_server.go) | What `LoginByRUT` wires on the server |
| [`serve.go`](serve.go) / [`migrate.go`](migrate.go) | Server build, migrations |
| [`run_browser.go`](run_browser.go) | The browser session |
| [`server.go`](server.go) | `MountServer`: rpc, MCP, `/_routes`, `/_schema` |
