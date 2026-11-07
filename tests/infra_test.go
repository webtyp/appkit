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
	"webtyp.com/router"
)

type modWithAPI struct {
	fakeOps
}

func (m modWithAPI) MountAPI(r router.Router) {
	r.Get("/x", func(ctx router.Context) {
		ctx.Write([]byte("ok"))
	})
}

func TestBuildModules_APIModuleIsNotMounted(t *testing.T) {
	m := appkit.Module{
		Name: "api_mod",
		Server: appkit.ModuleServer{
			Build: func(d appkit.Deps) (model.ModuleNaming, error) {
				return modWithAPI{}, nil
			},
		},
	}
	app := appkit.App{
		Log:      func(...any) {},
		Timezone: "America/Santiago",
		TenantID: "test",
		Server:   appkit.AppServer{Database: openMem},
		Modules:  []appkit.Module{m},
	}
	srv, err := appkit.BuildServer(app, appkit.ServerEnv{DSN: "x"}, ddl.Schema{})
	if err != nil {
		t.Fatalf("BuildServer: %v", err)
	}
	for _, r := range srv.Router().Routes() {
		if r.Path == "/x" {
			t.Fatal("route /x from domain module MountAPI was mounted, want excluded")
		}
	}
}

type fakeInfra struct{}

func (fakeInfra) ModelName() string { return "fake_infra" }
func (fakeInfra) MountAPI(r router.Router) {
	r.Get("/x", func(ctx router.Context) {
		ctx.Write([]byte("infra ok"))
	}).Public()
}

func TestInfra_Mounted(t *testing.T) {
	app := appkit.App{
		Log:      func(...any) {},
		Timezone: "America/Santiago",
		TenantID: "test",
		Server: appkit.AppServer{
			Database: openMem,
			Infra: []appkit.Infra{
				{
					Name: "auth",
					Build: func(d appkit.Deps) (router.APIModule, error) {
						return fakeInfra{}, nil
					},
				},
			},
		},
	}
	srv, err := appkit.BuildServer(app, appkit.ServerEnv{DSN: "x"}, ddl.Schema{})
	if err != nil {
		t.Fatalf("BuildServer: %v", err)
	}
	found := false
	for _, r := range srv.Router().Routes() {
		if r.Path == "/x" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("route /x from Infra was not mounted")
	}
}

type plainNaming struct{}

func (plainNaming) ModelName() string { return "plain" }

func TestBuildModules_NeitherCrudNorPartsIsError(t *testing.T) {
	m := appkit.Module{
		Name: "plain_mod",
		Server: appkit.ModuleServer{
			Build: func(d appkit.Deps) (model.ModuleNaming, error) {
				return plainNaming{}, nil
			},
		},
	}
	app := appkit.App{
		Log:      func(...any) {},
		Timezone: "America/Santiago",
		TenantID: "test",
		Server:   appkit.AppServer{Database: openMem},
		Modules:  []appkit.Module{m},
	}
	_, err := appkit.BuildServer(app, appkit.ServerEnv{DSN: "x"}, ddl.Schema{})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, appkit.ErrNotCrudModule) {
		t.Fatalf("expected ErrNotCrudModule, got %v", err)
	}
}

func TestPublicReads_Mounted(t *testing.T) {
	m := appkit.Module{
		Name: "fake_mod",
		Server: appkit.ModuleServer{
			Build: func(d appkit.Deps) (model.ModuleNaming, error) {
				return fakeOps{}, nil
			},
		},
	}
	app := appkit.App{
		Log:      func(...any) {},
		Timezone: "America/Santiago",
		TenantID: "test",
		Server: appkit.AppServer{
			Database:    openMem,
			PublicReads: []model.ModuleNaming{fakeModel{}},
		},
		Modules: []appkit.Module{m},
	}
	srv, err := appkit.BuildServer(app, appkit.ServerEnv{DSN: "x"}, ddl.Schema{})
	if err != nil {
		t.Fatalf("BuildServer: %v", err)
	}
	expectedSuffix := "/" + crud.Op(fakeModel{}, model.Read)
	found := false
	for _, r := range srv.Router().Routes() {
		if strings.HasSuffix(r.Path, expectedSuffix) {
			if r.Access != model.AccessPublic {
				t.Fatalf("expected AccessPublic, got %v", r.Access)
			}
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("route ending with %s not found", expectedSuffix)
	}
}

type otherModel struct{}

func (otherModel) ModelName() string { return "other" }

func TestPublicReads_UnknownIsError(t *testing.T) {
	m := appkit.Module{
		Name: "fake_mod",
		Server: appkit.ModuleServer{
			Build: func(d appkit.Deps) (model.ModuleNaming, error) {
				return fakeOps{}, nil
			},
		},
	}
	app := appkit.App{
		Log:      func(...any) {},
		Timezone: "America/Santiago",
		TenantID: "test",
		Server: appkit.AppServer{
			Database:    openMem,
			PublicReads: []model.ModuleNaming{otherModel{}},
		},
		Modules: []appkit.Module{m},
	}
	_, err := appkit.BuildServer(app, appkit.ServerEnv{DSN: "x"}, ddl.Schema{})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, appkit.ErrPublicReadNotMounted) {
		t.Fatalf("expected ErrPublicReadNotMounted, got %v", err)
	}
}
