package tests

import (
	"errors"
	"testing"

	"webtyp.com/appkit"
	"webtyp.com/model"
	"webtyp.com/view"
)

type dummyCaller struct{}

func (dummyCaller) Call(router.Operation, model.Encodable, model.Decodable, func(error)) {}
func (dummyCaller) Dispatch(router.Operation, model.Encodable)                             {}

func TestScreens_GetZone(t *testing.T) {
	var gotZone string
	mod := appkit.Module{
		Name: "test_mod",
		Browser: view.ModuleBrowser{
			Screens: func(d view.BrowserDeps) []view.Screen {
				gotZone = d.Zone.Name()
				return []view.Screen{{ID: "s1"}}
			},
		},
	}
	app := appkit.App{
		Timezone: "America/Santiago",
		Modules:  []appkit.Module{mod},
	}
	screens, err := appkit.Screens(app, dummyCaller{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(screens) != 1 || screens[0].ID != "s1" {
		t.Fatalf("unexpected screens: %+v", screens)
	}
	if gotZone != "America/Santiago" {
		t.Fatalf("expected America/Santiago, got %q", gotZone)
	}
}

func TestScreens_BadTimezone(t *testing.T) {
	app := appkit.App{
		Timezone: "Mars/Base",
		Modules:  []appkit.Module{},
	}
	_, err := appkit.Screens(app, dummyCaller{})
	if !errors.Is(err, appkit.ErrNoTimezone) {
		t.Fatalf("expected ErrNoTimezone, got %v", err)
	}
}

func TestEntryScreens_NoneIsZero(t *testing.T) {
	app := appkit.App{
		Timezone: "America/Santiago",
		Modules: []appkit.Module{
			{Name: "mod1"},
		},
	}
	entry, err := appkit.EntryScreens(app, dummyCaller{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if entry.IdleSeconds != 0 || entry.Login.Subtitle != "" {
		t.Fatalf("expected zero EntryScreens, got %+v", entry)
	}
}

func TestEntryScreens_OneModule(t *testing.T) {
	app := appkit.App{
		Timezone: "America/Santiago",
		Modules: []appkit.Module{
			{
				Name: "auth_mod",
				Browser: view.ModuleBrowser{
					Entry: func(d view.BrowserDeps) view.EntryScreens {
						return view.EntryScreens{IdleSeconds: 60}
					},
				},
			},
		},
	}
	entry, err := appkit.EntryScreens(app, dummyCaller{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if entry.IdleSeconds != 60 {
		t.Fatalf("expected IdleSeconds == 60, got %d", entry.IdleSeconds)
	}
}

func TestEntryScreens_TwoModulesIsError(t *testing.T) {
	app := appkit.App{
		Timezone: "America/Santiago",
		Modules: []appkit.Module{
			{
				Name: "auth1",
				Browser: view.ModuleBrowser{
					Entry: func(d view.BrowserDeps) view.EntryScreens {
						return view.EntryScreens{IdleSeconds: 60}
					},
				},
			},
			{
				Name: "auth2",
				Browser: view.ModuleBrowser{
					Entry: func(d view.BrowserDeps) view.EntryScreens {
						return view.EntryScreens{IdleSeconds: 30}
					},
				},
			},
		},
	}
	_, err := appkit.EntryScreens(app, dummyCaller{})
	if !errors.Is(err, appkit.ErrTwoEntryModules) {
		t.Fatalf("expected ErrTwoEntryModules, got %v", err)
	}
}
