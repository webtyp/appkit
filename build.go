//go:build !browser

package appkit

import (
	"webtyp.com/backoff"
	"webtyp.com/crud"
	"webtyp.com/ddl"
	"webtyp.com/externalcalls"
	"webtyp.com/fmt"
	"webtyp.com/model"
	"webtyp.com/orm"
	"webtyp.com/rbac"
	"webtyp.com/router"
	"webtyp.com/storage"
	"webtyp.com/time"
)

const msgNoDatabase = "appkit: no database address — starting without a database; modules are not built"

// Leftover column lines (D5): production never drops a column with data, it says so at every start.
const (
	msgLeftover         = "appkit: leftover column, the models no longer have it but it holds data:"
	msgLeftoverRows     = "rows:"
	msgLeftoverMore     = "or more"
	msgLeftoverMovedTo  = "copied to:"
	msgLeftoverNote     = "note:"
	msgLeftoversSkipped = "appkit: leftover columns not checked:"
	msgNoDDLCompiler    = "the database connection does not implement ddl.Compiler"
	msgNoSignIn         = "appkit: App.SignIn is empty — nobody can enter and every operation is denied"
	columnSeparator     = "."
)

// Built is the app built but not yet serving: what the server's HTTP layer needs (Authn, Authorize, Policy) and Mount,
// which publishes the operations. The main creates the HTTP server in between, because httpd.Config needs those three.
//
//	built, err := appkit.Build(config.App, config.Backend, config.Server, dsn, config.Schema)
type Built struct {
	Authn     router.Middleware     // who the caller is (session); nil without sign-in
	Authorize model.Authorizer      // what the caller may do; denies everything without sign-in
	Policy    model.PolicyDescriber // which role opens each resource (API explorer); nil when unknown

	app     App
	backend Backend
	d       Deps
	ops     []crud.Module
	api     []router.APIModule // the sign-in's own routes (login, logout, /me)
}

// Build checks what the app, its backend and its server declare, opens the database at dsn, builds the modules in
// App.Modules order, builds the sign-in and starts the external calls. It does not listen.
//
// With a database it also logs, one line each, the columns schema's models no longer have but that still hold
// data; it never drops them and never refuses to start. dsn "" = no database: nothing is built and everything is denied.
func Build(app App, b Backend, s Server, dsn string, schema ddl.Schema) (*Built, error) {
	if err := check(app, b, s); err != nil {
		return nil, err
	}
	ids, err := s.NewIDs()
	if err != nil {
		return nil, err
	}
	zone, err := time.LoadZone(app.Timezone)
	if err != nil {
		return nil, ErrNoTimezone
	}
	d := Deps{IDs: ids, Events: b.Events, TenantID: app.TenantID, Zone: zone, Countries: app.Countries, Log: app.Log}
	out := &Built{app: app, backend: b, Authorize: DenyAll(app.Log)}

	if dsn == "" {
		app.Log(msgNoDatabase)
		out.d = d
		return out, nil
	}
	if s.Database == nil {
		return nil, ErrNoOpen
	}
	conn, err := s.Database(dsn)
	if err != nil {
		return nil, err
	}
	d.DB = orm.New(conn)
	warnLeftovers(conn, schema, app.Log)

	policy, err := backoff.New(time.Second, 60*time.Second)
	if err != nil {
		return nil, err
	}
	q, err := externalcalls.New(d.DB, d.IDs, time.NowNano, policy)
	if err != nil {
		return nil, err
	}
	adapter := NewCallQueue(q)
	d.Queue = adapter
	crud.SetDefaultQueue(adapter)

	ops, err := buildModules(app, b, &d)
	if err != nil {
		return nil, err
	}
	if err := out.entry(&d, b, s, &ops); err != nil {
		return nil, err
	}
	if hasExternalCalls(ops) {
		StartCallExecutor(q, NewCallRunner(ops), d.DB, 5*time.Second, app.Log)
	}
	out.d, out.ops = d, ops
	return out, nil
}

// entry builds the sign-in and the default grants on d.DB: they need rbac, which appkit builds itself.
func (out *Built) entry(d *Deps, b Backend, s Server, ops *[]crud.Module) error {
	var grants []model.Grant
	for _, m := range b.Modules {
		grants = append(grants, m.DefaultGrants...)
	}
	switch out.app.SignIn {
	case 0:
		out.app.Log(msgNoSignIn)
	case SignInByTrustedIP:
	default:
		return ErrUnknownSignIn
	}
	if out.app.SignIn == 0 && len(grants) == 0 {
		return nil
	}
	svc, err := rbac.New(d.DB)
	if err != nil {
		return err
	}
	if len(grants) > 0 {
		if err := svc.EnsureGrants(out.app.TenantID, rbac.RoleAuthenticated, grants); err != nil {
			return err
		}
	}
	if out.app.SignIn == 0 {
		return nil
	}
	si, err := buildSignIn(*d, b.Events, svc, s.SessionIdleMinutes)
	if err != nil {
		return err
	}
	for _, p := range si.parts() {
		*d.built = append(*d.built, builtModule{name: p.ModelName(), v: p})
		if o, ok := p.(crud.Module); ok {
			*ops = append(*ops, o)
		}
	}
	out.api = append(out.api, si.users)
	out.Authn, out.Authorize = si.authn, si.authz
	return nil
}

