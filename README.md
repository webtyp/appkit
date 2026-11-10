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

- **Isomorphic separation**: [`app_server.go`](app_server.go), [`module_server.go`](module_server.go), and [`server.go`](server.go) are tagged `//go:build !browser` (`serve.go` is `!wasm`). In browser WASM, empty struct stand-ins ([`app_server_browser.go`](app_server_browser.go), [`module_server_browser.go`](module_server_browser.go)) keep types defined without compiling server dependencies.
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

2. The app declares itself with [`view.Brand`](https://github.com/webtyp/view):

```go
// config/app.go (no tag)
var App = appkit.App{
	TenantID: "acme",
	Timezone: "America/Santiago",
	Brand:    view.Brand{Name: "Acme", LogoMark: "logo.svg"},
	Modules:  []appkit.Module{access.Module, catalog.Module},
	Server:   server,
	Log:      fmt.Println,
}

// config/server.go (//go:build !browser)
var server = appkit.AppServer{
	Database: postgres.Open,
	Infra:    []appkit.Infra{access.Infra},
	Public: []crud.Public{
		{Model: &catalog.Category{}, Actions: model.Read},
		{Model: &contact.Message{}, Actions: model.Create},
	},
	Grants: rbacService,
}

// config/browser.go (//go:build browser)
var server appkit.AppServer
```

`Log` is **required** (`log.Println`): the server writes its warnings there and refuses to start without it (`ErrNoLog`).
`Timezone` is **required**: an IANA name (`America/Santiago`) loaded once for both `Deps.Zone` and `view.BrowserDeps.Zone` (`ErrNoTimezone`).

### Server Events and Transports

`BuildServer` initializes `Deps.Events`: the broker writes announce on (`crud` publishes after each commit). `MountServer` passes `ServerConfig.Events` to `rpc.Mount` and `mcp.HarvestOps` so every write arriving by HTTP or by an agent announces itself:

```go
cfg := appkit.ServerConfig{
	Name:       app.TenantID,
	Operations: ops,
	Events:     d.Events, // broker writes announce on
	Public:     app.Server.Public,
}
```

What any signed-in person may do is declared in `ModuleServer.DefaultGrants`, merged at boot and ensured via `AppServer.Grants`.

3. The generated mains:

```go
// web/server/main.go
func main() { if err := appkit.Server(config.App, config.Schema, devtools.Server(config.Schema)...); err != nil { log.Fatal(err) } }

// web/browser/main.go
func main() { appkit.Browser(config.App, layoutapp.New(devtools.Menu())) }
```

`appkit.Browser(config.App, layout)` compiles `view.Session` through `appkit.NewSession` and hands it to `view.Layout`.

## Documentation Map

`appkit` never imports `webtyp.com/layout`.

| File | What |
|---|---|
| [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md) | Design and decisions |
| [`docs/TESTS.md`](docs/TESTS.md) | Test suite inventory and coverage |
| [`app.go`](app.go) | `App`, `Module` |
| [`session.go`](session.go) | `NewSession` (`view.Session`) |
| [`browser.go`](browser.go) | `Browser` (`view.Layout`) |
| [`module_server.go`](module_server.go) | `ModuleServer`, `Deps`, `Find`, `Entry`, `Parts` |
| [`screens.go`](screens.go) | `Screens`, `EntryScreens` |
| [`brand.go`](brand.go) / [`brand_ssr.go`](brand_ssr.go) | Access screen copy and SSR pre-rendering |
| [`env.go`](env.go) / [`serve.go`](serve.go) | Server environment, server build (`Server`, `BuildServer`) |
| [`server.go`](server.go) | `MountServer`: rpc, MCP, `Mounts` |
