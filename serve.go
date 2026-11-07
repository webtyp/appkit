//go:build !wasm

package appkit

import (
	"webtyp.com/crud"
	"webtyp.com/ddl"
	"webtyp.com/env"
	eventsmock "webtyp.com/events/mock"
	"webtyp.com/fmt"
	"webtyp.com/model"
	"webtyp.com/orm"
	"webtyp.com/router"
	"webtyp.com/server/httpd"
	"webtyp.com/storage"
	"webtyp.com/time"
	"webtyp.com/unixid"
)

// Process arguments the dev environment and the self-host supervisor pass to web/server.
const (
	ArgPort      = "server_port"
	ArgPublicDir = "server_public_dir"
	ArgNoCache   = "server_no_cache"
)

const msgNoDatabase = "appkit: DATABASE_URL is not set — starting without a database; modules are not built"

// Leftover column lines (D5): production never drops a column with data, it says so at every start.
const (
	msgLeftover         = "appkit: leftover column, the models no longer have it but it holds data:"
	msgLeftoverRows     = "rows:"
	msgLeftoverMore     = "or more"
	msgLeftoverMovedTo  = "copied to:"
	msgLeftoverNote     = "note:"
	msgLeftoversSkipped = "appkit: leftover columns not checked:"
	msgNoDDLCompiler    = "the database connection does not implement ddl.Compiler"
	columnSeparator     = "."
)

// ServerEnv is what the process tells the server: where to listen, what to
// serve and which database to open ("" = none).
type ServerEnv struct {
	Port      string
	PublicDir string
	NoCache   bool
	DSN       string
}

// buildModules builds app.Modules in order on d.DB, appending each to d.built (so later modules Find it).
// It returns the crud modules among them, for BuildServer to mount.
func buildModules(app App, d *Deps) (ops []crud.Module, err error) {
	if d.built == nil {
		builtSlice := make([]model.ModuleNaming, 0, len(app.Modules))
		d.built = &builtSlice
	}
	appendBuilt := func(v model.ModuleNaming) {
		*d.built = append(*d.built, v)
		if o, ok := v.(crud.Module); ok {
			ops = append(ops, o)
		}
	}
	for _, m := range app.Modules {
		if m.Server.Build == nil {
			continue
		}
		built, err := m.Server.Build(*d)
		if err != nil {
			return nil, fmt.ErrType(fmt.Err(m.Name), err)
		}
		_, isCrud := built.(crud.Module)
		_, isParts := built.(Parts)
		if !isCrud && !isParts {
			return nil, fmt.ErrType(fmt.Err(m.Name), ErrNotCrudModule)
		}
		appendBuilt(built)
		if p, ok := built.(Parts); ok {
			for _, part := range p.Parts() {
				appendBuilt(part)
			}
		}
	}
	return ops, nil
}

func buildInfra(app App, d Deps) ([]router.APIModule, error) {
	var apis []router.APIModule
	for _, i := range app.Server.Infra {
		if i.Build == nil {
			continue
		}
		built, err := i.Build(d)
		if err != nil {
			return nil, fmt.ErrType(fmt.Err(i.Name), err)
		}
		if built != nil {
			apis = append(apis, built)
		}
	}
	return apis, nil
}

