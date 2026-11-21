package backend

import (
	"webtyp.com/country"
	"webtyp.com/crud"
	"webtyp.com/events"
	"webtyp.com/fmt"
	"webtyp.com/model"
	"webtyp.com/orm"
	"webtyp.com/router"
	"webtyp.com/storage"
	"webtyp.com/time"
)

var (
	ErrNoEvents       = fmt.Err("appkit: Backend.Events is nil — give it a broker (inprocess.Broker{})")
	ErrNoAPI          = fmt.Err("appkit: Backend.API is nil — give it the transport that publishes the operations (rpc.JSON{})")
	ErrNoIDs          = fmt.Err("appkit: Server.NewIDs is nil — give it the id generator (unixid.NewUnixID)")
	ErrNoOpen         = fmt.Err("appkit: a database address is set but Server.Database is nil — give it the database driver (postgres.Open)")
	ErrNoDatabase     = fmt.Err("appkit: BuildModules got no database — pass the *orm.DB of the transaction the import runs in")
	ErrNoPart         = fmt.Err("appkit: a module of App.Modules has no part in the list that must hold it — add one with the module's Name")
	ErrUnlistedPart   = fmt.Err("appkit: a part names a module that is not in App.Modules — list it there, in build order")
	ErrNotCrudModule  = fmt.Err("appkit: a module builds neither a crud.Module nor Parts — its operations would never be mounted")
	ErrNoModuleBuilt  = fmt.Err("appkit: no module with that name is built — list it before the one that needs it in App.Modules")
	ErrNoBuildFunc    = fmt.Err("appkit: a BackendModule has no Build — set how the module is constructed")
	ErrNoGrantsTarget = fmt.Err("appkit: a module declares DefaultGrants but the database has no rbac to hold them")
)

// Backend is what runs in the backend, the same in the native server and in a Worker: how each module is built, the
// transports and the broker. The database and the ids depend on the platform and live in Server.
//
//	var Backend = backend.Backend{Events: &inprocess.Broker{}, API: rpc.JSON{}, Agents: mcp.Tools{Allowed: MCPTools()}, Modules: parts}
type Backend struct {
	Events  events.Broker         // the notices between modules, in the same process (inprocess.Broker{})
	API     API                   // publishes the operations (rpc.JSON{})
	Agents  Agents                // lets an agent call some of them (mcp.Tools{Allowed: …}); nil = no agent endpoint
	Public  []crud.Public         // actions visitors may call without a session; nil = none
	Routes  func(r router.Router) // the app's own routes; nil = none
	Modules []Module              // how each module of App.Modules is built, found by Name
}

// Module is how one module is constructed on the backend. Name is the module's Definition.Name.
//
//	backend.Module{Name: devicemanager.Definition.Name, Build: func(d backend.Deps) (model.ModuleNaming, error) { return devicemanager.New(d.DB, devicemanager.Deps{IDs: d.IDs}) }}
type Module struct {
	Name          string
	Build         func(d Deps) (model.ModuleNaming, error)
	DefaultGrants []model.Grant // what any signed-in person may do with this module (granted to rbac.RoleAuthenticated)
}

// Server is what only the native server decides, because it depends on the platform: a Worker has its own (config/edge.go).
//
//	var Server = backend.Server{Database: postgres.Open, NewIDs: func() (model.IDGenerator, error) { return unixid.NewUnixID() }}
type Server struct {
	Database func(dsn string) (storage.Conn, error) // the driver (postgres.Open); nil = no database
	NewIDs   func() (model.IDGenerator, error)      // how ids are created

	SessionIdleMinutes int // minutes without activity before a session ends; 0 = 30
}

// API publishes the operations of the modules on a router. rpc.JSON{} is the one the apps use.
type API interface {
	MountOperations(r router.Router, ops []crud.Module, public []crud.Public, pub events.Publisher, log func(...any)) error
}

// Agents publishes the operations an agent may call. mcp.Tools{Allowed: …} is the one the apps use.
type Agents interface {
	MountTools(r router.Router, name string, authorize model.Authorizer, pub events.Publisher, ops []crud.Module) error
}

// Parts lets one module build several values (people, roles, administration): appkit adds each part to the built
// list, so FindBuiltModule, operations and API routes see them as if they were modules. Optional, asserted at the seam.
type Parts interface {
	Parts() []model.ModuleNaming
}

// Notifier sends a notice with no data to every browser where a user is signed in (sse.UserNotices). The screen
// that listens reads what changed through its caller. A module declares its own one-method interface and the app
// passes Deps.Notices to it.
//
//	d.Notices.Notify(userID, clinicalencounter.AgendaChanged)
type Notifier interface {
	Notify(userID, topic string)
}

// Deps is what every module gets from appkit: the database, ids, events, the installation's tenant and time zone,
// and FindBuiltModule over the modules built before it.
type Deps struct {
	DB         *orm.DB
	IDs        model.IDGenerator
	Events     events.Publisher  // publishes the notices between modules (Backend.Events)
	Subscriber events.Subscriber // listens to the notices between modules: the same broker as Events
	Notices    Notifier          // notices to the browsers of a signed-in user; nil when the app has no sign-in
	TenantID   string
	Zone       time.Zone       // the installation's zone (App.Timezone), loaded once: every local date and hour comes from it
	Countries  country.Catalog // the installation's countries (App.Countries): the default country and which come first
	Log        func(...any)    // the app's log (App.Log); never nil
	Queue      crud.CallQueue

	built *[]builtModule
}

// builtModule is a built module and the name it is found by: its Module.Name, or a part's ModelName().
type builtModule struct {
	name string
	v    model.ModuleNaming
}

// FindBuiltModule returns the module already built with that name (Definition.Name), so the app can hand one
// module what another provides without either importing the other. The caller asserts the type in one line.
//
//	m, err := backend.FindBuiltModule(d, devicemanager.Definition.Name)
//	dm, ok := m.(*devicemanager.Module)
func FindBuiltModule(d Deps, name string) (model.ModuleNaming, error) {
	if d.built != nil {
		for _, m := range *d.built {
			if m.name == name {
				return m.v, nil
			}
		}
	}
	return nil, fmt.ErrType(fmt.Err(name), ErrNoModuleBuilt)
}
