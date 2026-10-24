//go:build !wasm

package config

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

	"github.com/veltylabs/mjosefa-cms/modules"
)

const (
	// DefaultPort es el puerto del servidor cuando no se indica otro.
	// Ejemplo: cfg := config.ServerConfig{Port: config.DefaultPort}
	DefaultPort = httpd.DefaultPort
	// DefaultPublicDir es la carpeta que el servidor sirve cuando no se indica otra.
	// Ejemplo: cfg := config.ServerConfig{PublicDir: config.DefaultPublicDir}
	DefaultPublicDir = httpd.DefaultPublicDir
)

// ServerConfig es lo que main() resuelve desde el entorno.
// También lleva las dependencias inyectables (antes ServerDeps)
// para una configuración limpia en un único struct.
//
// Ejemplo: srv, err := config.BuildServer(config.ServerConfig{Port: config.DefaultPort})
type ServerConfig struct {
	Port      string
	PublicDir string
	NoCache   bool
	DSN       string // DATABASE_URL
	Logger    func(...any)

	// Dependencias inyectables
	OpenDB     func(dsn string) (*orm.DB, error)   // nil → postgres.Open
	Operations []router.OperationModule            // nil → defaultOperations: las de modules.NewBackends; se publican por rpc
	Providers  func(db *orm.DB) []mcp.ToolProvider // nil → derivado de Operations con MCPTools()
	Authn      router.Middleware                   // nil → anónimo
	Authorize  model.Authorizer                    // nil → denegar todo (ver D1)
	APIModules []router.APIModule                  // nil → ninguno
	IDs        model.IDGenerator                   // nil → unixid.NewUnixID()
	Events     events.Publisher                    // nil → broker in-proc
}

const (
	argPort      = "server_port"
	argPublicDir = "server_public_dir"
	argNoCache   = "server_no_cache"
)

// DefaultServerConfig resuelve la configuración por defecto leyendo el entorno.
// Ejemplo: cfg, err := config.DefaultServerConfig()
func DefaultServerConfig() (ServerConfig, error) {
	port := env.Arg(argPort)
	if port == "" {
		port = env.Get("PORT")
	}
	if port == "" {
		port = DefaultPort
	}
	publicDir := env.Arg(argPublicDir)
	if publicDir == "" {
		publicDir = DefaultPublicDir
	}
	cfg := ServerConfig{
		Port:      port,
		PublicDir: publicDir,
		NoCache:   env.Arg(argNoCache) == "true",
		Logger:    func(a ...any) { fmt.Println(a...) },
	}

	dsn := env.Get("DATABASE_URL")
	if dsn == "" {
		fmt.Println("app: DATABASE_URL is not set — starting without a database connection (safe default). To enable persistence: add DATABASE_URL to .env (see docs/DESARROLLO_LOCAL.md), run `go run ./cmd/migrate` once, then restart.")
		cfg.OpenDB = func(string) (*orm.DB, error) { return nil, nil }
		return cfg, nil
	}
	cfg.DSN = dsn

	conn, err := postgres.Open(dsn)
	if err != nil {
		return cfg, fmt.Err("app: DATABASE_URL is set but Postgres refused the connection: ", err, " — check the server is reachable, or comment the DATABASE_URL line out of .env to start without a database")
	}
	db := orm.New(conn)

	return ProductionDeps(db, cfg)
}

// RunServer = BuildServer + ListenAndServe. Es lo único que invoca main().
// Si se llama sin argumentos, resuelve DefaultServerConfig().
//
// Ejemplo: err := config.RunServer()
func RunServer(cfgs ...ServerConfig) error {
	var cfg ServerConfig
	if len(cfgs) > 0 {
		cfg = cfgs[0]
	} else {
		var err error
		cfg, err = DefaultServerConfig()
		if err != nil {
			return err
		}
	}
	srv, err := BuildServer(cfg)
	if err != nil {
		return err
	}
	return srv.ListenAndServe()
}

// BuildServer arma el servidor SIN escuchar. Puro y testeable con
// httptest.
//
// Ejemplo: srv, err := config.BuildServer(cfg)
func BuildServer(cfg ServerConfig) (*httpd.Server, error) {
	if err := validateAuthorize(cfg); err != nil {
		return nil, err
	}

	openDB := cfg.OpenDB
	if openDB == nil {
		openDB = func(dsn string) (*orm.DB, error) {
			conn, err := postgres.Open(dsn)
			if err != nil {
				return nil, err
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

	ops := cfg.Operations
	if ops == nil {
		if ops, err = defaultOperations(db, ids, pub, modules.TenantID); err != nil {
			return nil, err
		}
	}
	providers := cfg.Providers
	if providers == nil {
		provider, err := mcp.HarvestOps(MCPTools(), ops...)
		if err != nil {
			return nil, err
		}
		providers = func(*orm.DB) []mcp.ToolProvider { return []mcp.ToolProvider{provider} }
	}

	authorize := cfg.Authorize
	if authorize == nil {
		authorize = denyAll(cfg.Logger)
	}

	mcpServer, err := mcp.NewServer(
		mcp.Config{Name: modules.TenantID, Version: "0.0.1", Authorize: authorize},
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

	if err := rpc.Mount(srv.Router(), rpc.DefaultPrefix, ops...); err != nil {
		return nil, err
	}
	introspectionAccess(router.MountIntrospection(srv.Router(), router.IntrospectionPath, nil))
	mcpServer.MountAPI(srv.Router())
	for _, m := range cfg.APIModules {
		if m != nil {
			m.MountAPI(srv.Router())
		}
	}
	Register(srv.Router())

	// D5: valida RBAC fail-fast, en tiempo de BuildServer, no de ListenAndServe.
	if _, err := srv.Handler(); err != nil {
		return nil, err
	}

	return srv, nil
}

// denyAll es el Authorize de denegación-por-defecto usado cuando
// ServerConfig.Authorize es nil.
func denyAll(logger func(...any)) model.Authorizer {
	return func(userID string, resource model.Resource, action model.Action) bool {
		if logger != nil {
			logger("app: DENY (no Authorize configured) — resource:", resource, "action:", action, "userID:", userID)
		}
		return false
	}
}

// defaultOperations son las operaciones de dominio de modules.NewBackends: lo que el servidor
// publica por rpc y de lo que se deriva la herramienta MCP (filtrada por MCPTools()). Los módulos
// de transporte (auth/userserver) no están aquí: implementan router.APIModule y se montan en el
// Router de httpd (ServerConfig.APIModules).
func defaultOperations(db *orm.DB, ids model.IDGenerator, pub events.Publisher, tenantID string) ([]router.OperationModule, error) {
	if db == nil {
		return nil, nil
	}
	b, err := modules.NewBackends(db, ids, pub, tenantID)
	if err != nil {
		return nil, err
	}
	return b.Operations(), nil
}
