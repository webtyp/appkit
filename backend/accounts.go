package backend

import (
	"webtyp.com/auth"
	"webtyp.com/fmt"
)

// ErrNoAccountGiver: none of the app's sign-in modes can give an account (auth.GivesSignIn); a person saved in the
// staff screen could never sign in, so saving them is refused with this message instead of silently.
var ErrNoAccountGiver = fmt.Err("appkit: none of App.SignIn's modes gives sign-in accounts (auth.GivesSignIn)")

// lateAccounts is Deps.SignInAccounts: the modules are built before the sign-in (a mode finds the module it needs), so
// the mode that gives accounts is attached once it exists. Modules only call it from operations, after Build.
type lateAccounts struct{ giver auth.GivesSignIn }

func (l *lateAccounts) GiveSignIn(credential, name string) (string, error) {
	if l.giver == nil {
		return "", ErrNoAccountGiver
	}
	return l.giver.GiveSignIn(credential, name)
}
