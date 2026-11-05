package appkit

import (
	"webtyp.com/lang"
)

// Brand is what an app shows on its access screen and its chassis.
//
//	var Brand = appkit.Brand{Name: "MJosefa CMS", LogoMark: logo}
type Brand struct {
	Name     string
	LogoMark string // data: URL or path of the logo image
}

// Texts of the access screens, translated in lang.json.
const (
	textLoading lang.Text = "Loading…"
	textLogOut  lang.Text = "Log out"
)

// loadingHTML is what #app shows while the browser binary downloads. An app
// returns it from RenderHTML.
//
//	func (p *PreAuthScreen) RenderHTML() string { return appkit.loadingHTML() }
func loadingHTML() string {
	return `<div class="login"><div class="login__card"><p class="login__subtitle">` + lang.Translate(textLoading).String() + `</p></div></div>`
}
