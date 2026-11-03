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
	app := appkit.App{
		Modules: []appkit.Module{{Name: "x", Browser: appkit.ModuleBrowser{}}},
		Login:   appkit.LoginByRUT{},
	}
	if app.Modules[0].Name != "x" {
		t.Fatalf("Name = %q, want x", app.Modules[0].Name)
	}
}

func TestScreens_LoginByRUTAddsAccess(t *testing.T) {
	caller := view.NewRecords().Caller(rpc.NewCaller(""))
	without := appkit.Screens(appkit.App{}, caller)
	with := appkit.Screens(appkit.App{Login: appkit.LoginByRUT{}}, caller)
	if len(without) != 0 {
		t.Fatalf("without login: %d screens, want 0", len(without))
	}
	if len(with) == 0 || with[len(with)-1].ID != accessui.ID {
		t.Fatalf("with LoginByRUT: last screen must be %q, got %v", accessui.ID, with)
	}
}
