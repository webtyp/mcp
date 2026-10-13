package mcp_test

import (
	"strings"
	"testing"

	"webtyp.com/mcp"
	"webtyp.com/model"
	"webtyp.com/router"
)

// fakeMFAModule registers an MFA op ("wipe") and a non-MFA op ("list").
type fakeMFAModule struct{}

func (fakeMFAModule) ModelName() string { return "fake_mfa" }
func (fakeMFAModule) MountOperations(r router.OperationRegistry) {
	r.Operation("wipe", func(ctx router.Context) {}).Requires("res", model.Delete).RequiresMFA()
	r.Operation("list", func(ctx router.Context) {}).Requires("res", model.Read)
}

var _ router.OperationModule = fakeMFAModule{}

// M1: a module whose MountOperations registers op "wipe" with .Requires(res, model.Delete).RequiresMFA();
// HarvestOps([]ToolName{"<module>.wipe"}, m) returns an error containing `requires MFA`
func TestHarvestOps_ExposeMFAError(t *testing.T) {
	_, err := mcp.HarvestOps([]mcp.ToolName{mcp.ToolNameOf("fake_mfa", "wipe")}, fakeMFAModule{})
	if err == nil {
		t.Fatal("expected error when exposing an MFA operation, got nil")
	}
	if !strings.Contains(err.Error(), "requires MFA") {
		t.Fatalf("error message %v does not contain 'requires MFA'", err)
	}
}

// M2: same module plus op "list" without MFA; exposing only list succeeds and Tools() has exactly one tool
func TestHarvestOps_ExposeListOnly(t *testing.T) {
	provider, err := mcp.HarvestOps([]mcp.ToolName{mcp.ToolNameOf("fake_mfa", "list")}, fakeMFAModule{})
	if err != nil {
		t.Fatalf("HarvestOps: %v", err)
	}
	tools := provider.Tools()
	if len(tools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(tools))
	}
	if tools[0].Name != "fake_mfa.list" {
		t.Errorf("expected tool name 'fake_mfa.list', got %s", tools[0].Name)
	}
}

// M3: exposing only list while wipe (MFA) is registered: no error, wipe absent from Tools()
func TestHarvestOps_MFAOpNotInExposeSilent(t *testing.T) {
	provider, err := mcp.HarvestOps([]mcp.ToolName{mcp.ToolNameOf("fake_mfa", "list")}, fakeMFAModule{})
	if err != nil {
		t.Fatalf("HarvestOps: %v", err)
	}
	tools := provider.Tools()
	for _, tool := range tools {
		if tool.Name == "fake_mfa.wipe" {
			t.Fatalf("wipe tool should not be exposed")
		}
	}
}
