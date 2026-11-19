package appkit

import (
	"errors"

	"webtyp.com/auth"
	"webtyp.com/country"
	"webtyp.com/model"
)

var (
	ErrNoTimezone          = errors.New("appkit: App.Timezone is empty or not a known time zone — set an IANA name such as UTC")
	ErrNoLog               = errors.New("appkit: App.Log is nil — set it (log.Println) so the server's warnings are seen")
	ErrUnsupportedIndustry = errors.New("appkit: module does not support the application's industry")
)

// App is the whole application, the same value for the server, the Worker and the browser: web/server, web/edge and
// web/browser read it. What differs by platform lives in Backend, Server and Browser.
//
//	var App = appkit.App{Name: "Acme", TenantID: "acme", Timezone: "UTC",
//		SignIn: []auth.Mode{trustedip.Mode{Devices: staffmanager.Definition}},
//		Modules: []model.ModuleDefinition{catalog.Definition, devices.Definition}, Log: log.Println}
type App struct {
	Name      string                   // what the app is called: titles the entry screens and the chassis; the logo is docs/logo/logo.png
	TenantID  string                   // the installation: every module's tenant, the rbac project and the MCP server name
	Timezone  string                   // the installation's zone (IANA name), reaches modules as Deps.Zone / view.BrowserDeps.Zone
	Countries country.Catalog          // the default country and which come first, reaches modules as Deps.Countries / view.BrowserDeps.Countries
	Industry  model.Industry           // the business domain of the app (D1, D4-D5); modules must support it
	Roles     []model.Role             // application roles and their module uses (D3)
	SignIn    []auth.Mode              // the sign-in modes the app offers (auth.Mode values, e.g. passkey.Mode, trustedip.Mode); none = nobody enters and every guarded operation is denied
	Modules   []model.ModuleDefinition // the modules once and in order: build, migration and menu follow it; whoever provides to another goes first
	Log       func(...any)             // required: where the app writes its warnings (log.Println); nil → ErrNoLog, never silence
}
