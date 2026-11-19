package tests

import (
	"testing"

	"webtyp.com/auth"
	"webtyp.com/notify"
	"webtyp.com/securityevent"
)

// TestBuild_RepeatedFailedSignInsRaiseOneAlert: ten failed sign-ins from one IP raise one notify alert, not ten; a
// serious event (a forged token) raises one at once (SECURITY_OBSERVABILITY D2, D5).
func TestBuild_RepeatedFailedSignInsRaiseOneAlert(t *testing.T) {
	app, b, s := kit(part("a", returns(namedMod{"a"})))
	app.SignIn = []auth.Mode{&fakeMode{}}
	var alerts []notify.Alert
	if _, err := notify.Subscribe(b.Events, func(a notify.Alert) { alerts = append(alerts, a) }, func(error) {}); err != nil {
		t.Fatal(err)
	}
	build(t, app, b, s)
	reporter, err := securityevent.New(b.Events)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 12; i++ {
		_ = reporter.Report(securityevent.Event{Kind: securityevent.LoginFailed, IP: "10.0.0.66", Reason: "unknown RUT"})
	}
	if len(alerts) != 1 || alerts[0].Code != "security.authn_login_fail_max" || alerts[0].Scope != "security" {
		t.Fatalf("alerts = %+v, want one security.authn_login_fail_max", alerts)
	}
	_ = reporter.Report(securityevent.Event{Kind: securityevent.TokenForged, Mode: "cfaccess"})
	if len(alerts) != 2 || alerts[1].Code != "security.authn_token_forged" {
		t.Fatalf("alerts = %+v, want the forged token raised at once", alerts)
	}
}
