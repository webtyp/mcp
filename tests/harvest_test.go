package mcp_test

import (
	"strings"
	"testing"

	"webtyp.com/context"
	"webtyp.com/crud"
	"webtyp.com/mcp"
	"webtyp.com/model"
	"webtyp.com/storage"
)

type fakeArgs struct{ Value string }

func (a *fakeArgs) IsNil() bool                      { return a == nil }
func (a *fakeArgs) Schema() []model.Field            { return nil }
func (a *fakeArgs) Pointers() []any                  { return []any{&a.Value} }
func (a *fakeArgs) EncodeFields(w model.FieldWriter) { w.String("value", a.Value) }
func (a *fakeArgs) DecodeFields(r model.FieldReader) {
	if v, ok := r.String("value"); ok {
		a.Value = v
	}
}

type fakeRecord struct {
	ID    string
	Value string
}

func (r *fakeRecord) ModelName() string { return "fake_rec" }
func (r *fakeRecord) IsNil() bool       { return r == nil }
func (r *fakeRecord) Schema() []model.Field {
	return []model.Field{
		{Name: "id", Type: model.Text(), DB: &model.FieldDB{PK: true}},
		{Name: "value", Type: model.Text()},
	}
}
func (r *fakeRecord) Pointers() []any                  { return []any{&r.ID, &r.Value} }
func (r *fakeRecord) Allow() model.Action { return model.Read | model.Create }
func (r *fakeRecord) EncodeFields(w model.FieldWriter) { w.String("id", r.ID); w.String("value", r.Value) }
func (r *fakeRecord) DecodeFields(d model.FieldReader) {
	r.ID, _ = d.String("id")
	r.Value, _ = d.String("value")
}

type fakeRecordList []*fakeRecord

func (s *fakeRecordList) Len() int                         { return len(*s) }
func (s *fakeRecordList) At(i int) model.Fielder           { return (*s)[i] }
func (s *fakeRecordList) Append() model.Fielder            { v := &fakeRecord{}; *s = append(*s, v); return v }
func (s *fakeRecordList) IsNil() bool                      { return s == nil }
func (s *fakeRecordList) EncodeFields(_ model.FieldWriter) {}
func (s *fakeRecordList) DecodeFields(_ model.FieldReader) {}

// fakeModule mimics a domain module: implements crud.Module.
type fakeModule struct{}

func (fakeModule) ModelName() string { return "fake" }
func (m fakeModule) Definition() model.ModuleDefinition {
	return model.ModuleDefinition{Name: m.ModelName(), Entities: []model.Definition{{Name: (&fakeRecord{}).ModelName(), Wire: true, Allow: model.Read, Rules: model.Rules{Read: model.AddMyBusinessRule}}}}
}
func (fakeModule) MountOperations(reg crud.Registry) {
	reg.AddReadOnlyEntity(&fakeRecord{}, nil, func(c crud.CurrentUser, args crud.ReadArgs, filter model.Model) (model.ModelSlice, error) {
		out := fakeRecordList{&fakeRecord{Value: "echo:" + args.Search}}
		return &out, nil
	}).Describe("Do thing")
}

var _ crud.Module = fakeModule{}

type fakeMeModuleA struct{}

func (fakeMeModuleA) ModelName() string { return "fake_a" }
func (m fakeMeModuleA) Definition() model.ModuleDefinition {
	return model.ModuleDefinition{Name: m.ModelName(), Entities: []model.Definition{{Name: (&fakeRecord{}).ModelName(), Wire: true, Allow: model.Read, Rules: model.Rules{Read: model.AddMyBusinessRule}}}}
}
func (fakeMeModuleA) MountOperations(reg crud.Registry) {
	reg.AddReadOnlyEntity(&fakeRecord{}, nil, func(c crud.CurrentUser, args crud.ReadArgs, filter model.Model) (model.ModelSlice, error) {
		out := fakeRecordList{}
		return &out, nil
	})
}

var _ crud.Module = fakeMeModuleA{}

type fakeMeModuleB struct{}

func (fakeMeModuleB) ModelName() string { return "fake_b" }
func (m fakeMeModuleB) Definition() model.ModuleDefinition {
	return model.ModuleDefinition{Name: m.ModelName(), Entities: []model.Definition{{Name: (&fakeRecord{}).ModelName(), Wire: true, Allow: model.Read, Rules: model.Rules{Read: model.AddMyBusinessRule}}}}
}
func (fakeMeModuleB) MountOperations(reg crud.Registry) {
	reg.AddReadOnlyEntity(&fakeRecord{}, nil, func(c crud.CurrentUser, args crud.ReadArgs, filter model.Model) (model.ModelSlice, error) {
		out := fakeRecordList{}
		return &out, nil
	})
}

