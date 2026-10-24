//go:build !wasm

package config

import "webtyp.com/mcp"

// MCPTools son las operaciones que un agente puede usar por MCP (AS-4). Vacía = ninguna: MCP queda
// cerrado. Para abrir una: mcp.ToolNameOf("<módulo>", "<operación>").
//
// Ejemplo: provider, err := mcp.HarvestOps(config.MCPTools(), ops...)
func MCPTools() []mcp.ToolName { return nil }
