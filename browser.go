//go:build wasm

package appkit

import (
	"webtyp.com/auth"
	"webtyp.com/components/usermenu"
	. "webtyp.com/dom"
	"webtyp.com/fetch"
	. "webtyp.com/html"
	"webtyp.com/lang"
	appmenu "webtyp.com/layout/app"
	"webtyp.com/layout/platformd"
	"webtyp.com/model"
	"webtyp.com/router"
	"webtyp.com/rpc"
	"webtyp.com/time"
	"webtyp.com/unixid"
	"webtyp.com/view"
)

// Browser asks the server who the user is and then shows the first-admin
// setup, the access screen or the chassis with the screens the user may open.
// It is the whole body of an app's web/browser/main.go; it never returns.
//
//	func main() { appkit.Browser(config.App, devtools.Menu()) }
func Browser(app App, tools appmenu.Tools) {
	s := buildSession(app, rpc.NewCaller(""), tools)
	s.platform.Init(nil)
	Render("app", s)
	select {}
}

// session switches between the three exclusive screens.
type session struct {
	Element
	brand           Brand
	needsSetup      *SignalBool // the server has no administrator yet (auth.PathSetup answers 200)
	showLogin       *SignalBool
	isAuthenticated *SignalBool
	platform        *platformd.Platform
	entryScreens    EntryScreens
}

func (s *session) Render() *Element {
	setupView := Show(s.needsSetup, func() *Element {
		if s.entryScreens.Setup != nil {
			return s.entryScreens.Setup
		}
		return Div()
	})
	loginView := Show(s.showLogin, func() *Element {
		if s.entryScreens.Login != nil {
			return s.entryScreens.Login
		}
		return Div()
	})
	// The chassis is built with the profile already known: platformd filters
	// the screens during its render, and Show renders its content only once.
	platformView := Show(s.isAuthenticated, func() *Element { return s.platform.Render() })
	return Div().Class("spa-root").Child(setupView, loginView, platformView)
}

// profileIdentity reads the profile live: "me" fills it after Platform.User is set.
type profileIdentity struct{ profile *auth.ProfileDTO }

func (i profileIdentity) UserName() string    { return i.profile.Name }
func (i profileIdentity) UserAvatar() string  { return i.profile.Avatar }
func (i profileIdentity) UserRoles() []string { return i.profile.Roles }

func reload() { Navigate("") }

func logout() { fetch.Post(auth.PathLogout).Send(func(*fetch.Response, error) { reload() }) }

func buildSession(app App, caller router.Caller, tools appmenu.Tools) *session {
	ids, err := unixid.NewUnixID()
	if err != nil {
		panic(err) // NewUnixID never fails in the browser
	}

	zone, err := time.LoadZone(app.Timezone)
	if err != nil {
		panic(err)
	}

	var entryScreens EntryScreens
	var hasEntry bool
	for _, m := range app.Modules {
		if m.Browser.Entry != nil {
			if hasEntry {
				panic("appkit: two entry modules — only one module may set ModuleBrowser.Entry")
			}
			entryScreens = m.Browser.Entry(BrowserDeps{Caller: caller, Zone: zone, Brand: app.Brand})
			hasEntry = true
		}
	}

	// Who the user is comes first: blocking here is valid because Browser is
	// called from main, not from a JS callback.
	profile := &auth.ProfileDTO{}
	meDone := make(chan error, 1)
	caller.Call(auth.ModelName+"."+auth.OpMe, &emptyArgs{}, profile, func(err error) { meDone <- err })
	authenticated := <-meDone == nil

	// Without a session only the server knows whether the installation is new;
	// on any doubt the access screen is shown.
	setupRequired := false
	if !authenticated {
		setupDone := make(chan int, 1)
		fetch.Get(auth.PathSetup).Send(func(resp *fetch.Response, err error) {
			if err != nil || resp == nil {
				setupDone <- 0
				return
			}
			setupDone <- resp.Status
		})
		setupRequired = <-setupDone == 200
	}

	isAuthenticated := NewBool(authenticated)
	needsSetup := NewBool(setupRequired)
	showLogin := DeriveBool(func() bool { return !isAuthenticated.Get() && !needsSetup.Get() })

	// One record store per page: what one screen saves, the others see.
	screens, errScreens := Screens(app, view.NewRecords().Caller(caller))
	if errScreens != nil {
		if app.Log != nil {
			app.Log("appkit:", errScreens)
		}
		isAuthenticated.Set(false)
		needsSetup.Set(false)
		screens = nil
	}
	menu, err := appmenu.BuildMenu(
		screens,
		ids,
		func(r model.Resource) bool { return profile.Can(r, model.Read) },
		tools,
	)
	if err != nil {
		panic(err)
	}

	idle := 0
	if authenticated {
		idle = entryScreens.IdleSeconds
		if idle <= 0 {
			idle = 30 * 60
		}
	}
	p := &platformd.Platform{
		AppName: app.Brand.Name,
		Modules: menu.Modules,
		User:    profileIdentity{profile: profile},
		UserActions: func() Component {
			return Button().Text(lang.Translate(textLogOut).String()).
				Class(string(usermenu.NameUserMenu.Class(usermenu.PartAction))).
				OnClick(func(Event) { logout() })
		},
		CanView:     menu.CanView,
		IdleTimeout: idle,
		OnIdle:      logout,
	}

	return &session{
		brand: app.Brand, needsSetup: needsSetup, showLogin: showLogin, isAuthenticated: isAuthenticated,
		platform: p, entryScreens: entryScreens,
	}
}

type emptyArgs struct{}

func (emptyArgs) IsNil() bool                      { return false }
func (emptyArgs) EncodeFields(w model.FieldWriter) {}
func (emptyArgs) DecodeFields(r model.FieldReader) {}
func (emptyArgs) Validate(action byte) error       { return nil }

// Screens are the menu's screens: each module's in App.Modules order (the first one opens first).
//
//	screens := appkit.Screens(config.App, caller)
func Screens(app App, caller router.Caller) ([]view.Screen, error) {
	zone, err := time.LoadZone(app.Timezone)
	if err != nil {
		return nil, ErrNoTimezone
	}
	d := BrowserDeps{Caller: caller, Zone: zone, Brand: app.Brand}

	var out []view.Screen
	for _, m := range app.Modules {
		if m.Browser.Screens != nil {
			out = append(out, m.Browser.Screens(d)...)
		}
	}
	return out, nil
}
