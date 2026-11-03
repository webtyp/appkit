//go:build !wasm

package appkit

import (
	"errors"
	"fmt"

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
)

// Module is one domain module as an app lists it in App.Modules. appkit builds
// it with the shared Deps and migrates it, both in App.Modules order.
//
//	var Module = appkit.Module{Name: "staff_manager", Migrate: migrate.Migrate, Build: build}
type Module struct {
	Name    string                                             // identity: migration step name and error prefix
	Migrate func(conn ddl.Execer, compiler ddl.Compiler) error // nil: the module has no tables
	Build   func(d Deps) (model.ModuleNaming, error)           // nil: migrations only, nothing to serve
}

// Deps is what every module gets from appkit: the database, ids, events, the
// installation's tenant and time zone, and Find over the modules built before it.
type Deps struct {
	DB       *orm.DB
	IDs      model.IDGenerator
	Events   events.Publisher
	Tenant   string
	Timezone string

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
