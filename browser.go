//go:build browser

package appkit

import (
	"webtyp.com/rpc"
	"webtyp.com/view"
)

// Browser is the whole body of an app's web/browser/main.go: it builds the session and gives it to the layout the app chose; it never returns.
//
//	func main() { appkit.Browser(config.App, layoutapp.New(devtools.Menu())) }
func Browser(app App, layout view.Layout) {
	session, err := NewSession(app, rpc.NewCaller(""))
	if err != nil {
		app.Log("appkit:", err) // App.Log is required (ErrNoLog), so this never panics
		return
	}
	layout.Show(session)
	select {}
}
