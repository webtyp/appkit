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
	_, err := appkit.BuildModules(appkit.App{TenantID: "t"}, nil)
	if !errors.Is(err, appkit.ErrNoDatabase) {
		t.Fatalf("expected ErrNoDatabase, got %v", err)
	}
}

func TestBuildModules_FindsAppModules(t *testing.T) {
	trustedProvider := mod("trusted", func(d appkit.Deps) (model.ModuleNaming, error) {
		return trustedIPMod{}, nil
	})

	app := appkit.App{
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

	app := appkit.App{
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

	app := appkit.App{
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
