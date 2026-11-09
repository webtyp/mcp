package mcp_test

import (
	"strings"
	"testing"

	"webtyp.com/context"
	"webtyp.com/crud"
	"webtyp.com/mcp"
	"webtyp.com/model"
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
		{Name: "id", Type: model.Text()},
		{Name: "value", Type: model.Text()},
	}
}
func (r *fakeRecord) Pointers() []any { return []any{&r.ID, &r.Value} }
func (r *fakeRecord) EncodeFields(w model.FieldWriter) {
	w.String("id", r.ID)
	w.String("value", r.Value)
}
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
	return readOnlyDefinition(m.ModelName(), &fakeRecord{})
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
	return readOnlyDefinition(m.ModelName(), &fakeRecord{})
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
	return readOnlyDefinition(m.ModelName(), &fakeRecord{})
}
func (fakeMeModuleB) MountOperations(reg crud.Registry) {
	reg.AddReadOnlyEntity(&fakeRecord{}, nil, func(c crud.CurrentUser, args crud.ReadArgs, filter model.Model) (model.ModelSlice, error) {
		out := fakeRecordList{}
		return &out, nil
	})
}

var _ crud.Module = fakeMeModuleB{}

// fakeStored is a table the module writes through crud: Create only.
type fakeStored struct {
	ID   string
	Name string
}

func (r *fakeStored) ModelName() string   { return "fake_stored" }
func (r *fakeStored) Allow() model.Action { return model.Create }
func (r *fakeStored) IsNil() bool         { return r == nil }
func (r *fakeStored) Schema() []model.Field {
	return []model.Field{
		{Name: "id", Type: model.Text(), DB: &model.FieldDB{PK: true}},
		{Name: "name", Type: model.Text()},
	}
}
func (r *fakeStored) Pointers() []any { return []any{&r.ID, &r.Name} }
func (r *fakeStored) EncodeFields(w model.FieldWriter) {
	w.String("id", r.ID)
	w.String("name", r.Name)
}
func (r *fakeStored) DecodeFields(d model.FieldReader) {
	r.ID, _ = d.String("id")
	r.Name, _ = d.String("name")
}

type fakeStoredList []*fakeStored

func (s *fakeStoredList) Len() int                         { return len(*s) }
func (s *fakeStoredList) At(i int) model.Fielder           { return (*s)[i] }
func (s *fakeStoredList) Append() model.Fielder            { v := &fakeStored{}; *s = append(*s, v); return v }
func (s *fakeStoredList) IsNil() bool                      { return s == nil }
func (s *fakeStoredList) EncodeFields(_ model.FieldWriter) {}
func (s *fakeStoredList) DecodeFields(_ model.FieldReader) {}

// fakeTwoOpsModule offers a read (its own rule) on fake_rec and a create on the table fake_stored.
type fakeTwoOpsModule struct{ store *memStore }

func (fakeTwoOpsModule) ModelName() string { return "fake_two" }
func (m fakeTwoOpsModule) Definition() model.ModuleDefinition {
	def := readOnlyDefinition(m.ModelName(), &fakeRecord{})
	def.Entities = append(def.Entities, model.Definition{Name: "fake_stored", Allow: model.Create, Rules: model.Rules{Create: model.HaveNoBusinessRule}})
	return def
}
func (m fakeTwoOpsModule) MountOperations(reg crud.Registry) {
	reg.AddReadOnlyEntity(&fakeRecord{}, nil, func(c crud.CurrentUser, args crud.ReadArgs, filter model.Model) (model.ModelSlice, error) {
		out := fakeRecordList{}
		return &out, nil
	})
	reg.AddStoredEntity(crud.StoredEntity{
		Model:   &fakeStored{},
		NewList: func() model.ModelSlice { return &fakeStoredList{} },
		Store:   m.store,
		Rules:   model.Rules{Create: model.HaveNoBusinessRule},
	})
}

var _ crud.Module = fakeTwoOpsModule{}

type fakeUnnamedModule struct{}

