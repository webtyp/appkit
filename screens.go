package appkit

import (
	"errors"

	"webtyp.com/router"
	"webtyp.com/time"
	"webtyp.com/view"
)

var ErrTwoEntryModules = errors.New("appkit: two modules set ModuleBrowser.Entry — only the sign-in module (webtyp/access) may set it; remove Entry from the other one")

// Screens are the menu's screens: each module's in App.Modules order (the first one opens first).
//
//	screens, err := appkit.Screens(config.App, caller)
func Screens(app App, caller router.Caller) ([]view.Screen, error) {
	d, err := browserDeps(app, caller)
	if err != nil {
		return nil, err
	}
	var out []view.Screen
	for _, m := range app.Modules {
		if m.Browser.Screens != nil {
			out = append(out, m.Browser.Screens(d)...)
		}
	}
	return out, nil
}

// Entry is the sign-in and first-setup screens of the app's entry module; zero when no module sets Entry.
//
//	entry, err := appkit.Entry(config.App, caller)
func EntryScreens(app App, caller router.Caller) (view.EntryScreens, error) {
	d, err := browserDeps(app, caller)
	if err != nil {
		return view.EntryScreens{}, err
	}
	var out view.EntryScreens
	found := false
	for _, m := range app.Modules {
		if m.Browser.Entry == nil {
			continue
		}
		if found {
			return view.EntryScreens{}, ErrTwoEntryModules
		}
		out = m.Browser.Entry(d)
		found = true
	}
	return out, nil
}

func browserDeps(app App, caller router.Caller) (view.BrowserDeps, error) {
	zone, err := time.LoadZone(app.Timezone)
	if err != nil {
		return view.BrowserDeps{}, ErrNoTimezone
	}
	return view.BrowserDeps{Caller: caller, Zone: zone}, nil
}
