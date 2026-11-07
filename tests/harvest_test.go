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
func (r *fakeRecord) Pointers() []any                  { return []any{&r.ID, &r.Value} }
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
func (fakeModule) MountOperations(reg crud.Registry) {
	reg.Read(&fakeRecord{}, nil, func(c crud.Caller, args crud.ReadArgs, filter model.Model) (model.ModelSlice, error) {
		out := fakeRecordList{&fakeRecord{Value: "echo:" + args.Search}}
		return &out, nil
	}).Describe("Do thing")
}

var _ crud.Module = fakeModule{}

type fakeMeModuleA struct{}

func (fakeMeModuleA) ModelName() string { return "fake_a" }
func (fakeMeModuleA) MountOperations(reg crud.Registry) {
	reg.Read(&fakeRecord{}, nil, func(c crud.Caller, args crud.ReadArgs, filter model.Model) (model.ModelSlice, error) {
		out := fakeRecordList{}
		return &out, nil
	})
}

var _ crud.Module = fakeMeModuleA{}

type fakeMeModuleB struct{}

func (fakeMeModuleB) ModelName() string { return "fake_b" }
func (fakeMeModuleB) MountOperations(reg crud.Registry) {
	reg.Read(&fakeRecord{}, nil, func(c crud.Caller, args crud.ReadArgs, filter model.Model) (model.ModelSlice, error) {
		out := fakeRecordList{}
		return &out, nil
	})
}

var _ crud.Module = fakeMeModuleB{}

type fakeTwoOpsModule struct{}

func (fakeTwoOpsModule) ModelName() string { return "fake_two" }
func (fakeTwoOpsModule) MountOperations(reg crud.Registry) {
	reg.Read(&fakeRecord{}, nil, func(c crud.Caller, args crud.ReadArgs, filter model.Model) (model.ModelSlice, error) {
		out := fakeRecordList{}
		return &out, nil
	})
	reg.Create(func() model.ModelSlice { return &fakeRecordList{} }, func(c crud.Caller, list model.ModelSlice) error {
		return nil
	})
}

var _ crud.Module = fakeTwoOpsModule{}

type fakeUnnamedModule struct{}

func (fakeUnnamedModule) ModelName() string { return "" }
func (fakeUnnamedModule) MountOperations(reg crud.Registry) {
	reg.Read(&fakeRecord{}, nil, func(c crud.Caller, args crud.ReadArgs, filter model.Model) (model.ModelSlice, error) {
		out := fakeRecordList{}
		return &out, nil
	})
}

var _ crud.Module = fakeUnnamedModule{}

func TestHarvestOps_EmptyModelNameError(t *testing.T) {
	_, err := mcp.HarvestOps(nil, fakeUnnamedModule{})
	if err == nil {
		t.Fatal("expected error when a module's ModelName() is empty, got nil")
	}
	if !strings.Contains(err.Error(), "empty ModelName()") {
		t.Fatalf("error message %v does not explain the empty ModelName()", err)
	}
}

func TestHarvestOps_SameBareNameAcrossModulesNoLongerCollides(t *testing.T) {
	opName := crud.Op(&fakeRecord{}, model.Read)
	provider, err := mcp.HarvestOps([]mcp.ToolName{mcp.ToolNameOf("fake_a", opName), mcp.ToolNameOf("fake_b", opName)}, fakeMeModuleA{}, fakeMeModuleB{})
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
	wantA := "fake_a." + opName
	wantB := "fake_b." + opName
	if !names[wantA] || !names[wantB] {
		t.Fatalf("expected qualified names %q and %q, got %v", wantA, wantB, names)
	}
}

