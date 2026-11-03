//go:build !wasm

package appkit

import (
	"webtyp.com/mcp"
	"webtyp.com/model"
	"webtyp.com/router"
	"webtyp.com/storage"
)

// App is the whole server side of an application: its name, its installation
// values, its database driver and the list of its modules, in order.
type App struct {
	Name     string                                 // MCP server name
	Tenant   string                                 // Deps.Tenant for every module
	Timezone string                                 // Deps.Timezone, e.g. "America/Santiago"
	Open     func(dsn string) (storage.Conn, error) // the app's database driver (postgres.Open); nil = no database
	Modules  []Module                               // build and migration order
	Auth     func(d Deps) (Auth, error)             // runs after every module is built; nil = anonymous, deny all
	MCPTools []mcp.ToolName                         // operations an agent may call; empty = MCP closed
	Register func(r router.Router)                  // the app's own routes; nil = none
	Logger   func(...any)                           // nil = discard
}

// Auth is what App.Auth returns: who the caller is and what they may do.
type Auth struct {
	Authn      router.Middleware
	Authorize  model.Authorizer // nil = deny all
	Policy     model.PolicyDescriber
	APIModules []router.APIModule // auth's own endpoints (login, me, logout)
}
