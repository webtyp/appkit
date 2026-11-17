package appkit

import (
	"errors"
	"webtyp.com/country"
	"webtyp.com/model"
)

var (
	ErrNoTimezone          = errors.New("appkit: App.Timezone is empty or not a known time zone — set an IANA name such as America/Santiago")
	ErrUnknownSignIn       = errors.New("appkit: App.SignIn is not a known method — use appkit.SignInByTrustedIP")
	ErrNoLog               = errors.New("appkit: App.Log is nil — set it (log.Println) so the server's warnings are seen")
	ErrUnsupportedIndustry = errors.New("appkit: module does not support the application's industry")
)

// SignInMethod is how people enter the app. Zero means nobody can: every operation is denied.
type SignInMethod byte

// SignInByTrustedIP lets a person in with their key (RUT) from a device the staff register lists as trusted, and
// creates the first administrator the first time the app opens. appkit builds it with auth, rbac and access.
const SignInByTrustedIP SignInMethod = 1

// App is the whole application, the same value for the server, the Worker and the browser: web/server, web/edge and
// web/browser read it. What differs by platform lives in Backend, Server and Browser.
//
//	var App = appkit.App{Name: "Mjosefa", TenantID: "mjosefa-cms", Timezone: "America/Santiago", SignIn: appkit.SignInByTrustedIP,
//		Modules: []model.ModuleDefinition{catalog.Definition, devices.Definition}, Log: log.Println}
type App struct {
	Name      string                   // what the app is called: titles the entry screens and the chassis; the logo is docs/logo/logo.png
	TenantID  string                   // the installation: every module's tenant, the rbac project and the MCP server name
	Timezone  string                   // the installation's zone (IANA name), reaches modules as Deps.Zone / view.BrowserDeps.Zone
	Countries country.Catalog          // the default country and which come first, reaches modules as Deps.Countries / view.BrowserDeps.Countries
	Industry  model.Industry           // the business domain of the app (D1, D4-D5); modules must support it
	Roles     []model.Role             // application roles and their module uses (D3)
	SignIn    SignInMethod             // how people enter; zero = nobody does and every operation is denied
	Modules   []model.ModuleDefinition // the modules once and in order: build, migration and menu follow it; whoever provides to another goes first
	Log       func(...any)             // required: where the app writes its warnings (log.Println); nil → ErrNoLog, never silence
}
