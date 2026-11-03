package appkit

import (
	"webtyp.com/auth"
	"webtyp.com/form"
	"webtyp.com/html"
	"webtyp.com/lang"
	"webtyp.com/layout/login"
	"webtyp.com/model"
	"webtyp.com/unixid"
)

// Brand is what an app shows on its access screen and its chassis.
//
//	var Brand = appkit.Brand{Name: "MJosefa CMS", LogoMark: logo}
type Brand struct {
	Name     string
	LogoMark string // data: URL or path of the logo image
}

// LoginWrapperID is the parent id of the access form: the static pre-render and
// the browser must use the same one, because it decides the DOM ids.
const LoginWrapperID = "login-wrapper"

// Texts of the access screens, translated in lang.json.
const (
	textKey         lang.Text = "Key"
	textSignIn      lang.Text = "Sign in"
	textLoading     lang.Text = "Loading…"
	textName        lang.Text = "Name"
	textCreateAdmin lang.Text = "Create administrator"
	textSetupFailed lang.Text = "The administrator could not be created. Check the RUT entered."
	textLoginDenied lang.Text = "Access not authorized on this device"
	textLogOut      lang.Text = "Log out"
	labelRUT                  = "RUT" // the same word in every language: passed as data
)

// LoginForm builds the access form (key by trusted IP). The field is masked and
// does not say what it asks for: the real security is the assigned device; format
// validation is the server's job.
//
//	f, err := appkit.LoginForm(&auth.RUTLoginData{}, ids)
func LoginForm(data *auth.RUTLoginData, ids model.IDGenerator) (*form.Form, error) {
	f, err := form.New(LoginWrapperID, data, ids)
	if err != nil {
		return nil, err
	}
	if inp := f.Input("code"); inp != nil {
		if setter, ok := inp.(interface{ SetSkipValidation(bool) }); ok {
			setter.SetSkipValidation(true)
		}
		if masker, ok := inp.(interface{ SetMasked(bool) }); ok {
			masker.SetMasked(true)
		}
		if titler, ok := inp.(interface{ SetTitle(string) }); ok {
			titler.SetTitle(lang.Translate(textKey).String())
		}
	}
	f.SubmitLabel(lang.Translate(textSignIn).String())
	return f, nil
}

// LoginScreen wraps the access form in the access screen.
//
//	screen := appkit.LoginScreen(config.Brand, f)
func LoginScreen(b Brand, f *form.Form) *login.Login {
	return &login.Login{
		Title:    lang.Text(b.Name),
		Subtitle: "Enter your key to continue",
		Form:     f.Render(),
		LogoMark: b.LogoMark,
	}
}

// LoginPages is the access screen served at "/" before any wasm loads: a plain
// HTML form posting to auth.PathLoginRUT. An app returns it from RenderPages.
//
//	func (p *PreAuthScreen) RenderPages() []html.Page { return appkit.LoginPages(config.Brand) }
func LoginPages(b Brand) []html.Page {
	ids, err := unixid.NewUnixID()
	if err != nil {
		return nil
	}
	f, err := LoginForm(&auth.RUTLoginData{}, ids)
	if err != nil {
		return nil
	}
	f.SetSSR(true).SetAction(auth.PathLoginRUT)
	return []html.Page{{
		Path: "/",
		Doc:  html.DocumentOptions{Title: b.Name},
		Body: LoginScreen(b, f).Render().String(),
	}}
}

// LoadingHTML is what #app shows while the browser binary downloads. An app
// returns it from RenderHTML.
//
//	func (p *PreAuthScreen) RenderHTML() string { return appkit.LoadingHTML() }
func LoadingHTML() string {
	return `<div class="login"><div class="login__card"><p class="login__subtitle">` + lang.Translate(textLoading).String() + `</p></div></div>`
}
