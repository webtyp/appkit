package appkit

import (
	"errors"
	"fmt"

	"webtyp.com/time"

	"webtyp.com/ddl"
	"webtyp.com/events"
	"webtyp.com/model"
	"webtyp.com/orm"
)

// ErrNoProvider and ErrManyProviders are what Find returns when the modules
// built so far hold zero or more than one value of the requested type.
var (
	ErrNoProvider    = errors.New("appkit: no module provides it — list the module that does before the one that needs it in App.Modules")
	ErrManyProviders = errors.New("appkit: more than one module provides it — keep one in App.Modules")
	ErrNoTimezone    = errors.New("appkit: App.Timezone is empty or not a known time zone — set an IANA name such as America/Santiago")
)

type ModuleServer struct {
	Migrate func(conn ddl.Execer, compiler ddl.Compiler) error
	Build   func(d Deps) (model.ModuleNaming, error)
}

// Deps is what every module gets from appkit: the database, ids, events, the
// installation's tenant and time zone, and Find over the modules built before it.
type Deps struct {
	DB       *orm.DB
	IDs      model.IDGenerator
	Events   events.Publisher
	TenantID string
	Zone     time.Zone // the installation's zone (App.Timezone), loaded once: every local date and hour comes from it

	built []model.ModuleNaming
}

// Find returns the one module built so far that satisfies T, so a module asks
// for what it needs by interface and never imports the module that has it.
//
//	cats, err := appkit.Find[roomlayout.CategorySource](d)
func Find[T any](d Deps) (T, error) {
	var found, zero T
	n := 0
	for _, m := range d.built {
		if v, ok := m.(T); ok {
			found = v
			n++
		}
	}
	switch n {
	case 0:
		return zero, fmt.Errorf("%w: %T", ErrNoProvider, &zero)
	case 1:
		return found, nil
	default:
		return zero, fmt.Errorf("%w: %T", ErrManyProviders, &zero)
	}
}

// FirstAdminRegistrar is what a module offers to complete the first administrator (its device, its staff record).
// Optional: with no module providing it, only the user and its role are created.
type FirstAdminRegistrar interface {
	RegisterFirstAdmin(userID, code, name, ip string) error
}
