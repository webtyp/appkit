//go:build !wasm && !browser

package tests

import (
	"webtyp.com/crud"
	"webtyp.com/model"
	"webtyp.com/router"
	"webtyp.com/router/mock"
)

type fakeModel struct{}

func (fakeModel) ModelName() string { return "fake" }
func (fakeModel) Schema() []model.Field {
	return []model.Field{
		{Name: "id", Type: model.Text(), DB: &model.FieldDB{PK: true}},
	}
}
func (fakeModel) Pointers() []any                  { return []any{} }
func (fakeModel) IsNil() bool                      { return false }
func (fakeModel) EncodeFields(w model.FieldWriter) {}
func (fakeModel) DecodeFields(r model.FieldReader) {}

type fakeModelList []*fakeModel

func (l *fakeModelList) Append() model.Fielder {
	m := &fakeModel{}
	*l = append(*l, m)
	return m
}
func (l *fakeModelList) At(i int) model.Fielder           { return (*l)[i] }
func (l *fakeModelList) Len() int                         { return len(*l) }
func (l *fakeModelList) IsNil() bool                      { return l == nil }
func (l *fakeModelList) EncodeFields(w model.FieldWriter) {}
func (l *fakeModelList) DecodeFields(r model.FieldReader) {}

type fakeOps struct{}

func (fakeOps) ModelName() string { return "fake" }

func (fakeOps) Definition() model.ModuleDefinition {
	return model.ModuleDefinition{
		Name: "fake",
		Entities: []model.Definition{{
			Name:  "fake",
			Wire:  true,
			Allow: model.Read,
			Rules: model.Rules{Read: model.AddMyBusinessRule},
		}},
	}
}

func (fakeOps) MountOperations(reg crud.Registry) {
	reg.AddReadOnlyEntity(&fakeModel{}, nil, func(c crud.CurrentUser, args crud.ReadArgs, filter model.Model) (model.ModelSlice, error) {
		return &fakeModelList{}, nil
	})
}

func fakeAuthn(userID string) router.Middleware {
	return func(next router.HandlerFunc) router.HandlerFunc {
		return func(ctx router.Context) {
			ctx.SetUserID(userID)
			next(ctx)
		}
	}
}

func allowFakeRead(userID string, resource model.Resource, action model.Action) bool {
	return resource == "fake" && action == model.Read
}

func post(r *mock.Router, path, body string) (int, string) {
	ctx := &mock.Context{
		InMethod: "POST",
		InPath:   path,
		InBody:   []byte(body),
	}
	ctx.SetHeader("Content-Type", "application/json")
	r.Invoke("POST", path, ctx)
	status := ctx.Status
	if status == 0 {
		status = 200
	}
	return status, string(ctx.ResponseBody())
}

func get(r *mock.Router, path string) (int, string) {
	ctx := &mock.Context{
		InMethod: "GET",
		InPath:   path,
	}
	r.Invoke("GET", path, ctx)
	status := ctx.Status
	if status == 0 {
		status = 200
	}
	return status, string(ctx.ResponseBody())
}