var _ crud.Module = fakeMeModuleB{}

type fakeTwoOpsModule struct{}

func (fakeTwoOpsModule) ModelName() string { return "fake_two" }
func (m fakeTwoOpsModule) Definition() model.ModuleDefinition {
	return model.ModuleDefinition{Name: m.ModelName(), Entities: []model.Definition{{Name: (&fakeRecord{}).ModelName(), Allow: model.Read | model.Create, Rules: model.Rules{Read: model.AddMyBusinessRule, Create: model.AddMyBusinessRule}}}}
}
func (fakeTwoOpsModule) MountOperations(reg crud.Registry) {
	reg.AddStoredEntity(crud.StoredEntity{
		Model: &fakeRecord{},
		Store: &memStore{data: make(map[string]string)},
		NewList: func() model.ModelSlice { return &fakeRecordList{} },
		Rules: model.Rules{Read: model.AddMyBusinessRule, Create: model.AddMyBusinessRule},
		Search: func(c crud.CurrentUser, text string) ([]storage.Condition, error) { return nil, nil },
		Create: func(a crud.CurrentUser, tx crud.Tx, rec model.Model) (crud.RuleAnswer, error) { return crud.ApplyAndPublish, nil },
	})
}

var _ crud.Module = fakeTwoOpsModule{}

type fakeUnnamedModule struct{}

func (fakeUnnamedModule) ModelName() string { return "" }
func (m fakeUnnamedModule) Definition() model.ModuleDefinition {
	return model.ModuleDefinition{}
}
func (fakeUnnamedModule) MountOperations(reg crud.Registry) {
	reg.AddReadOnlyEntity(&fakeRecord{}, nil, func(c crud.CurrentUser, args crud.ReadArgs, filter model.Model) (model.ModelSlice, error) {
		out := fakeRecordList{}
		return &out, nil
	})
}

var _ crud.Module = fakeUnnamedModule{}

func TestHarvestOps_EmptyModelNameError(t *testing.T) {
	_, err := mcp.HarvestOps(nil, nil, fakeUnnamedModule{})
	if err == nil {
		t.Fatal("expected error when a module's ModelName() is empty, got nil")
	}
	if !strings.Contains(err.Error(), "empty ModelName()") {
		t.Fatalf("error message %v does not explain the empty ModelName()", err)
	}
}

func TestHarvestOps_SameBareNameAcrossModulesNoLongerCollides(t *testing.T) {

	provider, err := mcp.HarvestOps(nil, []mcp.ToolName{toolOf("fake_a", &fakeRecord{}, model.Read), toolOf("fake_b", &fakeRecord{}, model.Read)}, fakeMeModuleA{}, fakeMeModuleB{})
	if err != nil {
		t.Fatalf("HarvestOps: %v", err)
	}
	tools := provider.Tools()
	if len(tools) != 2 {
		t.Fatalf("expected 2 harvested tools, got %d: %+v", len(tools), tools)
	}
	names := map[string]bool{}
	for _, tool := range tools {
		names[tool.Name] = true
	}
	wantA := string(toolOf("fake_a", &fakeRecord{}, model.Read))
	wantB := string(toolOf("fake_b", &fakeRecord{}, model.Read))
	if !names[wantA] || !names[wantB] {
		t.Fatalf("expected qualified names %q and %q, got %v", wantA, wantB, names)
	}
}

func TestHarvestOps_DistinctNamesNoPanic(t *testing.T) {


	provider, err := mcp.HarvestOps(nil, []mcp.ToolName{toolOf("fake_two", &fakeRecord{}, model.Read), toolOf("fake_two", &fakeRecord{}, model.Create)}, fakeTwoOpsModule{})
	if err != nil {
		t.Fatalf("HarvestOps: %v", err)
	}
	tools := provider.Tools()
	if len(tools) != 2 {
		t.Fatalf("expected 2 harvested tools, got %d", len(tools))
	}
}

