//go:build browser

package appkit

import (
	accessui "webtyp.com/access/ui"
	"webtyp.com/auth"
	. "webtyp.com/dom"
	"webtyp.com/fetch"
	"webtyp.com/fmt"
	"webtyp.com/form"
	"webtyp.com/json"
	"webtyp.com/lang"
	"webtyp.com/model"
	"webtyp.com/view"
)

const sessionIdleSeconds = 30 * 60

const (
	textKey           lang.Text = "Key"
	textSignIn        lang.Text = "Sign in"
	textName          lang.Text = "Name"
	textCreateAdmin   lang.Text = "Create administrator"
	textSetupFailed   lang.Text = "The administrator could not be created. Check the RUT entered."
	textLoginDenied   lang.Text = "Access not authorized on this device"
	textLoginSubtitle lang.Text = "Enter your key to continue"
	textSetupSubtitle lang.Text = "First setup: create the administrator"
	labelRUT                    = "RUT"
	loginWrapperID              = "login-wrapper"
	setupWrapperID              = "setup-wrapper"
)

// signInMenu is the administration of users and roles the sign-in brings to the menu; none without sign-in.
func signInMenu(app App, d view.BrowserDeps) []view.Screen {
	if app.SignIn == 0 {
		return nil
	}
	return accessui.Screens(d.Caller)
}

// signInScreens are the sign-in and first-setup screens of the app's sign-in method; zero when the app has none.
func signInScreens(app App, d view.BrowserDeps) (view.EntryScreens, error) {
	if app.SignIn == 0 {
		return view.EntryScreens{}, nil
	}
	if app.SignIn != SignInByTrustedIP {
		return view.EntryScreens{}, ErrUnknownSignIn
	}
	loginMessage := NewString("")
	loginData := &auth.RUTLoginData{}
	loginF, err := loginForm(loginData, d.IDs)
	if err != nil {
		return view.EntryScreens{}, err
	}
	loginF.OnSubmit(func(fd model.Fielder, done func(error)) {
		post(auth.PathLoginRUT, loginData, done, func() {
			msg := lang.Translate(textLoginDenied).String()
			loginMessage.Set(msg)
			done(fmt.Err(msg))
		})
	})

	setupMessage := NewString("")
	setupData := &auth.SetupData{}
	setupF, err := form.New(setupWrapperID, setupData, d.IDs)
	if err != nil {
		return view.EntryScreens{}, err
	}
	title(setupF.Input("code"), labelRUT, true)
	title(setupF.Input("name"), lang.Translate(textName).String(), false)
	setupF.SubmitLabel(lang.Translate(textCreateAdmin).String())
	setupF.OnSubmit(func(fd model.Fielder, done func(error)) {
		post(auth.PathSetup, setupData, done, func() {
			msg := lang.Translate(textSetupFailed).String()
			setupMessage.Set(msg)
			done(fmt.Err(msg))
		})
	})
	return view.EntryScreens{
		Login:       view.Entry{Subtitle: textLoginSubtitle, Form: loginF.Render(), Message: loginMessage},
		Setup:       view.Entry{Subtitle: textSetupSubtitle, Form: setupF.Render(), Message: setupMessage},
		IdleSeconds: sessionIdleSeconds,
	}, nil
}

// title labels an input and, when skipValidation, lets any text through (the server checks the RUT).
func title(inp any, label string, skipValidation bool) {
	if t, ok := inp.(interface{ SetTitle(string) }); ok {
		t.SetTitle(label)
	}
	if s, ok := inp.(interface{ SetSkipValidation(bool) }); ok && skipValidation {
		s.SetSkipValidation(true)
	}
}

func loginForm(data *auth.RUTLoginData, ids model.IDGenerator) (*form.Form, error) {
	f, err := form.New(loginWrapperID, data, ids)
	if err != nil {
		return nil, err
	}
	if inp := f.Input("code"); inp != nil {
		title(inp, lang.Translate(textKey).String(), true)
		if m, ok := inp.(interface{ SetMasked(bool) }); ok {
			m.SetMasked(true)
		}
	}
	f.SubmitLabel(lang.Translate(textSignIn).String())
	return f, nil
}

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
		Navigate("")
		done(nil)
	})
}
