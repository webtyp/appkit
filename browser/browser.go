package browser

import (
	"errors"

	"webtyp.com/appkit"
	"webtyp.com/fmt"
	"webtyp.com/model"
	"webtyp.com/router"
	"webtyp.com/time"
	"webtyp.com/view"
)

var (
	ErrNoCaller        = errors.New("appkit: Browser.Caller is nil — give it the connection to the server (rpc.NewCaller(\"\"))")
	ErrNoBrowserIDs    = errors.New("appkit: Browser.NewIDs is nil — give it the id generator (unixid.NewUnixID)")
	ErrNoBrowserPart   = errors.New("appkit: a module of App.Modules has no part in Browser.Modules — add one with the module's Name (Screens may be nil)")
	ErrUnlistedScreens = errors.New("appkit: Browser.Modules names a module that is not in App.Modules — list it there")
)

// Browser is what runs in the browser: how it talks to the server, how it makes ids and each module's screens.
//
//	var Browser = browser.Browser{Caller: rpc.NewCaller(""), NewIDs: newIDs, Modules: []browser.Module{{Name: catalog.Definition.Name, Screens: catalogui.Screens}}}
type Browser struct {
	Caller  router.Caller                     // how the browser talks to the server
	NewIDs  func() (model.IDGenerator, error) // how ids are created in the browser
	Modules []Module                          // the screens of each module of App.Modules, found by Name
}

// Module is the screens of one module. Name is the module's Definition.Name; Screens nil = a module with no screen.
type Module struct {
	Name    string
	Screens func(d view.BrowserDeps) []view.Screen
}

// Run is the whole body of an app's web/browser/main.go: it builds the session and gives it to the layout the
// app chose; it never returns.
//
//	func main() { browser.Run(config.App, screens.Browser, layoutapp.New(devtools.Menu())) }
func Run(app appkit.App, b Browser, layout view.Layout) {
	session, err := NewSession(app, b)
	if err != nil {
		app.Log("appkit:", err) // App.Log is required (ErrNoLog), so this never panics
		return
	}
	layout.Show(session)
	select {}
}

// NewSession builds what a layout draws: the app's name, the connection to the server, the zone, the menu screens and
// the sign-in screens.
//
//	session, err := browser.NewSession(config.App, screens.Browser)
func NewSession(app appkit.App, b Browser) (view.Session, error) {
	d, err := browserDeps(app, b)
	if err != nil {
		return view.Session{}, err
	}
	records := view.NewRecords().Caller(b.Caller) // one page of records: what one saves the others see
	d.Caller = records
	screens := Screens(app, b, d)
	entry, err := signInScreens(app, d)
	if err != nil {
		return view.Session{}, err
	}
	return view.Session{Name: app.Name, Caller: b.Caller, Zone: d.Zone, Screens: screens, Entry: entry, Log: app.Log}, nil
}

// Screens are the menu's screens: the module administration of the sign-in first when the app has one, then each
// module's in App.Modules order (the first one opens first).
func Screens(app appkit.App, b Browser, d view.BrowserDeps) []view.Screen {
	out := signInMenu(app, d)
	for _, def := range app.Modules {
		for _, m := range b.Modules {
			if m.Name == def.Name && m.Screens != nil {
				out = append(out, m.Screens(d)...)
			}
		}
	}
	return out
}

func browserDeps(app appkit.App, b Browser) (view.BrowserDeps, error) {
	if app.Log == nil {
		return view.BrowserDeps{}, appkit.ErrNoLog
	}
	if b.Caller == nil {
		return view.BrowserDeps{}, ErrNoCaller
	}
	if b.NewIDs == nil {
		return view.BrowserDeps{}, ErrNoBrowserIDs
	}
	for _, def := range app.Modules {
		found := false
		for _, m := range b.Modules {
			if m.Name == def.Name {
				found = true
			}
		}
		if !found {
			return view.BrowserDeps{}, fmt.ErrType(fmt.Err(def.Name), ErrNoBrowserPart)
		}
	}
	for _, m := range b.Modules {
		listed := false
		for _, def := range app.Modules {
			if def.Name == m.Name {
				listed = true
			}
		}
		if !listed {
			return view.BrowserDeps{}, fmt.ErrType(fmt.Err(m.Name), ErrUnlistedScreens)
		}
	}
	ids, err := b.NewIDs()
	if err != nil {
		return view.BrowserDeps{}, err
	}
	zone, err := time.LoadZone(app.Timezone)
	if err != nil {
		return view.BrowserDeps{}, appkit.ErrNoTimezone
	}
	return view.BrowserDeps{Caller: b.Caller, Zone: zone, Countries: app.Countries, IDs: ids}, nil
}
