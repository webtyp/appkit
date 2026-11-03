//go:build wasm

package appkit

import (
	"syscall/js"

	accessui "webtyp.com/access/ui"
	"webtyp.com/auth"
	"webtyp.com/components/usermenu"
	. "webtyp.com/dom"
	"webtyp.com/fetch"
	"webtyp.com/fmt"
	"webtyp.com/form"
	. "webtyp.com/html"
	"webtyp.com/json"
	"webtyp.com/lang"
	"webtyp.com/layout/apiexplorer"
	appmenu "webtyp.com/layout/app"
	"webtyp.com/layout/login"
	"webtyp.com/layout/platformd"
	"webtyp.com/layout/schemaview"
	"webtyp.com/model"
	"webtyp.com/router"
	"webtyp.com/rpc"
	"webtyp.com/unixid"
	"webtyp.com/view"
)

// RunBrowser asks the server who the user is and then shows the first-admin
// setup, the access screen or the chassis with the screens the user may open.
// It is the whole body of an app's web/browser/main.go; it never returns.
//
//	func main() { appkit.RunBrowser(config.App) }
func RunBrowser(app App) {
	s := buildSession(app, rpc.NewCaller(""))
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
	loginForm       *form.Form
	setupForm       *form.Form
	loginMessage    *SignalString
	setupMessage    *SignalString
}

func (s *session) Render() *Element {
	setupView := Show(s.needsSetup, func() *Element {
		return (&login.Login{
			Title:    lang.Text(s.brand.Name),
			Subtitle: "First setup: create the administrator",
			Form:     s.setupForm.Render(),
			LogoMark: s.brand.LogoMark,
			Message:  s.setupMessage,
		}).Render()
	})
	loginScreen := LoginScreen(s.brand, s.loginForm)
	loginScreen.Message = s.loginMessage
	loginView := Show(s.showLogin, func() *Element { return loginScreen.Render() })
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

func reload() { js.Global().Get("location").Call("reload") }

func logout() { fetch.Post(auth.PathLogout).Send(func(*fetch.Response, error) { reload() }) }

// idleSeconds is the client idle lock of the app's login: LoginByRUT.IdleMinutes, else 30 minutes.
func idleSeconds(l Login) int {
	if r, ok := l.(LoginByRUT); ok && r.IdleMinutes > 0 {
		return r.IdleMinutes * 60
	}
	return defaultIdleMinutes * 60
}

func buildSession(app App, caller router.Caller) *session {
	ids, err := unixid.NewUnixID()
	if err != nil {
		panic(err) // NewUnixID never fails in the browser
	}

	// Who the user is comes first: blocking here is valid because RunBrowser is
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
	menu, err := appmenu.BuildMenu(
		Screens(app, view.NewRecords().Caller(caller)),
		ids,
		func(r model.Resource) bool { return profile.Can(r, model.Read) },
		appmenu.Tools{API: apiexplorer.FromServer(), Database: schemaview.FromServer()},
	)
	if err != nil {
		panic(err)
	}

	idle := 0
	if authenticated {
		idle = idleSeconds(app.Login)
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

	loginMessage := NewString("")
	setupMessage := NewString("")

	rutData := &auth.RUTLoginData{}
	loginForm, err := LoginForm(rutData, ids)
	if err == nil {
		loginForm.OnSubmit(func(fd model.Fielder, done func(error)) {
			post(auth.PathLoginRUT, rutData, done, func() {
				msg := lang.Translate(textLoginDenied).String()
				loginMessage.Set(msg)
				done(fmt.Err(msg))
			})
		})
	}

	// The setup field is not masked: the person declares their own RUT and must
	// be able to read it back before confirming.
	setupData := &auth.SetupData{}
	setupForm, errSetup := form.New("setup-wrapper", setupData, ids)
	if errSetup == nil {
		if inp := setupForm.Input("code"); inp != nil {
			if titler, ok := inp.(interface{ SetTitle(string) }); ok {
				titler.SetTitle(labelRUT)
			}
			// The server validates the RUT; the client charset rejects "-".
			if setter, ok := inp.(interface{ SetSkipValidation(bool) }); ok {
				setter.SetSkipValidation(true)
			}
		}
		if inp := setupForm.Input("name"); inp != nil {
			if titler, ok := inp.(interface{ SetTitle(string) }); ok {
				titler.SetTitle(lang.Translate(textName).String())
			}
		}
		setupForm.SubmitLabel(lang.Translate(textCreateAdmin).String())
		setupForm.OnSubmit(func(fd model.Fielder, done func(error)) {
			post(auth.PathSetup, setupData, done, func() {
				msg := lang.Translate(textSetupFailed).String()
				setupMessage.Set(msg)
				done(fmt.Err(msg))
			})
		})
	}

	return &session{
		brand: app.Brand, needsSetup: needsSetup, showLogin: showLogin, isAuthenticated: isAuthenticated,
		platform: p, loginForm: loginForm, setupForm: setupForm,
		loginMessage: loginMessage, setupMessage: setupMessage,
	}
}

// post sends data as JSON to path: 200 reloads the page, any other status calls refused.
func post(path string, data model.Encodable, done func(error), refused func()) {
	var out []byte
	if err := json.Encode(data, &out); err != nil {
		done(err)
		return
	}
	fetch.Post(path).ContentTypeJSON().Body(out).Send(func(resp *fetch.Response, err error) {
		if err != nil {
			done(err)
			return
		}
		if resp.Status != 200 {
			refused()
			return
		}
		reload()
		done(nil)
	})
}

type emptyArgs struct{}

func (emptyArgs) IsNil() bool                      { return false }
func (emptyArgs) EncodeFields(w model.FieldWriter) {}
func (emptyArgs) DecodeFields(r model.FieldReader) {}
func (emptyArgs) Validate(action byte) error       { return nil }

// Screens are the menu's screens: each module's in App.Modules order (the first one opens
// first), then the login's own (people and roles, from access, for LoginByRUT).
//
//	screens := appkit.Screens(config.App, caller)
func Screens(app App, caller router.Caller) []view.Screen {
	var out []view.Screen
	for _, m := range app.Modules {
		if m.Browser.Screens != nil {
			out = append(out, m.Browser.Screens(caller)...)
		}
	}
	if _, ok := app.Login.(LoginByRUT); ok {
		out = append(out, accessui.Screens(caller)...)
	}
	return out
}
