package backend

import (
	"sync"

	"webtyp.com/events"
	"webtyp.com/fmt"
	"webtyp.com/msgtype"
	"webtyp.com/notify"
	"webtyp.com/securityevent"
	"webtyp.com/time"
)

// What the app watches in its security events (SECURITY_OBSERVABILITY D2, D5): every event is written to the log;
// repeated failed sign-ins from one IP and the serious kinds raise a notify alert, delivered by whoever the app has
// listening on notify.TopicAlert. The blocking itself is the shared limiter (auth.Ports.Limiter), in the request.
const (
	maxLoginAttempts   = 3       // sign-in attempts per IP before the shared limiter blocks it
	loginWindow        = 15 * 60 // seconds the attempts are counted
	loginBlock         = 15 * 60 // seconds an IP stays blocked
	alertFailures      = 10      // failed sign-ins from one IP within alertWindow that raise an alert
	alertWindow        = 15 * 60 // seconds
	alertScopeSecurity = "security"
	alertCodePrefix    = "security."
	failMaxName        = "authn_login_fail_max"
)

// securityWatch counts failed sign-ins per IP (a slice, not a map: the edge build is WASM) and raises the alerts.
type securityWatch struct {
	mu       sync.Mutex
	failures []ipFailures
	alerts   events.Publisher
	log      func(...any)
}

type ipFailures struct {
	ip      string
	since   time.Unix
	count   int
	alerted bool
}

// watchSecurity listens to every security event on broker: it logs them and raises the alerts.
func watchSecurity(broker events.Broker, logger func(...any)) error {
	w := &securityWatch{alerts: broker, log: logger}
	if _, err := securityevent.Listen(broker, w.handle); err != nil {
		return err
	}
	return attachCriticalFileLog(broker, logger)
}

func (w *securityWatch) handle(e securityevent.Event) {
	if e.Kind == securityevent.LoginSucceeded && e.Mode == modeDevAutologin {
		return
	}
	w.log(logLine(e)...)
	switch e.Kind {
	case securityevent.LoginFailed:
		if w.countFailure(e.IP, e.At) {
			w.raise(failMaxName, msgtype.Error, fmt.Err(msgManyFailures).Error(), e)
		}
	case securityevent.TokenForged, securityevent.TokenReused, securityevent.ContentBlocked,
		securityevent.SecurityStoreFailed, securityevent.AdminCreated:
		w.raise(e.Kind.Name(), e.Kind.Level(), fmt.Err(msgSecurityEvent).Error(), e)
	}
}

// countFailure records one failed sign-in from ip at t and reports whether it just reached alertFailures.
func (w *securityWatch) countFailure(ip string, t time.Unix) bool {
	if ip == "" {
		return false
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	kept := w.failures[:0]
	var rec *ipFailures
	for i := range w.failures {
		f := w.failures[i]
		if t-f.since > alertWindow {
			continue // the window closed: forget it
		}
		kept = append(kept, f)
		if f.ip == ip {
			rec = &kept[len(kept)-1]
		}
	}
	w.failures = kept
	if rec == nil {
		w.failures = append(w.failures, ipFailures{ip: ip, since: t})
		rec = &w.failures[len(w.failures)-1]
	}
	rec.count++
	if rec.count >= alertFailures && !rec.alerted {
		rec.alerted = true
		return true
	}
	return false
}

func (w *securityWatch) raise(code string, level msgtype.Type, subject string, e securityevent.Event) {
	body := e.Kind.Name()
	for _, part := range []string{e.Mode, e.IP, e.UserID, string(e.Reason)} {
		if part != "" {
			body += " · " + part
		}
	}
	a := notify.Alert{Code: alertCodePrefix + code, Scope: alertScopeSecurity, Level: level, Subject: subject, Body: body}
	if err := notify.Raise(w.alerts, a); err != nil {
		w.log(err)
	}
}

// logLine is the event as one log line with its fields.
func logLine(e securityevent.Event) []any {
	args := []any{"app: SECURITY EVENT —", e.Kind.Level().String(), e.Kind.Name()}
	for _, f := range []struct{ label, value string }{
		{"mode:", e.Mode},
		{"userID:", e.UserID},
		{"resource:", e.Resource},
		{"IP:", e.IP},
		{"reason:", string(e.Reason)},
	} {
		if f.value != "" {
			args = append(args, f.label, f.value)
		}
	}
	return args
}

const (
	msgManyFailures  = "many failed sign-ins from one address: someone may be trying keys"
	msgSecurityEvent = "a serious security event happened"
)
