//go:build !wasm

package tests

import (
	"errors"
	"testing"

	"webtyp.com/appkit"
	"webtyp.com/appkit/backend"
	"webtyp.com/auth"
	"webtyp.com/ddl"
	"webtyp.com/events/inprocess"
	"webtyp.com/model"
	"webtyp.com/orm"
	"webtyp.com/rbac"
	"webtyp.com/rpc"
	"webtyp.com/storage"
	"webtyp.com/storage/mem"
	"webtyp.com/unixid"
)

func TestBuild_RejectsUnsupportedIndustry(t *testing.T) {
	def := model.ModuleDefinition{
		Name:       "clinical_encounter",
		Industries: []model.Industry{model.IndustryHumanHealth},
	}
	app := appkit.App{
		Name:     "Clinic",
		TenantID: "clinic",
		Timezone: "UTC",
		Industry: model.IndustryGeneralBusiness,
		Modules:  []model.ModuleDefinition{def},
		Log:      func(...any) {},
	}
	b := backend.Backend{
		Events: &inprocess.Broker{},
		API:    rpc.JSON{},
		Modules: []backend.Module{{
			Name:  "clinical_encounter",
			Build: func(d backend.Deps) (model.ModuleNaming, error) { return namedMod{"clinical_encounter"}, nil },
		}},
	}
	s := backend.Server{NewIDs: func() (model.IDGenerator, error) { return unixid.NewUnixID() }}

	_, err := backend.Build(app, b, s, "", ddl.Schema{})
	if !errors.Is(err, appkit.ErrUnsupportedIndustry) {
		t.Fatalf("want ErrUnsupportedIndustry, got %v", err)
	}
}

func TestBuild_EnsuresAppRolesAndModuleUseGrants(t *testing.T) {
	conn := mem.New()
	def := model.ModuleDefinition{
		Name:       "clients",
		Industries: model.AnyIndustry(),
		Entities: []model.Definition{{
			Name:  "client",
			Allow: model.AllActions,
			Fields: model.Fields{
				{Name: "id", Type: model.Text(), DB: &model.FieldDB{PK: true}},
			},
		}},
	}
	app := appkit.App{
		Name:     "Clinic",
		TenantID: "clinic",
		Timezone: "UTC",
		Industry: model.IndustryHumanHealth,
		SignIn:   []auth.Mode{&fakeMode{}},
		Roles: []model.Role{
			{Code: "reception", Name: "Reception", Use: model.WorksWithAllRecords},
		},
		Modules: []model.ModuleDefinition{def},
		Log:     func(...any) {},
	}
	b := backend.Backend{
		Events: &inprocess.Broker{},
		API:    rpc.JSON{},
		Modules: []backend.Module{{
			Name:  "clients",
			Build: func(d backend.Deps) (model.ModuleNaming, error) { return namedMod{"clients"}, nil },
		}},
	}
	s := backend.Server{
		Database: func(string) (storage.Conn, error) { return conn, nil },
		NewIDs:   func() (model.IDGenerator, error) { return unixid.NewUnixID() },
	}

	_, err := backend.Build(app, b, s, "mem", ddl.Schema{})
	if err != nil {
		t.Fatal(err)
	}
	svc, err := rbac.New(orm.New(conn))
	if err != nil {
		t.Fatal(err)
	}
	role, err := svc.GetRoleByCode("clinic", "reception")
	if err != nil || role == nil || role.Name != "Reception" {
		t.Fatalf("expected reception role in rbac, got %+v, err=%v", role, err)
	}
}
