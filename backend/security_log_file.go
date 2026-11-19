//go:build !wasm

package backend

import (
	"webtyp.com/events"
	"webtyp.com/notify"
)

func attachCriticalFileLog(broker events.Broker, logger func(...any)) error {
	fl := notify.NewCriticalFileLog(notify.DefaultCriticalLogFile)
	_, err := notify.Subscribe(broker, func(a notify.Alert) {
		if werr := fl.WriteAlert(a); werr != nil && logger != nil {
			logger(werr)
		}
	}, func(err error) {
		if logger != nil {
			logger(err)
		}
	})
	return err
}
