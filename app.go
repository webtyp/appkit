package appkit

import (
	"errors"

	"webtyp.com/view"
)

// ErrNoProvider and ErrManyProviders are what Find returns when the modules
// built so far hold zero or more than one value of the requested type.
var (
	ErrNoProvider       = errors.New("appkit: no module provides it — list the module that does before the one that needs it in App.Modules")
	ErrManyProviders    = errors.New("appkit: more than one module provides it — keep one in App.Modules")
	ErrNoTimezone       = errors.New("appkit: App.Timezone is empty or not a known time zone — set an IANA name such as America/Santiago")
	ErrNoLog            = errors.New("appkit: App.Log is nil — set it (log.Println) so the server's warnings are seen")
	ErrNotCrudModule    = errors.New("appkit: a module builds neither a crud.Module nor Parts — its operations would never be mounted")
	ErrPublicNotMounted = errors.New("appkit: AppServer.Public names an action no module offers")
	ErrNoGrants         = errors.New("appkit: a module declares DefaultGrants but AppServer.Grants is nil; set it to the app's rbac service")
)

// App is the whole application, for both binaries: web/server and web/browser read the same value.
//
//	var App = appkit.App{TenantID: "acme", Brand: Brand, Modules: []appkit.Module{access.Module, catalog.Module}, Server: server}
type App struct {
	TenantID string             // the installation: every module's tenant, the rbac project and the MCP server name
	Timezone string             // the installation's zone (IANA name), reaches modules as Deps.Zone / view.BrowserDeps.Zone
	Brand    Brand              // name and logo on the access screen and the chassis
	Modules  []Module           // build, migration and menu order; the entry module goes first: appkit.Find finds it among the built modules
	Server   AppServer          // what only the server binary needs; the app fills it from a !browser file, zero in the browser
	Log      func(...any)       // required: where the server writes its warnings (log.Println); nil → ErrNoLog, never silence
}

// Module is one domain module as an app lists it in App.Modules, with its server part and its screens.
//
//	var Module = appkit.Module{Name: "service_catalog", Server: server, Browser: browser}
type Module struct {
	Name    string             // identity: migration step name, error prefix
	Server  ModuleServer       // the module fills it from a !browser file; zero in the browser
	Browser view.ModuleBrowser // the module fills it from a browser file; zero on the server
}
