//go:build wasm

package browser

import (
	"webtyp.com/appkit"
	"webtyp.com/events"
	"webtyp.com/sse"
)

// connectNotices opens the signed-in user's notices stream (appkit.NoticesPath) and hands it to the screens as
// view.BrowserDeps.Notices. It reconnects on its own after a dropped connection or a server restart.
func connectNotices() events.Subscriber {
	stream := sse.New(&sse.Config{}).Client(&sse.ClientConfig{
		Endpoint:      appkit.NoticesPath,
		RetryInterval: 1000,
		MaxRetryDelay: 30000,
	})
	stream.Connect()
	return sse.Subscriber{Client: stream}
}
