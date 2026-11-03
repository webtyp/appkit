//go:build browser

package appkit

import (
	"webtyp.com/router"
	"webtyp.com/view"
)

type ModuleBrowser struct {
	Screens func(caller router.Caller) []view.Screen
}

type ModuleServer struct{}
