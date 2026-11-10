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
	"webtyp.com/rbac"
	"webtyp.com/router"
	"webtyp.com/rpc"
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

type fakeTableModel struct {
	id string
}

func (m *fakeTableModel) ModelName() string   { return "fake" }
func (m *fakeTableModel) Allow() model.Action { return model.Create | model.Read }
func (m *fakeTableModel) Schema() []model.Field {
	return []model.Field{
		{Name: "id", Type: model.Text(), DB: &model.FieldDB{PK: true}},
	}
}
func (m *fakeTableModel) Pointers() []any                  { return []any{&m.id} }
func (m *fakeTableModel) IsNil() bool                      { return m == nil }
func (m *fakeTableModel) EncodeFields(w model.FieldWriter) {}
func (m *fakeTableModel) DecodeFields(r model.FieldReader) {}

type fakeTableStore struct{}

func (fakeTableStore) NewID() string                                                               { return "1" }
func (fakeTableStore) ReadOne(rec model.Model, id string) error                                    { return nil }
func (fakeTableStore) ReadAll(newRow func() model.Model, q crud.Query, onRow func(model.Model)) error {
	return nil
}
func (fakeTableStore) Create(rec model.Model) error                          { return nil }
func (fakeTableStore) UpdateFields(rec model.Model, id string, fields []string) error { return nil }
func (fakeTableStore) Delete(rec model.Model, id string) error               { return nil }

type fakeTableOps struct{}

func (fakeTableOps) ModelName() string { return "fake" }

func (fakeTableOps) Definition() model.ModuleDefinition {
	return model.ModuleDefinition{
		Name: "fake",
		Entities: []model.Definition{{
			Name:  (&fakeTableModel{}).ModelName(),
			Allow: (&fakeTableModel{}).Allow(),
			Rules: model.Rules{
				Create: model.HaveNoBusinessRule,
				Read:   model.HaveNoBusinessRule,
			},
		}},
	}
}

func (fakeTableOps) MountOperations(reg crud.Registry) {
	reg.AddStoredEntity(crud.StoredEntity{
		Model:   &fakeTableModel{},
		NewList: func() model.ModelSlice { return &fakeModelList{} },
		Store:   fakeTableStore{},
		Rules: model.Rules{
			Create: model.HaveNoBusinessRule,
			Read:   model.HaveNoBusinessRule,
		},
	})
}

func TestPublic_CreateMounted(t *testing.T) {
	m := appkit.Module{
		Name: "fake_mod",
		Server: appkit.ModuleServer{
			Build: func(d appkit.Deps) (model.ModuleNaming, error) {
				return fakeTableOps{}, nil
			},
		},
	}
	app := appkit.App{
		Log:      func(...any) {},
		Timezone: "America/Santiago",
		TenantID: "test",
		Server: appkit.AppServer{
			Database: openMem,
			Public: []crud.Public{
				{Model: &fakeTableModel{}, Actions: model.Create},
			},
		},
		Modules: []appkit.Module{m},
	}
	srv, err := appkit.BuildServer(app, appkit.ServerEnv{DSN: "x"}, ddl.Schema{})
	if err != nil {
		t.Fatalf("BuildServer: %v", err)
	}
	expectedMethod := rpc.MethodOf(model.Create)
	expectedSuffix := "/" + (&fakeTableModel{}).ModelName()
	found := false
	for _, r := range srv.Router().Routes() {
		if r.Method == expectedMethod && strings.HasSuffix(r.Path, expectedSuffix) {
			if r.Access != model.AccessPublic {
				t.Fatalf("expected AccessPublic, got %v", r.Access)
			}
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("route with method %s ending with %s not found", expectedMethod, expectedSuffix)
	}
}

type otherModel struct{}

func (otherModel) ModelName() string { return "other" }

func TestPublic_UnknownIsError(t *testing.T) {
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
			Database: openMem,
			Public: []crud.Public{
				{Model: otherModel{}, Actions: model.Read},
			},
		},
		Modules: []appkit.Module{m},
	}
	_, err := appkit.BuildServer(app, appkit.ServerEnv{DSN: "x"}, ddl.Schema{})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, appkit.ErrPublicNotMounted) {
		t.Fatalf("expected ErrPublicNotMounted, got %v", err)
	}
}

type fakeGranter struct {
	calls []grantCall
}

type grantCall struct {
	projectID string
	role      model.RoleCode
	grants    []model.Grant
}

func (f *fakeGranter) EnsureGrants(projectID string, role model.RoleCode, grants []model.Grant) error {
	f.calls = append(f.calls, grantCall{projectID: projectID, role: role, grants: grants})
	return nil
}

func TestGrants_AppliedAtBoot(t *testing.T) {
	granter := &fakeGranter{}
	grants := []model.Grant{
		{Resource: "fake", Actions: model.Read},
	}
	m := appkit.Module{
		Name: "fake_mod",
		Server: appkit.ModuleServer{
			DefaultGrants: grants,
			Build: func(d appkit.Deps) (model.ModuleNaming, error) {
				return fakeOps{}, nil
			},
		},
	}
	app := appkit.App{
		Log:      func(...any) {},
		Timezone: "America/Santiago",
		TenantID: "tenant-123",
		Server: appkit.AppServer{
			Database: openMem,
			Grants:   granter,
		},
		Modules: []appkit.Module{m},
	}
	_, err := appkit.BuildServer(app, appkit.ServerEnv{DSN: "x"}, ddl.Schema{})
	if err != nil {
		t.Fatalf("BuildServer: %v", err)
	}
	if len(granter.calls) != 1 {
		t.Fatalf("expected 1 call to EnsureGrants, got %d", len(granter.calls))
	}
	call := granter.calls[0]
	if call.projectID != "tenant-123" {
		t.Errorf("projectID: got %q, want tenant-123", call.projectID)
	}
	if call.role != rbac.RoleAuthenticated {
		t.Errorf("role: got %q, want %q", call.role, rbac.RoleAuthenticated)
	}
	if len(call.grants) != 1 || call.grants[0] != grants[0] {
		t.Errorf("grants: got %+v, want %+v", call.grants, grants)
	}
}

func TestGrants_MissingGranterIsError(t *testing.T) {
	m := appkit.Module{
		Name: "fake_mod",
		Server: appkit.ModuleServer{
			DefaultGrants: []model.Grant{
				{Resource: "fake", Actions: model.Read},
			},
			Build: func(d appkit.Deps) (model.ModuleNaming, error) {
				return fakeOps{}, nil
			},
		},
	}
	app := appkit.App{
		Log:      func(...any) {},
		Timezone: "America/Santiago",
		TenantID: "tenant-123",
		Server: appkit.AppServer{
			Database: openMem,
			Grants:   nil,
		},
		Modules: []appkit.Module{m},
	}
	_, err := appkit.BuildServer(app, appkit.ServerEnv{DSN: "x"}, ddl.Schema{})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, appkit.ErrNoGrants) {
		t.Fatalf("expected ErrNoGrants, got %v", err)
	}
}
