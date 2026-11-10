//go:build !wasm

package tests

import (
	"errors"
	"strings"
	"testing"

	"webtyp.com/appkit"
	"webtyp.com/crud"
	"webtyp.com/model"
	"webtyp.com/orm"
	"webtyp.com/storage/mem"
)

type dummyMod struct{}

func (dummyMod) ModelName() string { return "dummy" }
func (dummyMod) Definition() model.ModuleDefinition { return model.ModuleDefinition{Name: "dummy"} }
func (dummyMod) MountOperations(crud.Registry) {}

type partItem struct{ name string }

func (p partItem) ModelName() string { return p.name }

type partsMod struct{}

func (partsMod) ModelName() string { return "parts_mod" }
func (partsMod) Definition() model.ModuleDefinition { return model.ModuleDefinition{Name: "parts_mod"} }
func (partsMod) MountOperations(crud.Registry) {}
func (partsMod) Parts() []model.ModuleNaming {
	return []model.ModuleNaming{partItem{"p1"}, partItem{"p2"}}
}

func TestBuildModules_NilDB(t *testing.T) {
	_, err := appkit.BuildModules(appkit.App{Log: func(...any) {}, Timezone: "America/Santiago", TenantID: "t"}, nil)
	if !errors.Is(err, appkit.ErrNoDatabase) {
		t.Fatalf("expected ErrNoDatabase, got %v", err)
	}
}

func TestBuildModules_FindsAppModules(t *testing.T) {
	trustedProvider := mod("trusted", func(d appkit.Deps) (model.ModuleNaming, error) {
		return dummyMod{}, nil
	})

	app := appkit.App{Log: func(...any) {}, Timezone: "America/Santiago",
		TenantID: "t",
		Modules:  []appkit.Module{trustedProvider},
	}
	db := orm.New(mem.New())

	d, err := appkit.BuildModules(app, db)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	if _, err := appkit.Find[dummyMod](d); err != nil {
		t.Fatalf("expected to find dummyMod, got %v", err)
	}
}

func TestBuildModules_PartsAddsAllParts(t *testing.T) {
	partsProvider := mod("parts", func(d appkit.Deps) (model.ModuleNaming, error) {
		return partsMod{}, nil
	})

	app := appkit.App{Log: func(...any) {}, Timezone: "America/Santiago",
		TenantID: "t",
		Modules:  []appkit.Module{partsProvider},
	}
	db := orm.New(mem.New())

	d, err := appkit.BuildModules(app, db)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	if _, err := appkit.Find[partItem](d); err == nil {
		// should find multiple parts since there are 2 partItem values
		t.Fatalf("expected ErrManyProviders for partItem, got nil error")
	} else if !errors.Is(err, appkit.ErrManyProviders) {
		t.Fatalf("expected ErrManyProviders, got %v", err)
	}
}

func TestBuildModules_ModuleErrorNamed(t *testing.T) {
	badProvider := mod("bad", func(d appkit.Deps) (model.ModuleNaming, error) {
		return nil, errors.New("boom")
	})

	app := appkit.App{Log: func(...any) {}, Timezone: "America/Santiago",
		TenantID: "t",
		Modules:  []appkit.Module{badProvider},
	}
	db := orm.New(mem.New())

	_, err := appkit.BuildModules(app, db)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "bad: boom") {
		t.Fatalf("expected error containing 'bad: boom', got %q", err.Error())
	}
}

func TestBuildModules_BadTimezone(t *testing.T) {
	app := appkit.App{Log: func(...any) {}, Timezone: "Mars/Base"}
	db := orm.New(mem.New())
	_, err := appkit.BuildModules(app, db)
	if !errors.Is(err, appkit.ErrNoTimezone) {
		t.Fatalf("expected ErrNoTimezone, got %v", err)
	}
}

func TestBuildModules_ZoneInDeps(t *testing.T) {
	var zoneName string
	m := appkit.Module{
		Name: "zone_reader",
		Server: appkit.ModuleServer{
			Build: func(d appkit.Deps) (model.ModuleNaming, error) {
				zoneName = d.Zone.Name()
				return dummyMod{}, nil
			},
		},
	}
	app := appkit.App{Log: func(...any) {}, Timezone: "America/Santiago", Modules: []appkit.Module{m}}
	db := orm.New(mem.New())
	_, err := appkit.BuildModules(app, db)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if zoneName != "America/Santiago" {
		t.Fatalf("expected zone name America/Santiago, got %q", zoneName)
	}
}
