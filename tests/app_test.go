//go:build !wasm

package tests

import (
	"errors"
	"strings"
	"testing"

	"webtyp.com/appkit"
	"webtyp.com/ddl"
	"webtyp.com/model"
	"webtyp.com/storage"
	"webtyp.com/storage/mem"
)

type providerMod struct{}                       // provides categories
func (providerMod) ModelName() string           { return "provider" }
func (providerMod) Categories() []string        { return []string{"a"} }

type CategorySource interface{ Categories() []string }

type consumerMod struct{ src CategorySource }
func (consumerMod) ModelName() string { return "consumer" }

var calls []string // order of Build/Migrate calls, reset per test
func mod(name string, build func(appkit.Deps) (model.ModuleNaming, error)) appkit.Module {
	return appkit.Module{
		Name:    name,
		Server: appkit.ModuleServer{
			Migrate: func(ddl.Execer, ddl.Compiler) error { calls = append(calls, "migrate:"+name); return nil },
			Build:   build,
		},
	}
}
func openMem(string) (storage.Conn, error) { return mem.New(), nil }

type provider2Mod struct{}
func (provider2Mod) ModelName() string           { return "provider2" }
func (provider2Mod) Categories() []string        { return []string{"b"} }

func TestFind_OneProvider(t *testing.T) {
	calls = nil
	var capturedCat string

	provider := mod("provider", func(d appkit.Deps) (model.ModuleNaming, error) {
		return providerMod{}, nil
	})
	consumer := mod("consumer", func(d appkit.Deps) (model.ModuleNaming, error) {
		src, err := appkit.Find[CategorySource](d)
		if err == nil {
			capturedCat = src.Categories()[0]
		}
		return consumerMod{src}, err
	})

	app := appkit.App{
		Server: appkit.AppServer{Database: openMem},
		Modules: []appkit.Module{provider, consumer},
	}

	_, err := appkit.BuildServer(app, appkit.ServerEnv{DSN: "mem"})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if capturedCat != "a" {
		t.Fatalf("expected capturedCat to be 'a', got %q", capturedCat)
	}
}

func TestFind_NoProvider(t *testing.T) {
	calls = nil
	consumer := mod("consumer", func(d appkit.Deps) (model.ModuleNaming, error) {
		src, err := appkit.Find[CategorySource](d)
		return consumerMod{src}, err
	})

	app := appkit.App{
		Server: appkit.AppServer{Database: openMem},
		Modules: []appkit.Module{consumer},
	}

	_, err := appkit.BuildServer(app, appkit.ServerEnv{DSN: "mem"})
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !errors.Is(err, appkit.ErrNoProvider) {
		t.Fatalf("expected ErrNoProvider, got %v", err)
	}
	if !strings.HasPrefix(err.Error(), "consumer: ") {
		t.Fatalf("expected error to start with 'consumer: ', got %q", err.Error())
	}
}

func TestFind_ProviderAfterConsumer(t *testing.T) {
	calls = nil
	provider := mod("provider", func(d appkit.Deps) (model.ModuleNaming, error) {
		return providerMod{}, nil
	})
	consumer := mod("consumer", func(d appkit.Deps) (model.ModuleNaming, error) {
		src, err := appkit.Find[CategorySource](d)
		return consumerMod{src}, err
	})

	app := appkit.App{
		Server: appkit.AppServer{Database: openMem},
		Modules: []appkit.Module{consumer, provider},
	}

	_, err := appkit.BuildServer(app, appkit.ServerEnv{DSN: "mem"})
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !errors.Is(err, appkit.ErrNoProvider) {
		t.Fatalf("expected ErrNoProvider, got %v", err)
	}
}

