//go:build !browser

package appkit

import (
	"webtyp.com/mcp"
	"webtyp.com/model"
	"webtyp.com/router"
	"webtyp.com/rpc"
)

// ServerConfig is everything an app tells appkit about its server contracts.
type ServerConfig struct {
	Name       string                   // the app's name; MCP server name
	Authorize  model.Authorizer         // nil = deny everything (and log it)
	Policy     model.PolicyDescriber    // optional: role permissions description for /_routes
	Operations []router.OperationModule // published at POST /api/<module>/<op>
	MCPTools   []mcp.ToolName           // operations an agent may call; empty = MCP closed
	APIModules []router.APIModule       // modules with their own routes (MountAPI)
	Register   func(r router.Router)    // the app's own routes; nil = none
	Logger     func(...any)
}

// MountServer mounts the application contracts onto r.
func MountServer(r router.Router, cfg ServerConfig) error {
	authorize := cfg.Authorize
	if authorize == nil {
		authorize = DenyAll(cfg.Logger)
	}

	if len(cfg.Operations) > 0 {
		if err := rpc.Mount(r, rpc.DefaultPrefix, cfg.Operations...); err != nil {
			return err
		}
	}

	provider, err := mcp.HarvestOps(cfg.MCPTools, cfg.Operations...)
	if err != nil {
		return err
	}
	mcpServer, err := mcp.NewServer(
		mcp.Config{Name: cfg.Name, Version: "0.0.1", Authorize: authorize},
		[]mcp.ToolProvider{provider},
	)
	if err != nil {
		return err
	}
	mcpServer.MountAPI(r)

	router.MountIntrospection(r, router.IntrospectionPath, cfg.Policy).Requires(router.ResourceIntrospection, model.Read)

	for _, m := range cfg.APIModules {
		if m != nil {
			m.MountAPI(r)
		}
	}

	if cfg.Register != nil {
		cfg.Register(r)
	}

	return nil
}

// DenyAll is the default-deny Authorizer used when ServerConfig.Authorize is nil.
func DenyAll(logger func(...any)) model.Authorizer {
	return func(userID string, resource model.Resource, action model.Action) bool {
		if logger != nil {
			logger("appkit: DENY (no Authorize configured) — resource:", resource, "action:", action, "userID:", userID)
		}
		return false
	}
}
