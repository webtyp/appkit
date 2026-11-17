//go:build !wasm && !browser

package tests

import (
	"webtyp.com/crud"
	"webtyp.com/model"
)

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

func (fakeTableStore) NewID() string                            { return "1" }
func (fakeTableStore) ReadOne(rec model.Model, id string) error { return nil }
func (fakeTableStore) ReadAll(newRow func() model.Model, q crud.Query, onRow func(model.Model)) error {
	return nil
}
func (fakeTableStore) Create(rec model.Model) error                                   { return nil }
func (fakeTableStore) UpdateFields(rec model.Model, id string, fields []string) error { return nil }
func (fakeTableStore) Delete(rec model.Model, id string) error                        { return nil }

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
