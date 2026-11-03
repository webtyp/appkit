//go:build !wasm

package appkit

import (
	"fmt"

	"webtyp.com/env"
	eventsmock "webtyp.com/events/mock"
	"webtyp.com/orm"
	"webtyp.com/router"
	"webtyp.com/server/httpd"
	"webtyp.com/unixid"
)

// Process arguments the dev environment and the self-host supervisor pass to web/server.
const (
	ArgPort      = "server_port"
	ArgPublicDir = "server_public_dir"
	ArgNoCache   = "server_no_cache"
)

const msgNoDatabase = "appkit: DATABASE_URL is not set — starting without a database; modules are not built"

// ServerEnv is what the process tells the server: where to listen, what to
// serve and which database to open ("" = none).
type ServerEnv struct {
	Port      string
	PublicDir string
	NoCache   bool
	DSN       string
}

// BuildServer builds the whole server without listening: database, modules in
// App.Modules order, auth, and every contract MountServer mounts.
//
//	srv, err := appkit.BuildServer(config.App, appkit.ServerEnv{Port: "8080"})
func BuildServer(app App, se ServerEnv) (*httpd.Server, error) {
	logger := loggerOf(app)
	ids, err := unixid.NewUnixID()
	if err != nil {
		return nil, err
	}
	d := Deps{IDs: ids, Events: &eventsmock.Broker{}, TenantID: app.TenantID, Timezone: app.Timezone}

	if se.DSN == "" {
		logger(msgNoDatabase)
	} else {
		if app.Server.Database == nil {
			return nil, ErrNoOpen
		}
		conn, err := app.Server.Database(se.DSN)
		if err != nil {
			return nil, err
		}
		d.DB = orm.New(conn)
	}

	var ops []router.OperationModule
	var apis []router.APIModule
	if d.DB != nil {
		for _, m := range app.Modules {
			if m.Server.Build == nil {
				continue
			}
			built, err := m.Server.Build(d)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", m.Name, err)
			}
			d.built = append(d.built, built)
			if o, ok := built.(router.OperationModule); ok {
				ops = append(ops, o)
			}
			if a, ok := built.(router.APIModule); ok {
				apis = append(apis, a)
			}
		}
	}

	var au serverAuth
	if d.DB != nil {
		switch l := app.Login.(type) {
		case LoginByRUT:
			if au, err = l.build(d, logger); err != nil {
				return nil, fmt.Errorf("login: %w", err)
			}
		}
	}
	authorize := au.Authorize
	if authorize == nil {
		authorize = DenyAll(logger)
	}

	srv := httpd.New(httpd.Config{
		Port: se.Port, PublicDir: se.PublicDir, Gzip: true, NoCache: se.NoCache, Health: true,
		Logger: logger, Authn: au.Authn, Authorize: authorize, Policy: au.Policy,
	})
	if err := MountServer(srv.Router(), ServerConfig{
		Name:       app.TenantID,
		Authorize:  authorize,
		Policy:     au.Policy,
		Operations: append(ops, au.Operations...),
		MCPTools:   app.Server.MCPTools,
		APIModules: append(apis, au.APIModules...),
		Register:   app.Server.Routes,
		Migrations: Steps(app),
		Logger:     logger,
	}); err != nil {
		return nil, err
	}
	if _, err := srv.Handler(); err != nil {
		return nil, err
	}
	return srv, nil
}

// Server reads the process arguments and DATABASE_URL, builds the server and
// listens. It is the whole body of an app's web/server/main.go.
func Server(app App) error {
	srv, err := BuildServer(app, processEnv())
	if err != nil {
		return err
	}
	return srv.ListenAndServe()
}

func processEnv() ServerEnv {
	se := ServerEnv{
		Port:      env.Arg(ArgPort),
		PublicDir: env.Arg(ArgPublicDir),
		NoCache:   env.Arg(ArgNoCache) == "true",
		DSN:       env.Get(EnvDatabaseURL),
	}
	if se.Port == "" {
		se.Port = httpd.DefaultPort
	}
	if se.PublicDir == "" {
		se.PublicDir = httpd.DefaultPublicDir
	}
	return se
}
