package appkit

import (
	"errors"

	"webtyp.com/events"
	"webtyp.com/model"
	"webtyp.com/orm"
	"webtyp.com/router"
	"webtyp.com/time"
)

// ErrNoProvider and ErrManyProviders are what Find returns when the modules
// built so far hold zero or more than one value of the requested type.
var (
	ErrNoProvider    = errors.New("appkit: no module provides it — list the module that does before the one that needs it in App.Modules")
	ErrManyProviders = errors.New("appkit: more than one module provides it — keep one in App.Modules")
	ErrNoTimezone    = errors.New("appkit: App.Timezone is empty or not a known time zone — set an IANA name such as America/Santiago")
	ErrNoLog         = errors.New("appkit: App.Log is nil — set it (log.Println) so the server's warnings are seen")
)

type ModuleServer struct {
	Build func(d Deps) (model.ModuleNaming, error)
}

// Entry is how people enter the app; one module of App.Modules provides it. Without it every operation is denied.
// Example: entry, err := appkit.Find[appkit.Entry](d)
type Entry interface {
	Authn() router.Middleware      // who the caller is (session)
	Authorize() model.Authorizer   // what the caller may do
	Policy() model.PolicyDescriber // which role opens each resource (API explorer)
}

// Parts lets one module build several values (people, roles, administration): appkit adds each part to the built
// list, so Find, operations and API routes see them as if they were modules. Optional, asserted at the seam.
type Parts interface {
	Parts() []model.ModuleNaming
}

// Deps is what every module gets from appkit: the database, ids, events, the
// installation's tenant and time zone, and Find over the modules built before it.
type Deps struct {
	DB       *orm.DB
	IDs      model.IDGenerator
	Events   events.Publisher
	TenantID string
	Zone     time.Zone    // the installation's zone (App.Timezone), loaded once: every local date and hour comes from it
	Log      func(...any) // the app's log (App.Log); never nil: a no-op when the app declares none

	built *[]model.ModuleNaming
}

// Find returns the one module built so far that satisfies T, so a module asks
// for what it needs by interface and never imports the module that has it.
//
//	cats, err := appkit.Find[roomlayout.CategorySource](d)
func Find[T any](d Deps) (T, error) {
	var found, zero T
	n := 0
	if d.built != nil {
		for _, m := range *d.built {
			if v, ok := m.(T); ok {
				found = v
				n++
			}
		}
	}
	switch n {
	case 0:
		return zero, ErrNoProvider
	case 1:
		return found, nil
	default:
		return zero, ErrManyProviders
	}
}
