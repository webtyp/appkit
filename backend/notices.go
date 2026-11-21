//go:build !wasm

package backend

import (
	"webtyp.com/appkit"
	"webtyp.com/router"
	"webtyp.com/sse"
)

// userNotices is the native server's notices stream at appkit.NoticesPath: the notifier modules get and the route
// Mount publishes.
func userNotices() (Notifier, router.APIModule) {
	n := sse.NewUserNotices(appkit.NoticesPath)
	return n, n
}
