# webtyp/appkit Architecture

## Why it exists

Every app with a login glued the same pieces by hand — server, session, permissions, menu, first admin —
and whatever crossed several pieces ("the menu shows only what you may use") belonged to none of them.
appkit **joins** those pieces; it does not own them. Each part stays in its owner (`router`, `rpc`, `mcp`,
`auth`, `rbac`, `layout`, `access`) and the app writes only its configuration.

## The name

"The kit to assemble an app" — precedent: Apple's AppKit/UIKit. Rejected: `appsetup` (clashes with the
first-admin "setup" flow, 42 uses) and `appconfig` (clashes with every app's `config/` folder and with
AWS AppConfig).

## Server half — `MountServer` (`//go:build !browser`)

`MountServer(r router.Router, cfg ServerConfig) error` works on the `router.Router` **contract**, never on a
concrete transport: it mounts rpc operations, the MCP server (only the tools named in `MCPTools`; empty =
MCP closed), `/_routes` protected with `Requires(router.ResourceIntrospection, model.Read)` in every mode, the
`APIModules`, and the app's own `Register(r)`. A nil `Authorize` becomes `DenyAll` (and logs every denial).

It deliberately has no `httpd`, `env`, `dev.go` or `prod.go`: the concrete transport lives in `webtyp/server` (or
the edge runtime), so the same mount runs in a local server and in a Worker, and tests use `router/mock`.

**appkit never chooses the database.** The app opens its connection on the engine it picked (postgres, sqlite,
indexdb, `storage/mem`) and hands it to appkit through the `storage` port; appkit may build on the engine-agnostic
contracts (`storage`, `orm`, `changelog`) but never imports a concrete engine.

## App and Module — one value, two binaries

`App` and `Module` carry no build tag, so `config.App` is the same value in `web/server` and `web/browser`. What belongs
to one binary is a part — `AppServer`, `ModuleServer`, `ModuleBrowser` — whose type exists in every binary (appkit
carries no destination tag, so gopls never sees an undefined symbol). The module and the app fill each part from their
own tagged files and leave it zero in the other binary, so the browser wasm never links a `Build` or a driver, and the
server never links a screen. `RunBrowser` is `wasm` only (it needs `syscall/js`). Rejected: two values (`App` + `Browser`) — every module
was listed twice and the browser main referenced a symbol gopls could not see.

Build order is `App.Modules` order; `Find` sees only modules built before it. No database means no module built (the
server still serves static files and `/_routes`). Migrations are `Steps(app)` — the login's own steps first (`auth`,
`rbac`, `access` for `LoginByRUT`), then each module — the same list `/_schema` draws.

One construction path: `buildModules` and `buildIdentity` are shared by `BuildServer` and `BuildModules`; server-only files.

## Login

`Login` is closed (an unexported method): only appkit implements it. `LoginByRUT` builds `authority`, `rbac` and
`access` after the modules, finds the `auth.TrustedIPStore` among them, and creates the first administrator: user,
RUT identity, optional `FirstAdminRegistrar` (device and record, owned by a domain module), `admin` role with the
wildcard permission. Brand `RenderPages`/`RenderHTML` are read by sitec from the app's `var Brand`; a zero `Brand`
renders nothing.
