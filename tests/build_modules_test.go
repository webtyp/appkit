//go:build !wasm

package tests

import (
	"errors"
	"strings"
	"testing"

	"webtyp.com/access"
	"webtyp.com/appkit"
	"webtyp.com/auth/authority"
	"webtyp.com/model"
	"webtyp.com/orm"
	"webtyp.com/storage/mem"
)

func TestBuildModules_NilDB(t *testing.T) {
	_, err := appkit.BuildModules(appkit.App{Timezone: "America/Santiago", TenantID: "t"}, nil)
	if !errors.Is(err, appkit.ErrNoDatabase) {
		t.Fatalf("expected ErrNoDatabase, got %v", err)
	}
}

func TestBuildModules_FindsAppModules(t *testing.T) {
	trustedProvider := mod("trusted", func(d appkit.Deps) (model.ModuleNaming, error) {
		return trustedIPMod{}, nil
	})

	app := appkit.App{Timezone: "America/Santiago",
		TenantID: "t",
		Modules:  []appkit.Module{trustedProvider},
	}
	db := orm.New(mem.New())

	d, err := appkit.BuildModules(app, db)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	if _, err := appkit.Find[trustedIPMod](d); err != nil {
		t.Fatalf("expected to find trustedIPMod, got %v", err)
	}
}

func TestBuildModules_LoginAddsPeople(t *testing.T) {
	trustedProvider := mod("trusted", func(d appkit.Deps) (model.ModuleNaming, error) {
		return trustedIPMod{}, nil
	})

	app := appkit.App{Timezone: "America/Santiago",
		TenantID: "t",
		Login:    appkit.LoginByRUT{},
		Modules:  []appkit.Module{trustedProvider},
	}
	db := orm.New(mem.New())

	d, err := appkit.BuildModules(app, db)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	if _, err := appkit.Find[*authority.Module](d); err != nil {
		t.Fatalf("expected to find *authority.Module, got %v", err)
	}
	if _, err := appkit.Find[*access.Module](d); err != nil {
		t.Fatalf("expected to find *access.Module, got %v", err)
	}
}

func TestBuildModules_ModuleErrorNamed(t *testing.T) {
	badProvider := mod("bad", func(d appkit.Deps) (model.ModuleNaming, error) {
		return nil, errors.New("boom")
	})

	app := appkit.App{Timezone: "America/Santiago",
		TenantID: "t",
		Modules:  []appkit.Module{badProvider},
	}
	db := orm.New(mem.New())

	_, err := appkit.BuildModules(app, db)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if err.Error() != "bad: boom" {
		if !strings.Contains(err.Error(), "bad: boom") {
			t.Fatalf("expected error containing 'bad: boom', got %q", err.Error())
		}
	}
}

func TestBuildModules_BadTimezone(t *testing.T) {
	app := appkit.App{Timezone: "Mars/Base"}
	db := orm.New(mem.New())
	_, err := appkit.BuildModules(app, db)
	if !errors.Is(err, appkit.ErrNoTimezone) {
		t.Fatalf("expected ErrNoTimezone, got %v", err)
	}
}

func TestBuildModules_ZoneInDeps(t *testing.T) {
	var zoneName string
	mod := appkit.Module{
		Name: "zone_reader",
		Server: appkit.ModuleServer{
			Build: func(d appkit.Deps) (model.ModuleNaming, error) {
				zoneName = d.Zone.Name()
				return trustedIPMod{}, nil
			},
		},
	}
	app := appkit.App{Timezone: "America/Santiago", Modules: []appkit.Module{mod}}
	db := orm.New(mem.New())
	_, err := appkit.BuildModules(app, db)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if zoneName != "America/Santiago" {
		t.Fatalf("expected zone name America/Santiago, got %q", zoneName)
	}
}
