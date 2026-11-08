# appkit
<img src="docs/img/badges.svg">

The application kit: one value turns WebTyp pieces (server, permissions, session, menu) into a ready app, for both binaries.

## Why This Exists

Without appkit, each app writes its server wiring, its login wiring (users, roles, first administrator, rate limit),
its browser session, and lists every module twice — once for the server, once for the menu.
With appkit, the app declares one [`App`](app.go); each module is one [`Module`](app.go) value; `web/server/main.go`
and `web/browser/main.go` are generated (migrations live in `webtyp.com/migrate`, dev tools in `webtyp.com/devtools`).

| Without | With |
|---|---|
| `config/server.go`, `config/auth.go`, `config/client.go` | One `appkit.App` |
| Every module listed for the server and again for the menu | `App.Modules`, once |
| Dev tools mounted by hand | Passed to `Server` and `Browser` |
| Adapters written by hand between modules | [`Find`](module_server.go) by interface |

## Before You Start

- **appkit's types carry no destination tag**: `App`, `Module`, `AppServer`, `ModuleServer`, `ModuleBrowser`
  exist in every binary, so gopls never reports an undefined symbol. What keeps server code out of the wasm is the
  **values**: a module (and the app) fills each part from a tagged file and leaves it zero in the other binary.
- Modules are built and shown in `App.Modules` order. The entry module (`webtyp.com/access/module`) goes first:
  appkit finds it among the built modules via `Find[Entry]`.
- The first screen of the list is the one opened first.
- appkit never imports a database driver: the app passes it in `AppServer.Database`.

## Step-by-Step Guide

1. A module exports one value from its `module/` package:

```go
// module/module.go (no tag)
var Module = appkit.Module{Name: "service_catalog", Server: server, Browser: browser}

// module/server.go (//go:build !browser)
var server = appkit.ModuleServer{Build: build}
var browser appkit.ModuleBrowser

func build(d appkit.Deps) (model.ModuleNaming, error) {
	return servicecatalog.New(d.DB, servicecatalog.Deps{IDs: d.IDs, Publisher: d.Events, TenantID: d.TenantID})
}

// module/browser.go (//go:build browser)
var server appkit.ModuleServer
var browser = view.ModuleBrowser{Screens: ui.Screens}
```

2. The app declares itself:

```go
// config/app.go (no tag)
var App = appkit.App{
	TenantID: "acme",
	Timezone: "America/Santiago",
	Brand:    Brand, // config/brand.go: appkit.Brand{Name, LogoMark}
	Modules:  []appkit.Module{access.Module, catalog.Module},
	Server:   server,
	Log:      fmt.Println,
}

// config/server.go (//go:build !browser)
var server = appkit.AppServer{
	Database:    postgres.Open,
	Infra:       []appkit.Infra{access.Infra},
	PublicReads: []model.ModuleNaming{&catalog.Category{}},
}

// config/browser.go (//go:build browser)
var server appkit.AppServer
```

`Log` is **required** too (`log.Println`): the server writes its warnings there — a leftover column, no entry module — and refuses to start without it (`ErrNoLog`), never in silence.

`Timezone` is **required**: an IANA name such as `America/Santiago`. appkit loads it once and every module receives
it already loaded — `Deps.Zone` on the server, `view.BrowserDeps.Zone` in the browser — so every date and hour is shown on
the installation's clock. Empty or unknown → the app does not start (`ErrNoTimezone`), never UTC in silence.

3. The generated mains:

```go
// web/server/main.go
func main() { if err := appkit.Server(config.App, config.Schema, devtools.Server(config.Schema)...); err != nil { log.Fatal(err) } }

// web/browser/main.go
func main() { layoutapp.Browser(config.App, devtools.Menu()) }
```

`config.Schema` (generated: every module's migration plus `config/removed.go`) lets the server log, at every start,
each column the models no longer have but that still holds data — it never drops one ([serve.go](serve.go)).

Import steps build the modules on their transaction with `BuildModules`:

```go
	plan := dataimport.Plan[appkit.Deps]{
		Build: func(tx *orm.DB) (appkit.Deps, error) { return appkit.BuildModules(config.App, tx) },
	}
	// in a step: users, err := appkit.Find[*authority.Module](s.Modules)
```

## Documentation Map

`appkit` never imports `webtyp.com/layout`.

| File | What |
|---|---|
| [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md) | Design and decisions |
| [`app.go`](app.go) | `App`, `Module` |
| [`module_server.go`](module_server.go) | `ModuleServer`, `Deps`, `Find`, `Entry`, `Parts` |
| [`screens.go`](screens.go) | `Screens`, `EntryScreens` |
| [`brand.go`](brand.go) / [`brand_ssr.go`](brand_ssr.go) | `Brand` and loading HTML |
| [`env.go`](env.go) / [`serve.go`](serve.go) | Server environment, server build (`Server`, `BuildServer`) |
| [`server.go`](server.go) | `MountServer`: rpc, MCP, `Mounts` |
