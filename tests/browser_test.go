//go:build wasm

package tests

import (
	"testing"

	"webtyp.com/appkit"
	"webtyp.com/rpc"
	"webtyp.com/view"
)

func TestApp_BrowserShape(t *testing.T) {
	app := appkit.App{Log: func(...any) {}, Timezone: "America/Santiago",
		Modules: []appkit.Module{{Name: "x", Browser: appkit.ModuleBrowser{}}},
	}
	if app.Modules[0].Name != "x" {
		t.Fatalf("Name = %q, want x", app.Modules[0].Name)
	}
}

func TestScreens_GetZone(t *testing.T) {
	var zoneName string
	m := appkit.Module{
		Name: "zone_reader",
		Browser: appkit.ModuleBrowser{
			Screens: func(d appkit.BrowserDeps) []view.Screen {
				zoneName = d.Zone.Name()
				return nil
			},
		},
	}
	app := appkit.App{Log: func(...any) {}, Timezone: "America/Santiago", Modules: []appkit.Module{m}}
	caller := view.NewRecords().Caller(rpc.NewCaller(""))
	_, err := appkit.Screens(app, caller)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if zoneName != "America/Santiago" {
		t.Fatalf("expected zone name America/Santiago, got %q", zoneName)
	}
}
