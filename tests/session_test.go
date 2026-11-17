package tests

import (
	"errors"
	"strings"
	"testing"

	"webtyp.com/appkit"
	"webtyp.com/appkit/browser"
	"webtyp.com/model"
	"webtyp.com/router"
	"webtyp.com/unixid"
	"webtyp.com/view"
)

type noCaller struct{}

func (noCaller) Call(router.Operation, model.Encodable, model.Decodable, func(error)) {}
func (noCaller) Dispatch(router.Operation, model.Encodable)                           {}

type recordingLayout struct{ got view.Session }

func (l *recordingLayout) Show(s view.Session) { l.got = s }

func browserKit(mods ...browser.Module) (appkit.App, browser.Browser) {
	app := appkit.App{Name: "acme", Timezone: "America/Santiago", Log: func(...any) {}}
	for _, m := range mods {
		app.Modules = append(app.Modules, model.ModuleDefinition{Name: m.Name})
	}
	return app, browser.Browser{
		Caller:  noCaller{},
		NewIDs:  func() (model.IDGenerator, error) { return unixid.NewUnixID() },
		Modules: mods,
	}
}

func screensOf(ids ...string) func(view.BrowserDeps) []view.Screen {
	return func(view.BrowserDeps) []view.Screen {
		var out []view.Screen
		for _, id := range ids {
			out = append(out, view.Screen{ID: id})
		}
		return out
	}
}

func TestNewSession_CarriesWhatTheAppIs(t *testing.T) {
	app, b := browserKit(browser.Module{Name: "a", Screens: screensOf("a1")}, browser.Module{Name: "b", Screens: screensOf("b1", "b2")})
	s, err := browser.NewSession(app, b)
	if err != nil {
		t.Fatal(err)
	}
	if s.Name != "acme" || s.Zone.Name() != "America/Santiago" || s.Log == nil || s.Caller == nil {
		t.Fatalf("session incomplete: %+v", s)
	}
	var ids []string
	for _, sc := range s.Screens {
		ids = append(ids, sc.ID)
	}
	if strings.Join(ids, ",") != "a1,b1,b2" {
		t.Fatalf("screens must follow App.Modules order, got %v", ids)
	}
}

func TestNewSession_ScreensGetZoneIdsAndCountries(t *testing.T) {
	var got view.BrowserDeps
	app, b := browserKit(browser.Module{Name: "a", Screens: func(d view.BrowserDeps) []view.Screen { got = d; return nil }})
	if _, err := browser.NewSession(app, b); err != nil {
		t.Fatal(err)
	}
	if got.Zone.Name() != "America/Santiago" || got.IDs == nil || got.Caller == nil {
		t.Fatalf("BrowserDeps incomplete: %+v", got)
	}
}

func TestNewSession_ScreensShareOnePageOfRecords(t *testing.T) {
	var callers []router.Caller
	grab := func(d view.BrowserDeps) []view.Screen { callers = append(callers, d.Caller); return nil }
	app, b := browserKit(browser.Module{Name: "a", Screens: grab}, browser.Module{Name: "b", Screens: grab})
	if _, err := browser.NewSession(app, b); err != nil {
		t.Fatal(err)
	}
	if len(callers) != 2 || callers[0] != callers[1] {
		t.Fatalf("every module must get the same records caller, got %v", callers)
	}
}

func TestNewSession_WhatTheAppForgotIsNamed(t *testing.T) {
	cases := []struct {
		name   string
		change func(*appkit.App, *browser.Browser)
		want   error
	}{
		{"no log", func(a *appkit.App, _ *browser.Browser) { a.Log = nil }, appkit.ErrNoLog},
		{"no caller", func(_ *appkit.App, b *browser.Browser) { b.Caller = nil }, browser.ErrNoCaller},
		{"no ids", func(_ *appkit.App, b *browser.Browser) { b.NewIDs = nil }, browser.ErrNoBrowserIDs},
		{"bad timezone", func(a *appkit.App, _ *browser.Browser) { a.Timezone = "Mars/Base" }, appkit.ErrNoTimezone},
		{"module without screens part", func(a *appkit.App, _ *browser.Browser) {
			a.Modules = append(a.Modules, model.ModuleDefinition{Name: "orphan"})
		}, browser.ErrNoBrowserPart},
		{"screens of an unlisted module", func(_ *appkit.App, b *browser.Browser) {
			b.Modules = append(b.Modules, browser.Module{Name: "stray"})
		}, browser.ErrUnlistedScreens},
		{"unknown sign-in", func(a *appkit.App, _ *browser.Browser) { a.SignIn = 99 }, appkit.ErrUnknownSignIn},
	}
	for _, c := range cases {
		app, b := browserKit(browser.Module{Name: "a"})
		c.change(&app, &b)
		if _, err := browser.NewSession(app, b); !errors.Is(err, c.want) {
			t.Errorf("%s: want %v, got %v", c.name, c.want, err)
		}
	}
}

func TestNewSession_WithoutSignInThereAreNoEntryScreens(t *testing.T) {
	app, b := browserKit(browser.Module{Name: "a", Screens: screensOf("a1")})
	s, err := browser.NewSession(app, b)
	if err != nil {
		t.Fatal(err)
	}
	if s.Entry.Login.Form != nil || s.Entry.Setup.Form != nil || len(s.Screens) != 1 {
		t.Fatalf("no sign-in: no entry screens and no administration screens, got %+v", s)
	}
}

func TestNewSession_SignInByTrustedIPBringsTheEntryAndTheAdministration(t *testing.T) {
	app, b := browserKit(browser.Module{Name: "a", Screens: screensOf("a1")})
	app.SignIn = appkit.SignInByTrustedIP
	s, err := browser.NewSession(app, b)
	if err != nil {
		t.Fatal(err)
	}
	if s.Entry.Login.Form == nil || s.Entry.Setup.Form == nil || s.Entry.IdleSeconds <= 0 {
		t.Fatalf("sign-in must give the login and the first-setup screens: %+v", s.Entry)
	}
	if len(s.Screens) < 2 || s.Screens[0].ID != "access" || s.Screens[len(s.Screens)-1].ID != "a1" {
		t.Fatalf("the administration of users and roles goes first, the app's screens after: %+v", s.Screens)
	}
}
