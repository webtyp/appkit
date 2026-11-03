//go:build !wasm

package tests

import (
	"errors"
	"strings"
	"testing"

	"webtyp.com/appkit"
	"webtyp.com/ddl"
	"webtyp.com/model"
)

func TestSteps_LoginByRUTFirst(t *testing.T) {
	app := appkit.App{TenantID: "test",
		Login:   appkit.LoginByRUT{},
		Modules: []appkit.Module{mod("a", nil)},
	}
	steps := appkit.Steps(app)
	if len(steps) != 4 {
		t.Fatalf("expected 4 steps, got %d", len(steps))
	}
	expected := []string{"auth", "rbac", "access", "a"}
	for i, s := range steps {
		if s.Name != expected[i] {
			t.Errorf("step %d: expected %s, got %s", i, expected[i], s.Name)
		}
	}
}

func TestSteps_NoLoginNoAuthSteps(t *testing.T) {
	app := appkit.App{TenantID: "test",
		Modules: []appkit.Module{mod("a", nil)},
	}
	steps := appkit.Steps(app)
	if len(steps) != 1 {
		t.Fatalf("expected 1 step, got %d", len(steps))
	}
	if steps[0].Name != "a" {
		t.Errorf("expected step 'a', got %s", steps[0].Name)
	}
}

func TestBuildServer_LoginNeedsTrustedIPStore(t *testing.T) {
	app := appkit.App{TenantID: "test",
		Login:  appkit.LoginByRUT{},
		Server: appkit.AppServer{Database: openMem},
	}
	_, err := appkit.BuildServer(app, appkit.ServerEnv{DSN: "x"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, appkit.ErrNoProvider) {
		t.Fatalf("expected ErrNoProvider, got %v", err)
	}
}

type trustedIPMod struct{}
func (trustedIPMod) ModelName() string { return "trusted" }
func (trustedIPMod) IsTrustedIP(userID, ip string) bool { return true }

type fakeCompiler struct{}
func (fakeCompiler) CompileDDL(b interface{}, s *ddl.Step) {}

func TestBuildServer_LoginByRUT(t *testing.T) {
	trustedProvider := mod("trusted", func(d appkit.Deps) (model.ModuleNaming, error) {
		return trustedIPMod{}, nil
	})

	app := appkit.App{TenantID: "test",
		Login:   appkit.LoginByRUT{},
		Server:  appkit.AppServer{Database: openMem},
		Modules: []appkit.Module{trustedProvider},
	}

	_, err := appkit.BuildServer(app, appkit.ServerEnv{DSN: "x"})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
}

func TestBrand_ZeroRendersNothing(t *testing.T) {
	b := appkit.Brand{}
	if pages := b.RenderPages(); pages != nil {
		t.Errorf("expected nil pages, got %v", pages)
	}
	if html := b.RenderHTML(); html != "" {
		t.Errorf("expected empty html, got %q", html)
	}
}

func TestBrand_RendersLogin(t *testing.T) {
	b := appkit.Brand{Name: "Acme"}
	pages := b.RenderPages()
	if len(pages) != 1 {
		t.Fatalf("expected 1 page, got %d", len(pages))
	}
	if pages[0].Path != "/" {
		t.Errorf("expected Path '/', got %q", pages[0].Path)
	}
	if !strings.Contains(pages[0].Body, "Acme") {
		t.Errorf("expected Body to contain 'Acme', got %q", pages[0].Body)
	}
}
