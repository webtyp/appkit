package appkit

// App is the whole application, for both binaries: web/server and web/browser read the same value.
//
//	var App = appkit.App{TenantID: "acme", Brand: Brand, Modules: []appkit.Module{catalog.Module}, Login: appkit.LoginByRUT{}, Server: server}
type App struct {
	TenantID string       // the installation: every module's tenant, the rbac project and the MCP server name
	Timezone string       // e.g. "America/Santiago"
	Brand    Brand        // name and logo on the access screen and the chassis
	Modules  []Module     // build, migration and menu order; the first screen is the one opened first
	Login    Login        // how people enter; nil = nobody: every operation is denied
	Server   AppServer    // what only the server binary needs; the app fills it from a !browser file, zero in the browser
	Log      func(...any) // nil = discard
}

// Module is one domain module as an app lists it in App.Modules, with its server part and its screens.
//
//	var Module = appkit.Module{Name: "service_catalog", Server: server, Browser: browser}
type Module struct {
	Name    string        // identity: migration step name, error prefix
	Server  ModuleServer  // the module fills it from a !browser file; zero in the browser
	Browser ModuleBrowser // the module fills it from a browser file; zero on the server
}

// Login is how people enter the app. Only appkit implements it: today LoginByRUT.
type Login interface{ loginMethod() }

// LoginByRUT is entry by RUT from a device the installation knows (trusted IP), with first-administrator setup.
//
//	Login: appkit.LoginByRUT{}
type LoginByRUT struct {
	IdleMinutes int // session idle lock; 0 = SESSION_IDLE_MINUTES or 30
}

func (LoginByRUT) loginMethod() {}

// defaultIdleMinutes is the session idle lock when LoginByRUT.IdleMinutes is 0 (and, on the server, SESSION_IDLE_MINUTES is unset).
const defaultIdleMinutes = 30