func TestHarvestOps_SameModuleInstanceTwiceError(t *testing.T) {
	fm := fakeModule{}
	_, err := mcp.HarvestOps(nil, nil, fm, fm)
	if err == nil {
		t.Fatal("expected error when the same module instance is harvested twice, got nil")
	}
	want := "duplicate tool name " + string(toolOf("fake", &fakeRecord{}, model.Read))
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("error message %v does not contain %q", err, want)
	}
}

func TestHarvestOps_ModuleReachesMCP(t *testing.T) {

	provider, err := mcp.HarvestOps(nil, []mcp.ToolName{toolOf("fake", &fakeRecord{}, model.Read)}, fakeModule{})
	if err != nil {
		t.Fatalf("HarvestOps: %v", err)
	}
	tools := provider.Tools()
	if len(tools) != 1 {
		t.Fatalf("expected 1 harvested tool, got %d", len(tools))
	}
	tool := tools[0]
	if tool.Name != string(toolOf("fake", &fakeRecord{}, model.Read)) || tool.Resource != "fake_rec" || tool.Action != model.Read {
		t.Fatalf("harvested tool metadata mismatch: %+v", tool)
	}
	if tool.Args == nil {
		t.Fatal("expected Accepts(...) to populate Tool.Args")
	}

	res, err := tool.Execute(nil, mcp.Request{Params: mcp.CallToolParams{Arguments: `{"search":"x"}`}})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if res.IsError {
		t.Fatalf("unexpected IsError, content: %s", res.Content)
	}
	if !strings.Contains(res.Content, `"value":"echo:x"`) {
		t.Errorf("unexpected content: %s", res.Content)
	}
}

type fakeAuthAndGuardedModule struct{}

func (fakeAuthAndGuardedModule) ModelName() string { return "fake_auth_guarded" }
func (m fakeAuthAndGuardedModule) Definition() model.ModuleDefinition {
	return model.ModuleDefinition{Name: m.ModelName(), Entities: []model.Definition{{Name: (&fakeRecord{}).ModelName(), Allow: model.Read | model.Create, Rules: model.Rules{Read: model.AddMyBusinessRule, Create: model.AddMyBusinessRule}}}}
}
func (fakeAuthAndGuardedModule) MountOperations(reg crud.Registry) {
	reg.AddStoredEntity(crud.StoredEntity{
		Model: &fakeRecord{},
		Store: &mockStore{},
		NewList: func() model.ModelSlice { return &fakeRecordList{} },
		Rules: model.Rules{Read: model.AddMyBusinessRule, Create: model.AddMyBusinessRule},
		Search: func(c crud.CurrentUser, text string) ([]storage.Condition, error) { return nil, nil },
		Create: func(a crud.CurrentUser, tx crud.Tx, rec model.Model) (crud.RuleAnswer, error) { return crud.ApplyAndPublish, nil },
	})
}

var _ crud.Module = fakeAuthAndGuardedModule{}

func TestHarvestOps_AuthenticatedAndGuardedOps(t *testing.T) {


	provider, err := mcp.HarvestOps(nil, []mcp.ToolName{toolOf("fake_auth_guarded", &fakeRecord{}, model.Read), toolOf("fake_auth_guarded", &fakeRecord{}, model.Create)}, fakeAuthAndGuardedModule{})
	if err != nil {
		t.Fatalf("HarvestOps: %v", err)
	}
	srv, err := mcp.NewServer(mcp.Config{
		Name:      "test-server",
		Version:   "1.0.0",
		Authorize: mcp.AllowAll,
	}, []mcp.ToolProvider{provider})
	if err != nil {
		t.Fatalf("NewServer failed to accept harvested tools: %v", err)
	}

	ctx := context.Background()
	ctx.Set(mcp.CtxKeyUserID, "user123")

	resMe, err := callTool(srv, ctx, string(toolOf("fake_auth_guarded", &fakeRecord{}, model.Read)))
	if err != nil {
		t.Fatalf("callTool('fake_auth_guarded') failed: %v", err)
	}
	if !strings.Contains(resMe, "user:user123") {
		t.Errorf("expected read result to contain 'user:user123', got: %s", resMe)
	}
}

func TestHarvestOps_ExposeNil(t *testing.T) {
	provider, err := mcp.HarvestOps(nil, nil, fakeModule{})
	if err != nil {
		t.Fatalf("HarvestOps: %v", err)
	}
	tools := provider.Tools()
	if len(tools) != 0 {
		t.Fatalf("expected 0 harvested tools, got %d", len(tools))
	}
}

