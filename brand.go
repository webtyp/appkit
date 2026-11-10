package appkit

import (
	"webtyp.com/lang"
)

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
