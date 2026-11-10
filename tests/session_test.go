package tests

import (
	"errors"
	"testing"

	"webtyp.com/appkit"
	"webtyp.com/router"
	"webtyp.com/view"
)

type recordingLayout struct {
	got view.Session
}

func (l *recordingLayout) Show(s view.Session) {
	l.got = s
}

func TestNewSession_CarriesWhatTheAppIs(t *testing.T) {
	var logged []any
	l := func(args ...any) { logged = append(logged, args...) }

	modScreens := appkit.Module{
		Name: "screens_mod",
		Browser: view.ModuleBrowser{
			Screens: func(d view.BrowserDeps) []view.Screen {
				return []view.Screen{{ID: "s1"}, {ID: "s2"}}
			},
		},
	}
	modEntry := appkit.Module{
		Name: "entry_mod",
		Browser: view.ModuleBrowser{
			Entry: func(d view.BrowserDeps) view.EntryScreens {
				return view.EntryScreens{IdleSeconds: 120}
			},
		},
	}

	app := appkit.App{
		Timezone: "America/Santiago",
		Brand:    view.Brand{Name: "acme", LogoMark: "logo.svg"},
		Log:      l,
		Modules:  []appkit.Module{modScreens, modEntry},
	}

	session, err := appkit.NewSession(app, dummyCaller{})
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}

	if session.Brand.Name != "acme" || session.Brand.LogoMark != "logo.svg" {
		t.Errorf("unexpected Brand: %+v", session.Brand)
	}
	if session.Zone.Name() != "America/Santiago" {
		t.Errorf("unexpected Zone: %v", session.Zone.Name())
	}
	if len(session.Screens) != 2 || session.Screens[0].ID != "s1" || session.Screens[1].ID != "s2" {
		t.Errorf("unexpected Screens: %+v", session.Screens)
	}
	if session.Entry.IdleSeconds != 120 {
		t.Errorf("unexpected Entry: %+v", session.Entry)
	}
	if session.Log == nil {
		t.Fatal("expected session.Log to be set")
	}
	session.Log("test")
	if len(logged) == 0 {
		t.Fatal("session.Log did not call app.Log")
	}
}

func TestNewSession_NoTimezoneIsAnError(t *testing.T) {
	app := appkit.App{}
	_, err := appkit.NewSession(app, dummyCaller{})
	if !errors.Is(err, appkit.ErrNoTimezone) {
		t.Fatalf("expected ErrNoTimezone, got %v", err)
	}
}

func TestNewSession_ScreensShareOnePageOfRecords(t *testing.T) {
	var caller1, caller2 router.Caller
	mod1 := appkit.Module{
		Name: "mod1",
		Browser: view.ModuleBrowser{
			Screens: func(d view.BrowserDeps) []view.Screen {
				caller1 = d.Caller
				return []view.Screen{{ID: "s1"}}
			},
		},
	}
	mod2 := appkit.Module{
		Name: "mod2",
		Browser: view.ModuleBrowser{
			Screens: func(d view.BrowserDeps) []view.Screen {
				caller2 = d.Caller
				return []view.Screen{{ID: "s2"}}
			},
		},
	}
	app := appkit.App{
		Timezone: "America/Santiago",
		Modules:  []appkit.Module{mod1, mod2},
	}

	_, err := appkit.NewSession(app, dummyCaller{})
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}

	if caller1 == nil || caller2 == nil {
		t.Fatalf("expected callers to be captured")
	}
	if caller1 != caller2 {
		t.Fatalf("screens must share the same records caller instance, got %p vs %p", caller1, caller2)
	}
}

func TestNewSession_TwoEntryModulesIsAnError(t *testing.T) {
	mod1 := appkit.Module{
		Name: "entry1",
		Browser: view.ModuleBrowser{
			Entry: func(d view.BrowserDeps) view.EntryScreens {
				return view.EntryScreens{IdleSeconds: 60}
			},
		},
	}
	mod2 := appkit.Module{
		Name: "entry2",
		Browser: view.ModuleBrowser{
			Entry: func(d view.BrowserDeps) view.EntryScreens {
				return view.EntryScreens{IdleSeconds: 30}
			},
		},
	}
	app := appkit.App{
		Timezone: "America/Santiago",
		Modules:  []appkit.Module{mod1, mod2},
	}

	_, err := appkit.NewSession(app, dummyCaller{})
	if !errors.Is(err, appkit.ErrTwoEntryModules) {
		t.Fatalf("expected ErrTwoEntryModules, got %v", err)
	}
}

func TestLayout_ShowReceivesTheSession(t *testing.T) {
	app := appkit.App{
		Timezone: "America/Santiago",
		Brand:    view.Brand{Name: "my-app"},
	}
	session, err := appkit.NewSession(app, dummyCaller{})
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}

	layout := &recordingLayout{}
	var l view.Layout = layout
	l.Show(session)

	if layout.got.Brand.Name != "my-app" {
		t.Fatalf("expected Brand.Name 'my-app', got %q", layout.got.Brand.Name)
	}
}

func TestApp_BrandIsViewBrand(t *testing.T) {
	var _ view.Brand = appkit.App{}.Brand
}
