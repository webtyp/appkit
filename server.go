//go:build !wasm

package appkit

import (
	"webtyp.com/env"
	"webtyp.com/fmt"
	"webtyp.com/mcp"
	"webtyp.com/model"
	"webtyp.com/orm"
	"webtyp.com/router"
	"webtyp.com/rpc"
	"webtyp.com/server/httpd"
)

// ServerConfig is everything an app tells appkit about its server.
type ServerConfig struct {
	Name       string                            // the app's name; MCP server name
	Port       string                            // "" = httpd.DefaultPort
	PublicDir  string                            // "" = httpd.DefaultPublicDir
	NoCache    bool
	Logger     func(...any)
	DB         *orm.DB                           // optional pre-opened database instance (postgres, d1, sqlite, mem)
	Operations []router.OperationModule          // published at POST /api/<module>/<op>
	MCPTools   []mcp.ToolName                    // operations an agent may call; empty = MCP closed
	Authn      router.Middleware                 // nil = anonymous
	Authorize  model.Authorizer                  // nil = deny everything (and log it)
	APIModules []router.APIModule                // modules with their own routes (MountAPI)
	Register   func(r router.Router)             // the app's own routes; nil = none
}

const ResourceAPIExplorer model.Resource = "api_explorer"

const (
	argPort      = "server_port"
	argPublicDir = "server_public_dir"
	argNoCache   = "server_no_cache"
)

// ReadEnv fills Port (-server_port arg, else PORT env), PublicDir (-server_public_dir), NoCache (-server_no_cache=true) when empty.
func ReadEnv(cfg ServerConfig) ServerConfig {
	if cfg.Port == "" {
		port := env.Arg(argPort)
		if port == "" {
			port = env.Get("PORT")
		}
		if port == "" {
			port = httpd.DefaultPort
		}
		cfg.Port = port
	}

	if cfg.PublicDir == "" {
		publicDir := env.Arg(argPublicDir)
		if publicDir == "" {
			publicDir = httpd.DefaultPublicDir
		}
		cfg.PublicDir = publicDir
	}

	if !cfg.NoCache {
		cfg.NoCache = env.Arg(argNoCache) == "true"
	}

	if cfg.Logger == nil {
		cfg.Logger = func(a ...any) { fmt.Println(a...) }
	}

	return cfg
}

// BuildServer arma el servidor SIN escuchar.
func BuildServer(cfg ServerConfig) (*httpd.Server, error) {
	if err := validateAuthorize(cfg); err != nil {
		return nil, err
	}

	provider, err := mcp.HarvestOps(cfg.MCPTools, cfg.Operations...)
	if err != nil {
		return nil, err
	}
	providers := func(*orm.DB) []mcp.ToolProvider { return []mcp.ToolProvider{provider} }

	authorize := cfg.Authorize
	if authorize == nil {
		authorize = denyAll(cfg.Logger)
	}

	mcpServer, err := mcp.NewServer(
		mcp.Config{Name: cfg.Name, Version: "0.0.1", Authorize: authorize},
		providers(cfg.DB),
	)
	if err != nil {
		return nil, err
	}

	srv := httpd.New(httpd.Config{
		Port:      cfg.Port,
		PublicDir: cfg.PublicDir,
		Gzip:      true,
		NoCache:   cfg.NoCache,
		Health:    true,
		Logger:    cfg.Logger,
		Authn:     cfg.Authn,
		Authorize: authorize,
	})

	if len(cfg.Operations) > 0 {
		if err := rpc.Mount(srv.Router(), rpc.DefaultPrefix, cfg.Operations...); err != nil {
			return nil, err
		}
	}
	introspectionAccess(router.MountIntrospection(srv.Router(), router.IntrospectionPath, nil))
	mcpServer.MountAPI(srv.Router())
	for _, m := range cfg.APIModules {
		if m != nil {
			m.MountAPI(srv.Router())
		}
	}
	if cfg.Register != nil {
		cfg.Register(srv.Router())
	}

	// Valida RBAC fail-fast en tiempo de armado.
	if _, err := srv.Handler(); err != nil {
		return nil, err
	}

	return srv, nil
}

// RunServer = BuildServer + ListenAndServe.
func RunServer(cfg ServerConfig) error {
	srv, err := BuildServer(cfg)
	if err != nil {
		return err
	}
	return srv.ListenAndServe()
}

// denyAll es el Authorize de denegación-por-defecto usado cuando
// ServerConfig.Authorize es nil.
func denyAll(logger func(...any)) model.Authorizer {
	return func(userID string, resource model.Resource, action model.Action) bool {
		if logger != nil {
			logger("appkit: DENY (no Authorize configured) — resource:", resource, "action:", action, "userID:", userID)
		}
		return false
	}
}
