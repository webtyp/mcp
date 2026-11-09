package mcp_test

import (
	"webtyp.com/crud"
	"webtyp.com/events"
	"webtyp.com/mcp"
	"webtyp.com/model"
	"webtyp.com/router"
)

// toolOf is the tool name of one action on an entity served by a module.
func toolOf(module string, rec model.ModuleNaming, a model.Action) mcp.ToolName {
	return mcp.ToolNameOf(router.Operation{Module: module, Entity: rec.ModelName(), Action: a})
}

type recorder struct {
	got []events.Event
}

func (r *recorder) Publish(e events.Event) {
	r.got = append(r.got, e)
}

type memStore struct {
	data map[string]string
}

func (s *memStore) NewID() string {
	return "rec123"
}

func (s *memStore) Create(rec model.Model) error {
	s.data[rec.ModelName()] = "stored"
	return nil
}

func (s *memStore) ReadOne(rec model.Model, id string) error {
	return model.NotFound("not found")
}

func (s *memStore) ReadAll(newRow func() model.Model, q crud.Query, onRow func(model.Model)) error {
	return model.NotFound("not found")
}

func (s *memStore) UpdateFields(rec model.Model, id string, fields []string) error {
	return model.NotFound("not found")
}

func (s *memStore) Delete(rec model.Model, id string) error {
	return model.NotFound("not found")
}

type mockStore struct { memStore }

func (s *mockStore) ReadAll(newRow func() model.Model, q crud.Query, onRow func(model.Model)) error {
	row := newRow().(*fakeRecord)
	row.Value = "user:user123"
	onRow(row)
	return nil
}