// BuildServer builds the whole server without listening: database, modules in
// App.Modules order, auth, and every contract MountServer mounts.
//
// With a database it also logs, one line each, the columns schema's models no longer have but that still hold
// data; it never drops them and never refuses to start.
//
//	srv, err := appkit.BuildServer(config.App, appkit.ServerEnv{Port: "8080"}, config.Schema)
func BuildServer(app App, se ServerEnv, schema ddl.Schema, mounts ...func(r router.Router, policy model.PolicyDescriber) error) (*httpd.Server, error) {
	if app.Log == nil {
		return nil, ErrNoLog
	}
	logger := app.Log
	ids, err := unixid.NewUnixID()
	if err != nil {
		return nil, err
	}
	zone, err := time.LoadZone(app.Timezone)
	if err != nil {
		return nil, ErrNoTimezone
	}
	d := Deps{IDs: ids, Events: &eventsmock.Broker{}, TenantID: app.TenantID, Zone: zone, Log: logger}

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
		warnLeftovers(conn, schema, logger)
	}

	var ops []crud.Module
	var apis []router.APIModule
	if d.DB != nil {
		if ops, err = buildModules(app, &d); err != nil {
			return nil, err
		}
		if apis, err = buildInfra(app, d); err != nil {
			return nil, err
		}
	}

	var (
		authn     router.Middleware
		authorize = DenyAll(logger)
		policy    model.PolicyDescriber
	)
	if d.DB != nil {
		entry, err := Find[Entry](d)
		if err != nil {
			if isErr(err, ErrNoProvider) {
				logger("appkit: no entry module — every operation is denied")
			} else {
				return nil, fmt.ErrType(fmt.Err("entry"), err)
			}
		} else {
			authn = entry.Authn()
			authorize = entry.Authorize()
			policy = entry.Policy()
		}
	}
	if authorize == nil {
		authorize = DenyAll(logger)
	}

	srv := httpd.New(httpd.Config{
		Port: se.Port, PublicDir: se.PublicDir, Gzip: true, NoCache: se.NoCache, Health: true,
		Logger: logger, Authn: authn, Authorize: authorize, Policy: policy,
	})
	if err := MountServer(srv.Router(), ServerConfig{
		Name:        app.TenantID,
		Authorize:   authorize,
		Policy:      policy,
		Operations:  ops,
		PublicReads: app.Server.PublicReads,
		MCPTools:    app.Server.MCPTools,
		APIModules:  apis,
		Mounts:      mounts,
		Register:    app.Server.Routes,
		Logger:      logger,
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
//
//	func main() { if err := appkit.Server(config.App, config.Schema, devtools.Server(config.Schema)...); err != nil { log.Fatal(err) } }
func Server(app App, schema ddl.Schema, mounts ...func(r router.Router, policy model.PolicyDescriber) error) error {
	srv, err := BuildServer(app, processEnv(), schema, mounts...)
	if err != nil {
		return err
	}
	return srv.ListenAndServe()
}

// warnLeftovers logs each column the database keeps although no model of schema has it, because it holds data
// (ddl.Leftovers, read-only). A database that cannot be checked is logged too; neither stops the server.
func warnLeftovers(conn storage.Conn, schema ddl.Schema, logger func(...any)) {
	if len(schema.Steps) == 0 {
		return
	}
	compiler, ok := conn.(ddl.Compiler)
	if !ok {
		logger(msgLeftoversSkipped, msgNoDDLCompiler)
		return
	}
	tables, err := ddl.Capture(schema.Steps)
	if err != nil {
		logger(msgLeftoversSkipped, err)
		return
	}
	left, err := ddl.New(conn, compiler).Leftovers(tables, schema.Removed)
	if err != nil {
		logger(msgLeftoversSkipped, err)
		return
	}
	for _, l := range left {
		line := []any{msgLeftover, l.Table + columnSeparator + l.Column, msgLeftoverRows, l.Rows}
		if l.More {
			line = append(line, msgLeftoverMore)
		}
		if l.Removed.MovedTo != "" {
			line = append(line, msgLeftoverMovedTo, l.Removed.MovedTo)
		}
		if l.Removed.Note != "" {
			line = append(line, msgLeftoverNote, l.Removed.Note)
		}
		logger(line...)
	}
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

func isErr(err, target error) bool {
	if err == target {
		return true
	}
	if u, ok := err.(interface{ Unwrap() error }); ok {
		return isErr(u.Unwrap(), target)
	}
	if u, ok := err.(interface{ Unwrap() []error }); ok {
		for _, e := range u.Unwrap() {
			if isErr(e, target) {
				return true
			}
		}
	}
	return false
}
