//go:build browser

package tests

import (
	"testing"

	"webtyp.com/appkit"
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