func TestHarvestOps_DistinctNamesNoPanic(t *testing.T) {
	opRead := crud.Op(&fakeRecord{}, model.Read)
	opCreate := crud.Op(&fakeRecord{}, model.Create)
	provider, err := mcp.HarvestOps([]mcp.ToolName{mcp.ToolNameOf("fake_two", opRead), mcp.ToolNameOf("fake_two", opCreate)}, fakeTwoOpsModule{})
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
	_, err := mcp.HarvestOps(nil, fm, fm)
	if err == nil {
		t.Fatal("expected error when the same module instance is harvested twice, got nil")
	}
	want := "duplicate tool name fake." + crud.Op(&fakeRecord{}, model.Read)
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("error message %v does not contain %q", err, want)
	}
}

func TestHarvestOps_ModuleReachesMCP(t *testing.T) {
	opRead := crud.Op(&fakeRecord{}, model.Read)
	provider, err := mcp.HarvestOps([]mcp.ToolName{mcp.ToolNameOf("fake", opRead)}, fakeModule{})
	if err != nil {
		t.Fatalf("HarvestOps: %v", err)
	}
	tools := provider.Tools()
	if len(tools) != 1 {
		t.Fatalf("expected 1 harvested tool, got %d", len(tools))
	}
	tool := tools[0]
	if tool.Name != "fake."+opRead || tool.Resource != "fake_rec" || tool.Action != model.Read {
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
func (fakeAuthAndGuardedModule) MountOperations(reg crud.Registry) {
	reg.Read(&fakeRecord{}, nil, func(c crud.Caller, args crud.ReadArgs, filter model.Model) (model.ModelSlice, error) {
		out := fakeRecordList{&fakeRecord{Value: "user:" + c.UserID}}
		return &out, nil
	})
	reg.Create(func() model.ModelSlice { return &fakeRecordList{} }, func(c crud.Caller, list model.ModelSlice) error {
		return nil
	})
}

var _ crud.Module = fakeAuthAndGuardedModule{}

func TestHarvestOps_AuthenticatedAndGuardedOps(t *testing.T) {
	opRead := crud.Op(&fakeRecord{}, model.Read)
	opCreate := crud.Op(&fakeRecord{}, model.Create)
	provider, err := mcp.HarvestOps([]mcp.ToolName{mcp.ToolNameOf("fake_auth_guarded", opRead), mcp.ToolNameOf("fake_auth_guarded", opCreate)}, fakeAuthAndGuardedModule{})
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

	resMe, err := callTool(srv, ctx, "fake_auth_guarded."+opRead)
	if err != nil {
		t.Fatalf("callTool('fake_auth_guarded.%s') failed: %v", opRead, err)
	}
	if !strings.Contains(resMe, "user:user123") {
		t.Errorf("expected read result to contain 'user:user123', got: %s", resMe)
	}
}

func TestHarvestOps_ExposeNil(t *testing.T) {
	provider, err := mcp.HarvestOps(nil, fakeModule{})
	if err != nil {
		t.Fatalf("HarvestOps: %v", err)
	}
	tools := provider.Tools()
	if len(tools) != 0 {
		t.Fatalf("expected 0 harvested tools, got %d", len(tools))
	}
}

func TestHarvestOps_ExposeUnknown(t *testing.T) {
	_, err := mcp.HarvestOps([]mcp.ToolName{mcp.ToolNameOf("fake", "unknown")}, fakeModule{})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "is not registered by any module") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestHarvestOps_ExposeTwice(t *testing.T) {
	opRead := crud.Op(&fakeRecord{}, model.Read)
	_, err := mcp.HarvestOps([]mcp.ToolName{mcp.ToolNameOf("fake", opRead), mcp.ToolNameOf("fake", opRead)}, fakeModule{})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "listed twice in expose") {
		t.Fatalf("unexpected error: %v", err)
	}
}

type errWiringModule struct{}

func (errWiringModule) ModelName() string { return "errwire" }
func (errWiringModule) MountOperations(reg crud.Registry) {
	reg.Create(nil, nil)
}

func TestHarvestOps_CrudWiringError(t *testing.T) {
	_, err := mcp.HarvestOps(nil, errWiringModule{})
	if err != crud.ErrNilFunction {
		t.Fatalf("expected crud.ErrNilFunction, got %v", err)
	}
}