func TestFind_TwoProviders(t *testing.T) {
	calls = nil
	provider := mod("provider", func(d appkit.Deps) (model.ModuleNaming, error) {
		return providerMod{}, nil
	})
	provider2 := mod("provider2", func(d appkit.Deps) (model.ModuleNaming, error) {
		return provider2Mod{}, nil
	})
	consumer := mod("consumer", func(d appkit.Deps) (model.ModuleNaming, error) {
		src, err := appkit.Find[CategorySource](d)
		return consumerMod{src}, err
	})

	app := appkit.App{
		Server: appkit.AppServer{Database: openMem},
		Modules: []appkit.Module{provider, provider2, consumer},
	}

	_, err := appkit.BuildServer(app, appkit.ServerEnv{DSN: "mem"})
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !errors.Is(err, appkit.ErrManyProviders) {
		t.Fatalf("expected ErrManyProviders, got %v", err)
	}
}

func TestBuildServer_NoDSN(t *testing.T) {
	calls = nil
	provider := appkit.Module{
		Name: "provider",
		Server: appkit.ModuleServer{
			Build: func(d appkit.Deps) (model.ModuleNaming, error) {
				calls = append(calls, "build:provider")
				return providerMod{}, nil
			},
		},
	}

	app := appkit.App{
		Server: appkit.AppServer{Database: openMem},
		Modules: []appkit.Module{provider},
	}

	_, err := appkit.BuildServer(app, appkit.ServerEnv{})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if len(calls) != 0 {
		t.Fatalf("expected calls to be empty, got %v", calls)
	}
}

func TestBuildServer_DSNWithoutOpen(t *testing.T) {
	calls = nil
	app := appkit.App{}

	_, err := appkit.BuildServer(app, appkit.ServerEnv{DSN: "x"})
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !errors.Is(err, appkit.ErrNoOpen) {
		t.Fatalf("expected ErrNoOpen, got %v", err)
	}
}

func TestSteps_Order(t *testing.T) {
	calls = nil
	a := mod("a", nil)
	b := mod("b", nil)
	c := appkit.Module{Name: "c"} // no Migrate

	app := appkit.App{
		Modules: []appkit.Module{a, b, c},
	}

	steps := appkit.Steps(app)
	if len(steps) != 2 {
		t.Fatalf("expected 2 steps, got %d", len(steps))
	}
	if steps[0].Name != "a" || steps[1].Name != "b" {
		t.Fatalf("expected steps 'a' and 'b', got %s and %s", steps[0].Name, steps[1].Name)
	}
}

func TestRunMigrations_Order(t *testing.T) {
	calls = nil
	a := mod("a", nil)
	b := mod("b", nil)

	app := appkit.App{
		Modules: []appkit.Module{a, b},
	}

	err := appkit.RunMigrations(app, mem.New(), nil)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if len(calls) != 2 || calls[0] != "migrate:a" || calls[1] != "migrate:b" {
		t.Fatalf("expected calls ['migrate:a', 'migrate:b'], got %v", calls)
	}
}

func TestRunMigrations_StopsAtError(t *testing.T) {
	calls = nil
	a := appkit.Module{
		Name: "a",
		Server: appkit.ModuleServer{
			Migrate: func(ddl.Execer, ddl.Compiler) error {
				calls = append(calls, "migrate:a")
				return errors.New("boom")
			},
		},
	}
	b := mod("b", nil)

	app := appkit.App{
		Modules: []appkit.Module{a, b},
	}

	err := appkit.RunMigrations(app, mem.New(), nil)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "migrate a: boom") {
		t.Fatalf("expected error containing 'migrate a: boom', got %q", err.Error())
	}
	for _, c := range calls {
		if c == "migrate:b" {
			t.Fatalf("expected no 'migrate:b' call")
		}
	}
}

func TestMigrate_NoDatabaseURL(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	app := appkit.App{}
	err := appkit.Migrate(app)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !errors.Is(err, appkit.ErrNoDatabaseURL) {
		t.Fatalf("expected ErrNoDatabaseURL, got %v", err)
	}
}

func TestMigrate_NoOpen(t *testing.T) {
	t.Setenv("DATABASE_URL", "x")
	app := appkit.App{}
	err := appkit.Migrate(app)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !errors.Is(err, appkit.ErrNoOpen) {
		t.Fatalf("expected ErrNoOpen, got %v", err)
	}
}
