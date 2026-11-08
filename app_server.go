package appkit

import (
	"webtyp.com/crud"
	"webtyp.com/mcp"
	"webtyp.com/model"
	"webtyp.com/router"
	"webtyp.com/storage"
)

// AppServer is what only the server binary needs.
//
//	var server = appkit.AppServer{Database: postgres.Open}
type AppServer struct {
	Database func(dsn string) (storage.Conn, error) // the driver (postgres.Open); nil = no database
	MCPTools []mcp.ToolName                         // operations an agent may call; empty = MCP closed
	Routes   func(r router.Router)                  // the app's own routes; nil = none
	Infra    []Infra                                // the only modules that get the whole Router (sessions, files); written by the app
	Public   []crud.Public                          // actions visitors may call without a session; nil = none
	Grants   interface {
		EnsureGrants(projectID string, role model.RoleCode, grants []model.Grant) error
	} // the app's rbac service; required when any module declares DefaultGrants
}

// Infra is a module with its own HTTP routes (login, logout, file upload), mounted only
// because the app lists it. It is built after App.Modules, so it can Find what they built.
//
//	Infra: []appkit.Infra{access.Infra}
type Infra struct {
	Name  string                                  // identity in error messages
	Build func(d Deps) (router.APIModule, error)
}
