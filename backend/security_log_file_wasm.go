//go:build wasm

package backend

import "webtyp.com/events"

func attachCriticalFileLog(_ events.Broker, _ func(...any)) error {
	return nil
}
