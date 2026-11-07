//go:build !wasm

package tests

import (
	"errors"
	"strings"
	"testing"

	"webtyp.com/appkit"
	"webtyp.com/crud"
	"webtyp.com/ddl"
	"webtyp.com/model"
	"webtyp.com/orm"
	"webtyp.com/storage"
	"webtyp.com/storage/mem"
)

type providerMod struct{}                // provides categories
func (providerMod) ModelName() string    { return "provider" }
func (providerMod) Categories() []string { return []string{"a"} }
func (providerMod) MountOperations(crud.Registry) {}

type CategorySource interface{ Categories() []string }

type consumerMod struct{ src CategorySource }

func (consumerMod) ModelName() string { return "consumer" }
func (consumerMod) MountOperations(crud.Registry) {}

var calls []string // order of Build/Migrate calls, reset per test
func mod(name string, build func(appkit.Deps) (model.ModuleNaming, error)) appkit.Module {
	return appkit.Module{
		Name: name,
		Server: appkit.ModuleServer{
			Build: build,
		},
	}
}
func openMem(string) (storage.Conn, error) { return mem.New(), nil }

type provider2Mod struct{}

func (provider2Mod) ModelName() string    { return "provider2" }
func (provider2Mod) Categories() []string { return []string{"b"} }
func (provider2Mod) MountOperations(crud.Registry) {}

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

	app := appkit.App{Log: func(...any) {}, Timezone: "America/Santiago",
		Server:  appkit.AppServer{Database: openMem},
		Modules: []appkit.Module{provider, consumer},
	}

	_, err := appkit.BuildServer(app, appkit.ServerEnv{DSN: "mem"}, ddl.Schema{})
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

	app := appkit.App{Log: func(...any) {}, Timezone: "America/Santiago",
		Server:  appkit.AppServer{Database: openMem},
		Modules: []appkit.Module{consumer},
	}

	_, err := appkit.BuildServer(app, appkit.ServerEnv{DSN: "mem"}, ddl.Schema{})
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

	app := appkit.App{Log: func(...any) {}, Timezone: "America/Santiago",
		Server:  appkit.AppServer{Database: openMem},
		Modules: []appkit.Module{consumer, provider},
	}

	_, err := appkit.BuildServer(app, appkit.ServerEnv{DSN: "mem"}, ddl.Schema{})
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

	app := appkit.App{Log: func(...any) {}, Timezone: "America/Santiago",
		Server:  appkit.AppServer{Database: openMem},
		Modules: []appkit.Module{provider, provider2, consumer},
	}

	_, err := appkit.BuildServer(app, appkit.ServerEnv{DSN: "mem"}, ddl.Schema{})
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

	app := appkit.App{Log: func(...any) {}, Timezone: "America/Santiago",
		Server:  appkit.AppServer{Database: openMem},
		Modules: []appkit.Module{provider},
	}

	_, err := appkit.BuildServer(app, appkit.ServerEnv{}, ddl.Schema{})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if len(calls) != 0 {
		t.Fatalf("expected calls to be empty, got %v", calls)
	}
}

func TestBuildServer_DSNWithoutOpen(t *testing.T) {
	calls = nil
	app := appkit.App{Log: func(...any) {}, Timezone: "America/Santiago"}

	_, err := appkit.BuildServer(app, appkit.ServerEnv{DSN: "x"}, ddl.Schema{})
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !errors.Is(err, appkit.ErrNoOpen) {
		t.Fatalf("expected ErrNoOpen, got %v", err)
	}
}

func TestBuildServer_NoTimezone(t *testing.T) {
	app := appkit.App{Log: func(...any) {}}
	_, err := appkit.BuildServer(app, appkit.ServerEnv{DSN: "mem"}, ddl.Schema{})
	if !errors.Is(err, appkit.ErrNoTimezone) {
		t.Fatalf("expected ErrNoTimezone, got %v", err)
	}
}

func TestApp_NoLogIsAnError(t *testing.T) {
	app := appkit.App{Timezone: "America/Santiago", Server: appkit.AppServer{Database: openMem}}
	if _, err := appkit.BuildServer(app, appkit.ServerEnv{DSN: "mem"}, ddl.Schema{}); !errors.Is(err, appkit.ErrNoLog) {
		t.Fatalf("BuildServer without App.Log: want ErrNoLog, got %v", err)
	}
	if _, err := appkit.BuildModules(app, orm.New(mem.New())); !errors.Is(err, appkit.ErrNoLog) {
		t.Fatalf("BuildModules without App.Log: want ErrNoLog, got %v", err)
	}
}
