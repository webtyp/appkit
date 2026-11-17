//go:build !wasm

package appkit

import "webtyp.com/lang"

// Texts of the loading screen, translated in lang.json.
const textLoading lang.Text = "Loading…"

// RenderHTML is what #app shows while the browser binary downloads.
func (a App) RenderHTML() string {
	if a.Name == "" {
		return ""
	}
	return `<div class="login"><div class="login__card"><p class="login__subtitle">` + lang.Translate(textLoading).String() + `</p></div></div>`
}
