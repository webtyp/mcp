package mcp_test

import (
	"strings"
	"testing"

	"webtyp.com/context"
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

// fakeUnnamedModule mimics a module that forgot (or was written wrong) to
// return a real ModelName() — the case HarvestOps must refuse, not harvest
// under an empty prefix.
type fakeUnnamedModule struct{}

func (fakeUnnamedModule) ModelName() string { return "" }
func (fakeUnnamedModule) MountOperations(r router.OperationRegistry) {
	r.Operation("op", func(ctx router.Context) {}).Public()
}

var _ router.OperationModule = fakeUnnamedModule{}

func TestHarvestOps_EmptyModelNameError(t *testing.T) {
	_, err := mcp.HarvestOps(nil, fakeUnnamedModule{})
	if err == nil {
		t.Fatal("expected error when a module's ModelName() is empty, got nil")
	}
	if !strings.Contains(err.Error(), "empty ModelName()") {
		t.Fatalf("error message %v does not explain the empty ModelName()", err)
	}
}

// TestHarvestOps_SameBareNameAcrossModulesNoLongerCollides is the acid test
// of module-qualified names: fakeMeModuleA and fakeMeModuleB both register
// the bare name "me" — before qualification this panicked ("duplicate tool
// name"); now the two are "fake_a.me" and "fake_b.me", genuinely distinct,
// and both must be reachable.
func TestHarvestOps_SameBareNameAcrossModulesNoLongerCollides(t *testing.T) {
	provider, err := mcp.HarvestOps([]mcp.ToolName{mcp.ToolNameOf("fake_a", "me"), mcp.ToolNameOf("fake_b", "me")}, fakeMeModuleA{}, fakeMeModuleB{})
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
	if !names["fake_a.me"] || !names["fake_b.me"] {
		t.Fatalf("expected qualified names %q and %q, got %v", "fake_a.me", "fake_b.me", names)
	}
}

func TestHarvestOps_DistinctNamesNoPanic(t *testing.T) {
	provider, err := mcp.HarvestOps([]mcp.ToolName{mcp.ToolNameOf("fake_two", "op_a"), mcp.ToolNameOf("fake_two", "op_b")}, fakeTwoOpsModule{})
	if err != nil {
		t.Fatalf("HarvestOps: %v", err)
	}
	tools := provider.Tools()
	if len(tools) != 2 {
		t.Fatalf("expected 2 harvested tools, got %d", len(tools))
	}
}

// TestHarvestOps_SameModuleInstanceTwiceError: the SAME module (same
// ModelName()) harvested twice produces the SAME qualified name twice —
// qualification narrows the panic to genuine duplicates, it does not remove
// it.
func TestHarvestOps_SameModuleInstanceTwiceError(t *testing.T) {
	fm := fakeModule{}
	_, err := mcp.HarvestOps(nil, fm, fm)
	if err == nil {
		t.Fatal("expected error when the same module instance is harvested twice, got nil")
	}
	if !strings.Contains(err.Error(), `duplicate tool name fake.do_thing`) {
		t.Fatalf("error message %v does not name the duplicated qualified tool", err)
	}
}

func TestHarvestOps_ModuleReachesMCP(t *testing.T) {
	provider, err := mcp.HarvestOps([]mcp.ToolName{mcp.ToolNameOf("fake", "do_thing")}, fakeModule{})
	if err != nil {
		t.Fatalf("HarvestOps: %v", err)
	}
	tools := provider.Tools()
	if len(tools) != 1 {
		t.Fatalf("expected 1 harvested tool, got %d", len(tools))
	}
	tool := tools[0]
	if tool.Name != "fake.do_thing" || tool.Resource != "fake_resource" || tool.Action != model.Read {
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

type fakeAuthAndGuardedModule struct{}

func (fakeAuthAndGuardedModule) ModelName() string { return "fake_auth_guarded" }
func (fakeAuthAndGuardedModule) MountOperations(r router.OperationRegistry) {
	r.Operation("me", func(ctx router.Context) {
		_ = ctx.Encode(&fakeArgs{Value: "user:" + ctx.UserID()})
	}).Authenticated()

	r.Operation("read_thing", func(ctx router.Context) {
		_ = ctx.Encode(&fakeArgs{Value: "thing_read"})
	}).Requires("thing", model.Read)
}

var _ router.OperationModule = fakeAuthAndGuardedModule{}

func TestHarvestOps_AuthenticatedAndGuardedOps(t *testing.T) {
	provider, err := mcp.HarvestOps([]mcp.ToolName{mcp.ToolNameOf("fake_auth_guarded", "me"), mcp.ToolNameOf("fake_auth_guarded", "read_thing")}, fakeAuthAndGuardedModule{})
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

	resMe, err := callTool(srv, ctx, "fake_auth_guarded.me")
	if err != nil {
		t.Fatalf("callTool('fake_auth_guarded.me') failed: %v", err)
	}
	if !strings.Contains(resMe, "user:user123") {
		t.Errorf("expected me result to contain 'user:user123', got: %s", resMe)
	}

	resRead, err := callTool(srv, ctx, "fake_auth_guarded.read_thing")
	if err != nil {
		t.Fatalf("callTool('fake_auth_guarded.read_thing') failed: %v", err)
	}
	if !strings.Contains(resRead, "thing_read") {
		t.Errorf("expected read_thing result to contain 'thing_read', got: %s", resRead)
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
	_, err := mcp.HarvestOps([]mcp.ToolName{mcp.ToolNameOf("fake", "do_thing"), mcp.ToolNameOf("fake", "do_thing")}, fakeModule{})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "listed twice in expose") {
		t.Fatalf("unexpected error: %v", err)
	}
}