func (fakeUnnamedModule) ModelName() string                  { return "" }
func (fakeUnnamedModule) Definition() model.ModuleDefinition { return model.ModuleDefinition{} }
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
	provider, err := mcp.HarvestOps(nil, []mcp.ToolName{toolOf("fake_two", &fakeRecord{}, model.Read), toolOf("fake_two", &fakeStored{}, model.Create)}, fakeTwoOpsModule{store: &memStore{}})
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
	opRead := toolOf("fake", &fakeRecord{}, model.Read)
	provider, err := mcp.HarvestOps(nil, []mcp.ToolName{opRead}, fakeModule{})
	if err != nil {
		t.Fatalf("HarvestOps: %v", err)
	}
	tools := provider.Tools()
	if len(tools) != 1 {
		t.Fatalf("expected 1 harvested tool, got %d", len(tools))
	}
	tool := tools[0]
	if tool.Name != string(opRead) || tool.Resource != "fake_rec" || tool.Action != model.Read {
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

type fakeAuthAndGuardedModule struct{ store *memStore }

func (fakeAuthAndGuardedModule) ModelName() string { return "fake_auth_guarded" }
func (m fakeAuthAndGuardedModule) Definition() model.ModuleDefinition {
	def := readOnlyDefinition(m.ModelName(), &fakeRecord{})
	def.Entities = append(def.Entities, model.Definition{Name: "fake_stored", Allow: model.Create, Rules: model.Rules{Create: model.HaveNoBusinessRule}})
	return def
}
func (m fakeAuthAndGuardedModule) MountOperations(reg crud.Registry) {
	reg.AddReadOnlyEntity(&fakeRecord{}, nil, func(c crud.CurrentUser, args crud.ReadArgs, filter model.Model) (model.ModelSlice, error) {
		out := fakeRecordList{&fakeRecord{Value: "user:" + c.UserID}}
		return &out, nil
	})
	reg.AddStoredEntity(crud.StoredEntity{
		Model:   &fakeStored{},
		NewList: func() model.ModelSlice { return &fakeStoredList{} },
		Store:   m.store,
		Rules:   model.Rules{Create: model.HaveNoBusinessRule},
	})
}

var _ crud.Module = fakeAuthAndGuardedModule{}

func TestHarvestOps_AuthenticatedAndGuardedOps(t *testing.T) {
	opRead := toolOf("fake_auth_guarded", &fakeRecord{}, model.Read)
	opCreate := toolOf("fake_auth_guarded", &fakeStored{}, model.Create)
	provider, err := mcp.HarvestOps(nil, []mcp.ToolName{opRead, opCreate}, fakeAuthAndGuardedModule{store: &memStore{}})
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

	resMe, err := callTool(srv, ctx, string(opRead))
	if err != nil {
		t.Fatalf("callTool(%s) failed: %v", opRead, err)
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
	_, err := mcp.HarvestOps(nil, []mcp.ToolName{mcp.ToolName("fake.fake_rec.unknown")}, fakeModule{})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "is not registered by any module") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestHarvestOps_ExposeTwice(t *testing.T) {
	opRead := toolOf("fake", &fakeRecord{}, model.Read)
	_, err := mcp.HarvestOps(nil, []mcp.ToolName{opRead, opRead}, fakeModule{})
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
	return readOnlyDefinition(m.ModelName(), &fakeRecord{})
}
func (errWiringModule) MountOperations(reg crud.Registry) {
	reg.AddReadOnlyEntity(&fakeRecord{}, nil, nil)
}

func TestHarvestOps_CrudWiringError(t *testing.T) {
	_, err := mcp.HarvestOps(nil, nil, errWiringModule{})
	if err != crud.ErrNilFunction {
		t.Fatalf("expected crud.ErrNilFunction, got %v", err)
	}
}

func TestHarvest_ToolPerAllowedAction(t *testing.T) {
	create := toolOf("fake_two", &fakeStored{}, model.Create)
	read := toolOf("fake_two", &fakeRecord{}, model.Read)
	provider, err := mcp.HarvestOps(nil, []mcp.ToolName{create, read}, fakeTwoOpsModule{store: &memStore{}})
	if err != nil {
		t.Fatalf("HarvestOps: %v", err)
	}
	if got := len(provider.Tools()); got != 2 {
		t.Fatalf("expected 2 tools, got %d", got)
	}
	update := toolOf("fake_two", &fakeStored{}, model.Update)
	_, err = mcp.HarvestOps(nil, []mcp.ToolName{update}, fakeTwoOpsModule{store: &memStore{}})
	if err == nil || !strings.Contains(err.Error(), "is not registered by any module") {
		t.Fatalf("exposing a tool the module does not offer must fail, got %v", err)
	}
}

func TestHarvest_WriteToolPublishesAfterCommit(t *testing.T) {
	pub := &recorder{}
	store := &memStore{}
	create := toolOf("fake_two", &fakeStored{}, model.Create)
	provider, err := mcp.HarvestOps(pub, []mcp.ToolName{create}, fakeTwoOpsModule{store: store})
	if err != nil {
		t.Fatalf("HarvestOps: %v", err)
	}
	res, err := provider.Tools()[0].Execute(nil, mcp.Request{Params: mcp.CallToolParams{Arguments: `{"records":[{"id":"a","name":"x"}]}`}})
	if err != nil || res == nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(store.created) != 1 {
		t.Fatalf("expected 1 stored record, got %d", len(store.created))
	}
	if len(pub.got) != 1 || pub.got[0].Topic != crud.Topic("fake_two", "fake_stored", model.Create) {
		t.Fatalf("expected one %s event, got %+v", crud.Topic("fake_two", "fake_stored", model.Create), pub.got)
	}
}

func TestHarvest_NilPublisherIsFine(t *testing.T) {
	store := &memStore{}
	create := toolOf("fake_two", &fakeStored{}, model.Create)
	provider, err := mcp.HarvestOps(nil, []mcp.ToolName{create}, fakeTwoOpsModule{store: store})
	if err != nil {
		t.Fatalf("HarvestOps: %v", err)
	}
	if _, err := provider.Tools()[0].Execute(nil, mcp.Request{Params: mcp.CallToolParams{Arguments: `{"records":[{"id":"a","name":"x"}]}`}}); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(store.created) != 1 {
		t.Fatalf("expected 1 stored record, got %d", len(store.created))
	}
}
