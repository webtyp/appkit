//go:build !wasm

package appkit

import (
	"errors"
	"fmt"

	"webtyp.com/ddl"
	"webtyp.com/env"
)

// EnvDatabaseURL is the environment variable that holds the database address.
const EnvDatabaseURL = "DATABASE_URL"

var (
	ErrNoDatabaseURL = errors.New("appkit: DATABASE_URL is not set")
	ErrNoOpen        = errors.New("appkit: DATABASE_URL is set but App.Open is nil — give App the database driver (postgres.Open)")
	ErrNoCompiler    = errors.New("appkit: the database connection does not implement ddl.Compiler")
)

// Steps are the migrations of App.Modules, in that order; a module without
// Migrate has none.
func Steps(app App) []ddl.Step {
	var out []ddl.Step
	for _, m := range app.Modules {
		if m.Migrate != nil {
			out = append(out, ddl.Step{Name: m.Name, Run: m.Migrate})
		}
	}
	return out
}

// RunMigrations runs Steps(app) against conn, stopping at the first error.
func RunMigrations(app App, conn ddl.Execer, compiler ddl.Compiler) error {
	logger := loggerOf(app)
	for _, step := range Steps(app) {
		logger("migrate:", step.Name)
		if err := step.Run(conn, compiler); err != nil {
			return fmt.Errorf("migrate %s: %w", step.Name, err)
		}
	}
	return nil
}

// Migrate opens DATABASE_URL with App.Open and runs every module's migration.
// It is the whole body of an app's migrate/main.go.
func Migrate(app App) error {
	dsn := env.Get(EnvDatabaseURL)
	if dsn == "" {
		return ErrNoDatabaseURL
	}
	if app.Open == nil {
		return ErrNoOpen
	}
	conn, err := app.Open(dsn)
	if err != nil {
		return err
	}
	compiler, ok := conn.(ddl.Compiler)
	if !ok {
		return ErrNoCompiler
	}
	return RunMigrations(app, conn, compiler)
}

func loggerOf(app App) func(...any) {
	if app.Logger != nil {
		return app.Logger
	}
	return func(...any) {}
}