func TestHarvestOps_ExposeUnknown(t *testing.T) {
	_, err := mcp.HarvestOps(nil, []mcp.ToolName{toolOf("fake", &fakeRecord{}, 'x')}, fakeModule{})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "is not registered by any module") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestHarvestOps_ExposeTwice(t *testing.T) {

	_, err := mcp.HarvestOps(nil, []mcp.ToolName{toolOf("fake", &fakeRecord{}, model.Read), toolOf("fake", &fakeRecord{}, model.Read)}, fakeModule{})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "listed twice in expose") {
		t.Fatalf("unexpected error: %v", err)
	}
}

type errWiringModule struct{}

func (errWiringModule) ModelName() string { return "errwire" }
func (m errWiringModule) Definition() model.ModuleDefinition {
	return model.ModuleDefinition{Name: m.ModelName(), Entities: []model.Definition{{Name: (&fakeRecord{}).ModelName(), Allow: model.Read | model.Create, Rules: model.Rules{Read: model.HaveNoBusinessRule, Create: model.AddMyBusinessRule}}}}
}
func (errWiringModule) MountOperations(reg crud.Registry) {
	reg.AddStoredEntity(crud.StoredEntity{Model: &fakeRecord{}, Store: &memStore{data: make(map[string]string)}, NewList: func() model.ModelSlice { return &fakeRecordList{} }, Rules: model.Rules{Read: model.HaveNoBusinessRule, Create: model.AddMyBusinessRule}})
}

func TestHarvestOps_CrudWiringError(t *testing.T) {
	_, err := mcp.HarvestOps(nil, nil, errWiringModule{})
	if !strings.Contains(err.Error(), "does not match definition") {
		t.Fatalf("expected mismatched definition error, got %v", err)
	}
}

func TestHarvest_ToolPerAllowedAction(t *testing.T) {
	opRead := toolOf("fake_two", &fakeRecord{}, model.Read)
	opCreate := toolOf("fake_two", &fakeRecord{}, model.Create)

	provider, err := mcp.HarvestOps(nil, []mcp.ToolName{opCreate, opRead}, fakeTwoOpsModule{})
	if err != nil {
		t.Fatalf("HarvestOps: %v", err)
	}
	tools := provider.Tools()
	if len(tools) != 2 {
		t.Fatalf("expected 2 tools, got %d", len(tools))
	}

	opUpdate := toolOf("fake_two", &fakeRecord{}, model.Update)
	_, err = mcp.HarvestOps(nil, []mcp.ToolName{opUpdate}, fakeTwoOpsModule{})
	if err == nil {
		t.Fatal("expected error exposing update tool, got nil")
	}
	if !strings.Contains(err.Error(), "is not registered by any module") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestHarvest_WriteToolPublishesAfterCommit(t *testing.T) {
	pub := &recorder{}
	opCreate := toolOf("fake_two", &fakeRecord{}, model.Create)
	provider, err := mcp.HarvestOps(pub, []mcp.ToolName{opCreate}, fakeTwoOpsModule{})
	if err != nil {
		t.Fatalf("HarvestOps: %v", err)
	}
	tools := provider.Tools()
	if len(tools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(tools))
	}

	tool := tools[0]
	res, err := tool.Execute(nil, mcp.Request{Params: mcp.CallToolParams{Arguments: `{"records":[{"id":"a","value":"x"}]}`}})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if res.IsError {
		t.Fatalf("unexpected error result: %s", res.Content)
	}

	if len(pub.got) != 1 {
		t.Fatalf("expected 1 event published, got %d", len(pub.got))
	}
	wantTopic := string(crud.Topic("fake_two", "fake_rec", model.Create))
	if string(pub.got[0].Topic) != wantTopic {
		t.Errorf("expected topic %q, got %q", wantTopic, pub.got[0].Topic)
	}
}

func TestHarvest_NilPublisherIsFine(t *testing.T) {
	opCreate := toolOf("fake_two", &fakeRecord{}, model.Create)
	provider, err := mcp.HarvestOps(nil, []mcp.ToolName{opCreate}, fakeTwoOpsModule{})
	if err != nil {
		t.Fatalf("HarvestOps: %v", err)
	}
	tools := provider.Tools()
	if len(tools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(tools))
	}

	tool := tools[0]
	res, err := tool.Execute(nil, mcp.Request{Params: mcp.CallToolParams{Arguments: `{"records":[{"id":"a","value":"x"}]}`}})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if res.IsError {
		t.Fatalf("unexpected error result: %s", res.Content)
	}
}
