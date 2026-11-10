//go:build !wasm

package tests

import (
	"sort"
	"strconv"
	"sync"

	"webtyp.com/crud"
	"webtyp.com/events"
	"webtyp.com/fmt"
	"webtyp.com/model"
	"webtyp.com/storage"
)

// recorder records events published on it.
type recorder struct {
	mu  sync.Mutex
	got []events.Event
}

func (r *recorder) Publish(e events.Event) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.got = append(r.got, e)
}

func (r *recorder) events() []events.Event {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]events.Event, len(r.got))
	copy(out, r.got)
	return out
}

type tableModel struct {
	ID   string
	Name string
}

func (m *tableModel) ModelName() string   { return "rec" }
func (m *tableModel) Allow() model.Action { return model.AllActions }
func (m *tableModel) IsNil() bool         { return m == nil }

func (m *tableModel) Schema() []model.Field {
	return []model.Field{
		{Name: "id", Type: model.Text(), DB: &model.FieldDB{PK: true, AutoInc: true}},
		{Name: "name", Type: model.Text()},
	}
}

func (m *tableModel) Pointers() []any {
	return []any{&m.ID, &m.Name}
}

func (m *tableModel) EncodeFields(w model.FieldWriter) {
	w.String("id", m.ID)
	w.String("name", m.Name)
}

func (m *tableModel) DecodeFields(r model.FieldReader) {
	if v, ok := r.String("id"); ok {
		m.ID = v
	}
	if v, ok := r.String("name"); ok {
		m.Name = v
	}
}

type tableModelList []*tableModel

func (l *tableModelList) Append() model.Fielder {
	m := &tableModel{}
	*l = append(*l, m)
	return m
}

func (l *tableModelList) At(i int) model.Fielder { return (*l)[i] }
func (l *tableModelList) Len() int             { return len(*l) }
func (l *tableModelList) IsNil() bool          { return l == nil }

func (l *tableModelList) EncodeFields(w model.FieldWriter) {
	a := w.Array("records", len(*l))
	for i := range *l {
		a.Object((*l)[i])
	}
}

func (l *tableModelList) DecodeFields(r model.FieldReader) {
	if a, ok := r.Array("records"); ok {
		for i := 0; i < a.Len(); i++ {
			f := l.Append().(model.Decodable)
			a.Object(i, f)
		}
	}
}

type tableModule struct {
	store crud.Store
}

func (m tableModule) ModelName() string { return "tmod" }

func (m tableModule) Definition() model.ModuleDefinition {
	return model.ModuleDefinition{
		Name: "tmod",
		Entities: []model.Definition{{
			Name:  "rec",
			Allow: model.AllActions,
			Rules: model.Rules{
				Create: model.HaveNoBusinessRule,
				Read:   model.HaveNoBusinessRule,
				Update: model.HaveNoBusinessRule,
				Delete: model.HaveNoBusinessRule,
			},
		}},
	}
}

func (m tableModule) MountOperations(reg crud.Registry) {
	reg.AddStoredEntity(crud.StoredEntity{
		Model:   &tableModel{},
		NewList: func() model.ModelSlice { return &tableModelList{} },
		Store:   m.store,
		Rules: model.Rules{
			Create: model.HaveNoBusinessRule,
			Read:   model.HaveNoBusinessRule,
			Update: model.HaveNoBusinessRule,
			Delete: model.HaveNoBusinessRule,
		},
	})
}

type memStore struct {
	mu     sync.Mutex
	rows   map[string]map[string]any
	nextID int
}

func newMemStore() *memStore {
	return &memStore{rows: make(map[string]map[string]any)}
}

func (s *memStore) NewID() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nextID++
	return strconv.Itoa(s.nextID)
}

func (s *memStore) ReadOne(rec model.Model, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	row, ok := s.rows[id]
	if !ok {
		return model.NotFound("not found")
	}
	schema := rec.Schema()
	ptrs := rec.Pointers()
	for i, f := range schema {
		if v, ok := row[f.Name]; ok {
			if p, ok := ptrs[i].(*string); ok {
				*p = v.(string)
			}
		}
	}
	return nil
}

func (s *memStore) Create(rec model.Model) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	schema := rec.Schema()
	ptrs := rec.Pointers()
	row := make(map[string]any)
	var id string
	for i, f := range schema {
		if p, ok := ptrs[i].(*string); ok {
			row[f.Name] = *p
			if f.IsPK() {
				id = *p
			}
		}
	}
	if id == "" {
		s.nextID++
		id = strconv.Itoa(s.nextID)
		for i, f := range schema {
			if f.IsPK() {
				if p, ok := ptrs[i].(*string); ok {
					*p = id
					row[f.Name] = id
				}
			}
		}
	}
	if _, exists := s.rows[id]; exists {
		return storage.Duplicate(fmt.Err("duplicate id", id))
	}
	s.rows[id] = row
	return nil
}

func (s *memStore) UpdateFields(rec model.Model, id string, fields []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	row, ok := s.rows[id]
	if !ok {
		return model.NotFound("not found")
	}
	schema := rec.Schema()
	ptrs := rec.Pointers()
	for _, fName := range fields {
		for i, f := range schema {
			if f.Name == fName {
				if p, ok := ptrs[i].(*string); ok {
					row[f.Name] = *p
				}
			}
		}
	}
	return nil
}

func (s *memStore) Delete(rec model.Model, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.rows[id]; !ok {
		return model.NotFound("not found")
	}
	delete(s.rows, id)
	return nil
}

func (s *memStore) ReadAll(newRow func() model.Model, q crud.Query, onRow func(model.Model)) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	var ids []string
	for id := range s.rows {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		row := s.rows[id]
		m := newRow()
		schema := m.Schema()
		ptrs := m.Pointers()
		for i, f := range schema {
			if v, ok := row[f.Name]; ok {
				if p, ok := ptrs[i].(*string); ok {
					*p = v.(string)
				}
			}
		}
		onRow(m)
	}
	return nil
}
