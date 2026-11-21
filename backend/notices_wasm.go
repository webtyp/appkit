//go:build wasm

package backend

import "webtyp.com/router"

// userNotices: a Worker answers one request and holds no stream, so it has no notices (Deps.Notices stays nil).
func userNotices() (Notifier, router.APIModule) { return nil, nil }
