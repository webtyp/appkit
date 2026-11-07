//go:build !wasm

package appkit

import (
	eventsmock "webtyp.com/events/mock" // same import path serve.go uses for eventsmock.Broker
	"webtyp.com/orm"
	"webtyp.com/time"
	"webtyp.com/unixid"
)

// BuildModules builds the app's modules, in App.Modules order, all on db. An import passes its transaction,
// so every write rolls back with it. Events go to a broker nobody listens to: importing history is not a business action.
//
// Example:
//
//	plan := dataimport.Plan[appkit.Deps]{
//		Build: func(tx *orm.DB) (appkit.Deps, error) { return appkit.BuildModules(config.App, tx) },
//	}
//	// in a step: users, err := appkit.Find[*authority.Module](s.Modules)
func BuildModules(app App, db *orm.DB) (Deps, error) {
	if db == nil {
		return Deps{}, ErrNoDatabase
	}
	if app.Log == nil {
		return Deps{}, ErrNoLog
	}
	ids, err := unixid.NewUnixID()
	if err != nil {
		return Deps{}, err
	}
	zone, err := time.LoadZone(app.Timezone)
	if err != nil {
		return Deps{}, ErrNoTimezone
	}
	d := Deps{DB: db, IDs: ids, Events: &eventsmock.Broker{}, TenantID: app.TenantID, Zone: zone, Log: app.Log}
	if _, err := buildModules(app, &d); err != nil {
		return Deps{}, err
	}
	return d, nil
}
