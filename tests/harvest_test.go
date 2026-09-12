package mcp_test

import (
	"strings"
	"testing"

	"webtyp.com/mcp"
	"webtyp.com/model"
	"webtyp.com/router"
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

// fakeModule mimics a domain module: implements router.OperationModule, imports ONLY router+model.
type fakeModule struct{}

func (fakeModule) ModelName() string { return "fake" }
func (fakeModule) MountOperations(r router.OperationRegistry) {
	r.Operation("do_thing", func(ctx router.Context) {
		var in fakeArgs
		if err := ctx.Decode(&in); err != nil {
			ctx.WriteStatus(500)
			return
		}
		_ = ctx.Encode(&fakeArgs{Value: "echo:" + in.Value})
	}).Requires("fake_resource", model.Read).Accepts(&fakeArgs{})
}

var _ router.OperationModule = fakeModule{}

// fakeMeModuleA and fakeMeModuleB mimic two independent domain modules that happen to both
// register the "me" operation — the real-world collision (e.g. authority.Module vs an app-local
// module both exposing "me").
type fakeMeModuleA struct{}

func (fakeMeModuleA) ModelName() string { return "fake_a" }
func (fakeMeModuleA) MountOperations(r router.OperationRegistry) {
	r.Operation("me", func(ctx router.Context) {}).Public()
}

var _ router.OperationModule = fakeMeModuleA{}

type fakeMeModuleB struct{}

func (fakeMeModuleB) ModelName() string { return "fake_b" }
func (fakeMeModuleB) MountOperations(r router.OperationRegistry) {
	r.Operation("me", func(ctx router.Context) {}).Public()
}

var _ router.OperationModule = fakeMeModuleB{}

// fakeTwoOpsModule registers two distinct operation names — must harvest cleanly with no panic.
type fakeTwoOpsModule struct{}

func (fakeTwoOpsModule) ModelName() string { return "fake_two" }
func (fakeTwoOpsModule) MountOperations(r router.OperationRegistry) {
	r.Operation("op_a", func(ctx router.Context) {}).Public()
	r.Operation("op_b", func(ctx router.Context) {}).Public()
}

var _ router.OperationModule = fakeTwoOpsModule{}

func TestHarvestOps_DuplicateNameAcrossModulesPanics(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic on duplicate tool name, got none")
		}
		msg, ok := r.(string)
		if !ok || !strings.Contains(msg, `duplicate tool name "me"`) {
			t.Fatalf("panic message %v does not name the duplicated tool", r)
		}
	}()
	mcp.HarvestOps(fakeMeModuleA{}, fakeMeModuleB{})
}

func TestHarvestOps_DistinctNamesNoPanic(t *testing.T) {
	provider := mcp.HarvestOps(fakeTwoOpsModule{})
	tools := provider.Tools()
	if len(tools) != 2 {
		t.Fatalf("expected 2 harvested tools, got %d", len(tools))
	}
}

func TestHarvestOps_SameModuleInstanceTwicePanics(t *testing.T) {
	fm := fakeModule{}
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic when the same module instance is harvested twice, got none")
		}
		msg, ok := r.(string)
		if !ok || !strings.Contains(msg, `duplicate tool name "do_thing"`) {
			t.Fatalf("panic message %v does not name the duplicated tool", r)
		}
	}()
	mcp.HarvestOps(fm, fm)
}

func TestHarvestOps_ModuleReachesMCP(t *testing.T) {
	provider := mcp.HarvestOps(fakeModule{})
	tools := provider.Tools()
	if len(tools) != 1 {
		t.Fatalf("expected 1 harvested tool, got %d", len(tools))
	}
	tool := tools[0]
	if tool.Name != "do_thing" || tool.Resource != "fake_resource" || tool.Action != model.Read {
		t.Fatalf("harvested tool metadata mismatch: %+v", tool)
	}
	if tool.Args == nil {
		t.Fatal("expected Accepts(...) to populate Tool.Args")
	}

	res, err := tool.Execute(nil, mcp.Request{Params: mcp.CallToolParams{Arguments: `{"value":"x"}`}})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if res.IsError {
		t.Fatalf("unexpected IsError, content: %s", res.Content)
	}
	if res.Content != `{"value":"echo:x"}` {
		t.Errorf("unexpected content: %s", res.Content)
	}
}
