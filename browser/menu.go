package browser

import (
	"errors"

	"webtyp.com/fmt"
	"webtyp.com/lang"
	"webtyp.com/view"
)

var (
	ErrMenuUnknownScreen  = errors.New("appkit: Browser.Menu lists a screen that no module offers — use the ID the module exports")
	ErrMenuRepeatedScreen = errors.New("appkit: Browser.Menu lists the same screen twice — each screen goes in one group")
	ErrMenuScreenMissing  = errors.New("appkit: a screen is in no Browser.Menu group — list it in one, or the menu would hide it")
	ErrMenuEmptyGroup     = errors.New("appkit: a Browser.Menu group has no screens — remove it or list its screens")
)

// MenuGroup is one folder of the menu: its name and the screens inside, top to bottom.
// To move a screen, move its line; the order written is the order shown.
//
//	browser.MenuGroup{ID: "menu.operation", Label: "Operation", Help: "Daily work of the clinic.",
//		Screens: []string{roomsui.OccupancyID, bookingui.ID}}
type MenuGroup struct {
	ID      string    // menu route and identity, unique in the app
	Label   lang.Text // menu text
	Help    lang.Text // what the group is for
	Screens []string  // IDs of the screens in the group, in display order
}

// groupMenu puts every screen of the app into the groups of menu, in the order written. It fails loudly when a
// group lists a screen nobody offers, lists one twice, is empty, or when a screen is left in no group.
func groupMenu(screens []view.Screen, menu []MenuGroup) ([]view.Screen, error) {
	leaves := leafScreens(screens)
	placed := make([]bool, len(leaves)) // placed[i]: leaves[i] is already in a group
	var out []view.Screen
	for _, g := range menu {
		if len(g.Screens) == 0 {
			return nil, fmt.ErrType(fmt.Err(g.ID), ErrMenuEmptyGroup)
		}
		children := make([]view.Screen, 0, len(g.Screens))
		for _, id := range g.Screens {
			i := indexOfScreen(leaves, id)
			switch {
			case i < 0:
				return nil, fmt.ErrType(fmt.Err(id), ErrMenuUnknownScreen)
			case placed[i]:
				return nil, fmt.ErrType(fmt.Err(id), ErrMenuRepeatedScreen)
			}
			placed[i] = true
			children = append(children, leaves[i])
		}
		out = append(out, view.Screen{ID: g.ID, Label: g.Label, Icon: children[0].Icon, Help: g.Help, Children: children})
	}
	for i, s := range leaves {
		if !placed[i] {
			return nil, fmt.ErrType(fmt.Err(s.ID), ErrMenuScreenMissing)
		}
	}
	return out, nil
}

// leafScreens are the screens that open something, however deep a module nested them.
func leafScreens(screens []view.Screen) []view.Screen {
	var leaves []view.Screen
	for _, s := range screens {
		if len(s.Children) > 0 {
			leaves = append(leaves, leafScreens(s.Children)...)
			continue
		}
		leaves = append(leaves, s)
	}
	return leaves
}

// indexOfScreen is the position of the screen with this ID, or -1.
func indexOfScreen(screens []view.Screen, id string) int {
	for i, s := range screens {
		if s.ID == id {
			return i
		}
	}
	return -1
}
