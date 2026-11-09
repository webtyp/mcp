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

// readOnlyDefinition is the Definition of a module whose only entity is a read-only Wire model answered by its own rule.
func readOnlyDefinition(module string, rec model.ModuleNaming) model.ModuleDefinition {
	return model.ModuleDefinition{Name: module, Entities: []model.Definition{{
		Name: rec.ModelName(), Wire: true, Allow: model.Read, Rules: model.Rules{Read: model.AddMyBusinessRule}}}}
}

// recorder is an events.Publisher that remembers what it was given.
type recorder struct{ got []events.Event }

func (r *recorder) Publish(e events.Event) { r.got = append(r.got, e) }

var _ events.Publisher = (*recorder)(nil)

// memStore is a crud.Store for the records of these tests: only NewID and Create hold data.
type memStore struct{ created []model.Model }

func (s *memStore) NewID() string { return "id-1" }
func (s *memStore) ReadOne(rec model.Model, id string) error {
	return model.NotFound("not found")
}
func (s *memStore) ReadAll(newRow func() model.Model, q crud.Query, onRow func(model.Model)) error {
	return nil
}
func (s *memStore) Create(rec model.Model) error {
	s.created = append(s.created, rec)
	return nil
}
func (s *memStore) UpdateFields(rec model.Model, id string, fields []string) error {
	return model.NotFound("not found")
}
func (s *memStore) Delete(rec model.Model, id string) error { return model.NotFound("not found") }

var _ crud.Store = (*memStore)(nil)
