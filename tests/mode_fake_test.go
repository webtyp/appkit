package tests

import (
	"webtyp.com/auth"
	"webtyp.com/auth/authority"
	"webtyp.com/model"
	"webtyp.com/router"
	"webtyp.com/view"
)

// pathFakeLogin is the route the fake mode mounts.
const pathFakeLogin = "/fake-login"

// fakeMode is a sign-in mode at the edge: it records the pieces appkit built it with and gives one entry screen.
type fakeMode struct {
	ports    *auth.Ports
	buildErr error
	closed   bool // its screen is not open at this door
}

func (f *fakeMode) Build(p auth.Ports) (auth.Authenticator, error) {
	if f.buildErr != nil {
		return nil, f.buildErr
	}
	f.ports = &p
	return fakeAuthenticator{}, nil
}

func (f *fakeMode) EntryScreen(view.BrowserDeps) (view.Entry, bool, error) {
	return view.Entry{Subtitle: "fake"}, !f.closed, nil
}

type fakeAuthenticator struct{}

func (fakeAuthenticator) Name() string { return "fake" }
func (fakeAuthenticator) Mount(r router.Router) {
	r.Post(pathFakeLogin, func(router.Context) {}).Public()
}

// meCaller answers who is signed in: me when set, otherwise err (nil err and nil me = nobody).
type meCaller struct {
	me  *authority.Me
	err error
}

type unreachableErr struct{}

func (unreachableErr) Error() string         { return "no answer" }
func (unreachableErr) Kind() model.ErrorKind { return model.ErrorUnreachable }

type deniedErr struct{}

func (deniedErr) Error() string { return "denied" }

func (c meCaller) Call(op router.Operation, args model.Encodable, into model.Decodable, done func(error)) {
	if c.me != nil {
		if list, ok := into.(*authority.MeList); ok {
			*list = authority.MeList{c.me}
		}
		done(nil)
		return
	}
	if c.err != nil {
		done(c.err)
		return
	}
	done(deniedErr{})
}
func (meCaller) Dispatch(router.Operation, model.Encodable) {}
