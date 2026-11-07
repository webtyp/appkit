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
concrete transport: it mounts rpc operations (`crud.Module`), the MCP server (only the tools named in `MCPTools`; empty =
MCP closed), the `APIModules` (built from `AppServer.Infra`), extra routes passed in `Mounts` (dev tools), and the app's own `Register(r)`.
A nil `Authorize` becomes `DenyAll` (and logs every denial).

Domain modules never receive `router.Router`: they are mounted strictly as `crud.Module`. Routes with the full router come only from `AppServer.Infra`, which the **app** writes in `config/`. Public read operations are declared explicitly by the app in `AppServer.PublicReads`. Build order is `Modules` → `Infra` → `MountServer`.

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
server never links a screen. `Browser` is `wasm` only. Rejected: two values (`App` + `Browser`) — every module
was listed twice and the browser main referenced a symbol gopls could not see.

Build order is `App.Modules` order; `Find` sees only modules built before it. The zone is loaded once by appkit; modules never load it. No database means no module built (the
server still serves static files). Database migrations live in `webtyp.com/migrate` and dev tools in `webtyp.com/devtools`.

One construction path: `buildModules` is shared by `BuildServer` and `BuildModules`; server-only files.

Leftover columns: `BuildServer` receives the app's `ddl.Schema` and, once the database is open, logs one line per
column its models no longer have but that still holds data (`ddl.Leftovers`, read-only). Production never drops a
column — that is a person's decision in `webtyp dev` (`migrate -drop`) — and a database that cannot be checked is
logged, never a reason not to start.

## Entry

Entry is a module in `App.Modules` (`webtyp.com/access/module`). On the server, `appkit.Find[appkit.Entry]`
finds it among built modules and obtains `Authn`, `Authorize` and `Policy`. If no entry module is present,
operations are denied by default (`DenyAll`). If the entry module builds multiple domain parts (users, access),
it implements `Parts` so that `appkit.Find`, operations and API routes discover them as if they were top-level modules.
In the browser, the entry module sets `ModuleBrowser.Entry` returning `EntryScreens` (`Login`, `Setup`, `IdleSeconds`).
