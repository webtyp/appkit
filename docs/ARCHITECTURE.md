# webtyp/appkit Architecture

> STATUS (remove this line when APP_COMPOSITION_MASTER_PLAN closes): session, permissions and first-admin
> pieces are still being added; today appkit ships the server half only.

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
MCP closed), `/_routes` protected with `Requires(<introspection resource>, model.Read)` in every mode, the
`APIModules`, and the app's own `Register(r)`. A nil `Authorize` becomes `DenyAll` (and logs every denial).

It deliberately has no `httpd`, `env`, `dev.go` or `prod.go`: the concrete transport lives in `webtyp/server` (or
the edge runtime), so the same mount runs in a local server and in a Worker, and tests use `router/mock`.

**appkit never chooses the database.** The app opens its connection on the engine it picked (postgres, sqlite,
indexdb, `storage/mem`) and hands it to appkit through the `storage` port; appkit may build on the engine-agnostic
contracts (`storage`, `orm`, `changelog`) but never imports a concrete engine.
