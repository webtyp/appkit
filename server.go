//go:build !browser

package appkit

import (
	"webtyp.com/crud"
	"webtyp.com/events"
	"webtyp.com/fmt"
	"webtyp.com/mcp"
	"webtyp.com/model"
	"webtyp.com/router"
	"webtyp.com/rpc"
)

// ServerConfig is everything an app tells appkit about its server contracts.
type ServerConfig struct {
	Name        string                                                      // the app's name; MCP server name
	Authorize   model.Authorizer                                            // nil = deny everything (and log it)
	Policy      model.PolicyDescriber                                       // optional: role permissions description for /_routes
	Operations  []crud.Module                                               // published at POST /api/<module>/<op>
	Events      events.Publisher                                            // the broker writes announce on (crud publishes after each commit); nil = nobody listens
	Public      []crud.Public                                               // actions visitors may call without a session; nil = none
	MCPTools    []mcp.ToolName                                              // operations an agent may call; empty = MCP closed
	APIModules  []router.APIModule                                          // built from AppServer.Infra
	Mounts      []func(r router.Router, policy model.PolicyDescriber) error // extra routes the app's main passes (dev tools); nil = none
	Register    func(r router.Router)                                       // the app's own routes; nil = none
	Logger      func(...any)
}

// MountServer mounts the application contracts onto r.
func MountServer(r router.Router, cfg ServerConfig) error {
	authorize := cfg.Authorize
	if authorize == nil {
		authorize = DenyAll(cfg.Logger)
	}

	if len(cfg.Operations) > 0 {
		if err := rpc.Mount(r, rpc.DefaultPrefix, cfg.Logger, cfg.Public, cfg.Events, cfg.Operations...); err != nil {
			return err
		}
	}

	if err := checkPublic(cfg.Public, cfg.Operations); err != nil {
		return err
	}

	provider, err := mcp.HarvestOps(cfg.Events, cfg.MCPTools, cfg.Operations...)
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

	for _, m := range cfg.APIModules {
		if m != nil {
			m.MountAPI(r)
		}
	}

	for _, mount := range cfg.Mounts {
		if err := mount(r, cfg.Policy); err != nil {
			return err
		}
	}

	if cfg.Register != nil {
		cfg.Register(r)
	}

	return nil
}

func checkPublic(public []crud.Public, ops []crud.Module) error {
	if len(public) == 0 {
		return nil
	}
	routes, err := rpc.Routes(rpc.DefaultPrefix, public, ops...)
	if err != nil {
		return err
	}
	actions := []model.Action{model.Create, model.Read, model.Update, model.Delete}
	for _, p := range public {
		suffix := "/" + p.Model.ModelName()
		for _, act := range actions {
			if !p.Actions.Has(act) {
				continue
			}
			method := rpc.MethodOf(act)
			found := false
			for _, r := range routes {
				if r.Access == model.AccessPublic && r.Method == method && fmt.HasSuffix(r.Path, suffix) {
					found = true
					break
				}
			}
			if !found {
				return fmt.ErrType(fmt.Err(p.Model.ModelName()), ErrPublicNotMounted)
			}
		}
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