func check(app App, b Backend, s Server) error {
	switch {
	case app.Log == nil:
		return ErrNoLog
	case b.Events == nil:
		return ErrNoEvents
	case b.API == nil:
		return ErrNoAPI
	case s.NewIDs == nil:
		return ErrNoIDs
	}
	return checkParts(app, b)
}

// checkParts: every module of App.Modules has exactly one BackendModule, and every BackendModule is in App.Modules.
func checkParts(app App, b Backend) error {
	for _, def := range app.Modules {
		if _, ok := partOf(b, def.Name); !ok {
			return fmt.ErrType(fmt.Err(def.Name), ErrNoPart)
		}
	}
	for _, m := range b.Modules {
		listed := false
		for _, def := range app.Modules {
			if def.Name == m.Name {
				listed = true
			}
		}
		if !listed {
			return fmt.ErrType(fmt.Err(m.Name), ErrUnlistedPart)
		}
		if m.Build == nil {
			return fmt.ErrType(fmt.Err(m.Name), ErrNoBuildFunc)
		}
	}
	return nil
}

func partOf(b Backend, name string) (BackendModule, bool) {
	for _, m := range b.Modules {
		if m.Name == name {
			return m, true
		}
	}
	return BackendModule{}, false
}

// buildModules builds app.Modules in order on d.DB, appending each to d.built (so later modules find it).
// It returns the crud modules among them, for Mount.
func buildModules(app App, b Backend, d *Deps) (ops []crud.Module, err error) {
	built := make([]builtModule, 0, len(app.Modules))
	d.built = &built
	add := func(name string, v model.ModuleNaming) {
		built = append(built, builtModule{name: name, v: v})
		d.built = &built
		if o, ok := v.(crud.Module); ok {
			ops = append(ops, o)
		}
	}
	for _, def := range app.Modules {
		part, _ := partOf(b, def.Name)
		v, err := part.Build(*d)
		if err != nil {
			return nil, fmt.ErrType(fmt.Err(def.Name), err)
		}
		_, isCrud := v.(crud.Module)
		p, isParts := v.(Parts)
		if !isCrud && !isParts {
			return nil, fmt.ErrType(fmt.Err(def.Name), ErrNotCrudModule)
		}
		add(def.Name, v)
		if isParts {
			for _, sub := range p.Parts() {
				add(sub.ModelName(), sub)
			}
		}
	}
	return ops, nil
}

// BuildModules builds the app's modules, in App.Modules order, all on db. An import passes its transaction,
// so every write rolls back with it. The sign-in is not built: an import is not a person entering.
//
//	plan := dataimport.Plan[appkit.Deps]{
//		Build: func(tx *orm.DB) (appkit.Deps, error) { return appkit.BuildModules(config.App, config.Backend, config.Server, tx) },
//	}
//	// in a step: m, err := appkit.FindBuiltModule(s.Modules, authority.ModuleName)
func BuildModules(app App, b Backend, s Server, db *orm.DB) (Deps, error) {
	if db == nil {
		return Deps{}, ErrNoDatabase
	}
	if err := check(app, b, s); err != nil {
		return Deps{}, err
	}
	ids, err := s.NewIDs()
	if err != nil {
		return Deps{}, err
	}
	zone, err := time.LoadZone(app.Timezone)
	if err != nil {
		return Deps{}, ErrNoTimezone
	}
	policy, err := backoff.New(time.Second, 60*time.Second)
	if err != nil {
		return Deps{}, err
	}
	q, err := externalcalls.New(db, ids, time.NowNano, policy)
	if err != nil {
		return Deps{}, err
	}
	d := Deps{DB: db, IDs: ids, Events: b.Events, TenantID: app.TenantID, Zone: zone, Countries: app.Countries, Log: app.Log, Queue: NewCallQueue(q)}
	if _, err := buildModules(app, b, &d); err != nil {
		return Deps{}, err
	}
	return d, nil
}

// Mount publishes the built operations on r with the API and the agents the backend chose, then the app's own routes
// and extra mounts (dev tools), which receive the policy.
//
//	err = built.Mount(srv.Router(), devtools.Server(config.Schema)...)
func (out *Built) Mount(r router.Router, mounts ...func(r router.Router, policy model.PolicyDescriber) error) error {
	b := out.backend
	if err := b.API.MountOperations(r, out.ops, b.Public, out.d.Events, out.app.Log); err != nil {
		return err
	}
	if b.Agents != nil {
		if err := b.Agents.MountTools(r, out.app.TenantID, out.Authorize, out.d.Events, out.ops); err != nil {
			return err
		}
	}
	for _, m := range out.api {
		m.MountAPI(r)
	}
	for _, mount := range mounts {
		if err := mount(r, out.Policy); err != nil {
			return err
		}
	}
	if b.Routes != nil {
		b.Routes(r)
	}
	return nil
}

// DenyAll is the default-deny Authorizer used when there is no sign-in.
func DenyAll(logger func(...any)) model.Authorizer {
	return func(userID string, resource model.Resource, action model.Action) bool {
		if logger != nil {
			logger("appkit: DENY (no sign-in configured) — resource:", resource, "action:", action, "userID:", userID)
		}
		return false
	}
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
