package appkit

import "webtyp.com/view"

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
