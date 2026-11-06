//go:build !wasm

package tests

import (
	"strings"
	"testing"

	"webtyp.com/appkit"
	"webtyp.com/ddl"
	"webtyp.com/model"
	"webtyp.com/router"
)

type dummyEntry struct{}

func (dummyEntry) ModelName() string        { return "dummy_entry" }
func (dummyEntry) Authn() router.Middleware { return nil }
func (dummyEntry) Authorize() model.Authorizer {
	return func(string, model.Resource, model.Action) bool { return true }
}
func (dummyEntry) Policy() model.PolicyDescriber { return nil }

func TestBuildServer_WithEntryModule(t *testing.T) {
	entryProvider := mod("entry", func(d appkit.Deps) (model.ModuleNaming, error) {
		return dummyEntry{}, nil
	})

	app := appkit.App{Log: func(...any) {}, Timezone: "America/Santiago", TenantID: "test",
		Server:  appkit.AppServer{Database: openMem},
		Modules: []appkit.Module{entryProvider},
	}

	_, err := appkit.BuildServer(app, appkit.ServerEnv{DSN: "x"}, ddl.Schema{})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
}

func TestBuildServer_WithoutEntryModule(t *testing.T) {
	var logged []string
	app := appkit.App{Timezone: "America/Santiago", TenantID: "test",
		Server: appkit.AppServer{Database: openMem},
		Log: func(args ...any) {
			for _, a := range args {
				logged = append(logged, a.(string))
			}
		},
	}

	_, err := appkit.BuildServer(app, appkit.ServerEnv{DSN: "x"}, ddl.Schema{})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	foundLog := false
	for _, l := range logged {
		if strings.Contains(l, "no entry module") {
			foundLog = true
			break
		}
	}
	if !foundLog {
		t.Fatalf("expected log about no entry module, got: %v", logged)
	}
}

func TestBrand_RenderHTML(t *testing.T) {
	b := appkit.Brand{Name: "Acme"}
	html := b.RenderHTML()
	if !strings.Contains(html, "login__card") {
		t.Fatalf("expected RenderHTML to contain login__card, got %q", html)
	}
	zero := appkit.Brand{}
	if zero.RenderHTML() != "" {
		t.Fatalf("expected zero brand RenderHTML to be empty, got %q", zero.RenderHTML())
	}
}
