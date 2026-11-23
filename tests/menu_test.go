package tests

import (
	"errors"
	"strings"
	"testing"

	"webtyp.com/appkit/browser"
)

func menuIDs(t *testing.T, menu []browser.MenuGroup) (string, error) {
	t.Helper()
	app, b := browserKit(browser.Module{Name: "a", Screens: screensOf("a1", "a2")}, browser.Module{Name: "b", Screens: screensOf("b1")})
	b.Menu = menu
	s, err := browser.NewSession(app, b)
	if err != nil {
		return "", err
	}
	var out []string
	for _, g := range s.Screens {
		out = append(out, g.ID+":")
		for _, c := range g.Children {
			out = append(out, c.ID)
		}
	}
	return strings.Join(out, " "), nil
}

func TestMenu_OrderIsTheOrderWritten(t *testing.T) {
	got, err := menuIDs(t, []browser.MenuGroup{
		{ID: "g1", Screens: []string{"b1", "a2"}},
		{ID: "g2", Screens: []string{"a1"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got != "g1: b1 a2 g2: a1" {
		t.Fatalf("menu must follow the written order, got %q", got)
	}
}

func TestMenu_WrongMenuStopsTheApp(t *testing.T) {
	cases := []struct {
		name string
		menu []browser.MenuGroup
		want error
	}{
		{"unknown screen", []browser.MenuGroup{{ID: "g", Screens: []string{"a1", "a2", "b1", "zz"}}}, browser.ErrMenuUnknownScreen},
		{"repeated screen", []browser.MenuGroup{{ID: "g", Screens: []string{"a1", "a2", "b1", "a1"}}}, browser.ErrMenuRepeatedScreen},
		{"hidden screen", []browser.MenuGroup{{ID: "g", Screens: []string{"a1", "b1"}}}, browser.ErrMenuScreenMissing},
		{"empty group", []browser.MenuGroup{{ID: "g"}}, browser.ErrMenuEmptyGroup},
	}
	for _, c := range cases {
		if _, err := menuIDs(t, c.menu); !errors.Is(err, c.want) {
			t.Errorf("%s: want %v, got %v", c.name, c.want, err)
		}
	}
}
