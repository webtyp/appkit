//go:build !wasm

package appkit

import (
	"webtyp.com/mcp"
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
}
