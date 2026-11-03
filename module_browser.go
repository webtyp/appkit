package appkit

import (
	"webtyp.com/router"
	"webtyp.com/view"
)

// ModuleBrowser is what a module adds to the browser: its screens. A module fills it
// from its browser file and leaves it zero on the server, so no binary links what it never shows.
type ModuleBrowser struct {
	Screens func(caller router.Caller) []view.Screen
}
