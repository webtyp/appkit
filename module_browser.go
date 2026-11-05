package appkit

import (
	. "webtyp.com/dom"
	"webtyp.com/router"
	"webtyp.com/time"
	"webtyp.com/view"
)

// BrowserDeps is what every module's screens get from appkit in the browser: how to call the server and the
// installation's zone (App.Timezone, loaded once). A new need of the screens becomes a field here, never a new
// parameter.
//
// Example: Screens: func(d appkit.BrowserDeps) []view.Screen { return ui.Screens(d.Caller, d.Zone) }
type BrowserDeps struct {
	Caller router.Caller
	Zone   time.Zone
	Brand  Brand
}

// ModuleBrowser is what a module adds to the browser: its screens. A module fills it
// from its browser file and leaves it zero on the server, so no binary links what it never shows.
type ModuleBrowser struct {
	Screens func(d BrowserDeps) []view.Screen
	Entry   func(d BrowserDeps) EntryScreens // only the entry module sets it
}

// EntryScreens are what the browser shows before the menu: the sign-in and, while the server has no
// administrator, the first-administrator setup. IdleSeconds locks an idle session (0 = 30 minutes).
type EntryScreens struct {
	Login       *Element // webtyp.com/dom Element, rendered as today's loginScreen.Render()
	Setup       *Element
	IdleSeconds int
}
