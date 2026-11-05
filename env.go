//go:build !wasm

package appkit

import "webtyp.com/fmt"

// EnvDatabaseURL is the environment variable that holds the database address.
const EnvDatabaseURL = "DATABASE_URL"

var (
	ErrNoOpen     = fmt.Err("appkit: DATABASE_URL is set but App.Server.Database is nil — give it the database driver (postgres.Open)")
	ErrNoDatabase = fmt.Err("appkit: BuildModules got no database — pass the *orm.DB of the transaction the import runs in")
)

func loggerOf(app App) func(...any) {
	if app.Log != nil {
		return app.Log
	}
	return func(...any) {}
}
