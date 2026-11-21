//go:build !wasm

package browser

import "webtyp.com/events"

// connectNotices: outside the browser (tests, server-side render) there is no stream to open.
func connectNotices() events.Subscriber { return nil }
