//go:build !wasm

package appkit

import (
	"webtyp.com/env"
	"webtyp.com/events"
	eventsmock "webtyp.com/events/mock"
	"webtyp.com/fmt"
	"webtyp.com/mcp"
	"webtyp.com/model"
	"webtyp.com/orm"
	"webtyp.com/postgres"
	"webtyp.com/router"
	"webtyp.com/rpc"
	"webtyp.com/server/httpd"
	"webtyp.com/unixid"
)

// ServerConfig is everything an app tells appkit about its server.
type ServerConfig struct {
	Name       string                            // the app's name; MCP server name
	Port       string                            // "" = httpd.DefaultPort
	PublicDir  string                            // "" = httpd.DefaultPublicDir
	NoCache    bool
	DSN        string                            // DATABASE_URL; "" = no database
	Logger     func(...any)
	OpenDB     func(dsn string) (*orm.DB, error) // nil = postgres.Open
	Operations []router.OperationModule          // published at POST /api/<module>/<op>
	MCPTools   []mcp.ToolName                    // operations an agent may call; empty = MCP closed
	Authn      router.Middleware                 // nil = anonymous
	Authorize  model.Authorizer                  // nil = deny everything (and log it)
	APIModules []router.APIModule
	Register   func(r router.Router)             // the app's own routes; nil = none
	IDs        model.IDGenerator                 // nil = unixid
	Events     events.Publisher                  // nil = in-process broker
}

const ResourceAPIExplorer model.Resource = "api_explorer"

const (
	argPort      = "server_port"
	argPublicDir = "server_public_dir"
	argNoCache   = "server_no_cache"
)

// ReadEnv fills Port (-server_port arg, else PORT env), PublicDir (-server_public_dir), NoCache (-server_no_cache=true), DSN (DATABASE_URL) when empty.
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

	if cfg.DSN == "" {
		cfg.DSN = env.Get("DATABASE_URL")
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

	openDB := cfg.OpenDB
	if openDB == nil {
		openDB = func(dsn string) (*orm.DB, error) {
			if dsn == "" {
				if cfg.Logger != nil {
					cfg.Logger("appkit: DATABASE_URL is not set — starting without a database connection (safe default).")
				}
				return nil, nil
			}
			conn, err := postgres.Open(dsn)
			if err != nil {
				return nil, fmt.Err("appkit: DATABASE_URL is set but Postgres refused the connection: ", err)
			}
			return orm.New(conn), nil
		}
	}
	db, err := openDB(cfg.DSN)
	if err != nil {
		return nil, err
	}

	ids := cfg.IDs
	if ids == nil {
		u, err := unixid.NewUnixID()
		if err != nil {
			return nil, err
		}
		ids = u
	}
	pub := cfg.Events
	if pub == nil {
		pub = &eventsmock.Broker{}
	}

	_ = ids
	_ = pub

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
		providers(db),
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

	// D5: valida RBAC fail-fast, en tiempo de BuildServer, no de ListenAndServe.
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
