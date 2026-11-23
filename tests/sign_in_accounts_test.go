package tests

import (
	"errors"
	"testing"

	"webtyp.com/auth"
	"webtyp.com/appkit/backend"
	"webtyp.com/model"
)

// givingMode is a sign-in mode whose authenticator can give accounts (auth.GivesSignIn).
type givingMode struct{ fakeMode }

func (g *givingMode) Build(p auth.Ports) (auth.Authenticator, error) {
	if _, err := g.fakeMode.Build(p); err != nil {
		return nil, err
	}
	return givingAuthenticator{}, nil
}

type givingAuthenticator struct{ fakeAuthenticator }

func (givingAuthenticator) GiveSignIn(credential, name string) (string, error) { return "user-of-" + credential, nil }

// A module built BEFORE the sign-in keeps Deps.SignInAccounts and, once Build finishes, it gives accounts through the
// app's mode. With no mode able to give them it answers ErrNoAccountGiver, never an empty user.
func TestSignInAccounts_ReachTheModeAfterBuild(t *testing.T) {
	var got backend.SignInAccounts
	capture := func(d backend.Deps) (model.ModuleNaming, error) { got = d.SignInAccounts; return namedMod{"staff"}, nil }

	app, b, s := kit(part("staff", capture))
	app.SignIn = []auth.Mode{&givingMode{}}
	build(t, app, b, s)
	if got == nil {
		t.Fatal("a module of an app with sign-in must get Deps.SignInAccounts")
	}
	if user, err := got.GiveSignIn("12345678-5", "Ana"); err != nil || user != "user-of-12345678-5" {
		t.Fatalf("GiveSignIn after Build: %q %v", user, err)
	}

	app, b, s = kit(part("staff", capture))
	app.SignIn = []auth.Mode{&fakeMode{}}
	build(t, app, b, s)
	if _, err := got.GiveSignIn("12345678-5", "Ana"); !errors.Is(err, backend.ErrNoAccountGiver) {
		t.Fatalf("no mode gives accounts: err = %v, want ErrNoAccountGiver", err)
	}
}
