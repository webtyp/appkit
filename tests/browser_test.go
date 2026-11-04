//go:build wasm

package tests

import (
	"testing"

	accessui "webtyp.com/access/ui"
	"webtyp.com/appkit"
	"webtyp.com/rpc"
	"webtyp.com/view"
)

func TestApp_BrowserShape(t *testing.T) {
	app := appkit.App{Timezone: "America/Santiago",
		Modules: []appkit.Module{{Name: "x", Browser: appkit.ModuleBrowser{}}},
		Login:   appkit.LoginByRUT{},
	}
	if app.Modules[0].Name != "x" {
		t.Fatalf("Name = %q, want x", app.Modules[0].Name)
	}
}

func TestScreens_LoginByRUTAddsAccess(t *testing.T) {
	caller := view.NewRecords().Caller(rpc.NewCaller(""))
	without, _ := appkit.Screens(appkit.App{Timezone: "America/Santiago"}, caller)
	with, _ := appkit.Screens(appkit.App{Timezone: "America/Santiago", Login: appkit.LoginByRUT{}}, caller)
	if len(without) != 0 {
		t.Fatalf("without login: %d screens, want 0", len(without))
	}
	if len(with) == 0 || with[len(with)-1].ID != accessui.ID {
		t.Fatalf("with LoginByRUT: last screen must be %q, got %v", accessui.ID, with)
	}
}

func TestScreens_GetZone(t *testing.T) {
	var zoneName string
	mod := appkit.Module{
		Name: "zone_reader",
		Browser: appkit.ModuleBrowser{
			Screens: func(d appkit.BrowserDeps) []view.Screen {
				zoneName = d.Zone.Name()
				return nil
			},
		},
	}
	app := appkit.App{Timezone: "America/Santiago", Modules: []appkit.Module{mod}}
	caller := view.NewRecords().Caller(rpc.NewCaller(""))
	_, err := appkit.Screens(app, caller)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if zoneName != "America/Santiago" {
		t.Fatalf("expected zone name America/Santiago, got %q", zoneName)
	}
}
