package appkit

import (
	"webtyp.com/router"
	"webtyp.com/time"
	"webtyp.com/view"
)

// NewSession builds what a layout draws: the app's brand, the connection to the server, the zone, the menu screens and the entry screens.
//
//	session, err := appkit.NewSession(config.App, rpc.NewCaller(""))
func NewSession(app App, caller router.Caller) (view.Session, error) {
	records := view.NewRecords().Caller(caller) // one page of records: what one saves the others see
	screens, err := Screens(app, records)
	if err != nil {
		return view.Session{}, err
	}
	entry, err := EntryScreens(app, records)
	if err != nil {
		return view.Session{}, err
	}
	zone, err := time.LoadZone(app.Timezone)
	if err != nil {
		return view.Session{}, ErrNoTimezone
	}
	return view.Session{Brand: app.Brand, Caller: caller, Zone: zone, Screens: screens, Entry: entry, Log: app.Log}, nil
}
