package mcp_test

import (
	"strings"
	"testing"

	"webtyp.com/crud"
	"webtyp.com/mcp"
	"webtyp.com/model"
)

type mfaRecord struct {
	ID string
}

func (r *mfaRecord) ModelName() string { return "mfa_rec" }
func (r *mfaRecord) IsNil() bool       { return r == nil }
func (r *mfaRecord) Schema() []model.Field {
	return []model.Field{{Name: "id", Type: model.Text()}}
}
func (r *mfaRecord) Pointers() []any                  { return []any{&r.ID} }
func (r *mfaRecord) EncodeFields(w model.FieldWriter) { w.String("id", r.ID) }
func (r *mfaRecord) DecodeFields(d model.FieldReader) { r.ID, _ = d.String("id") }

type mfaRecordList []*mfaRecord

func (s *mfaRecordList) Len() int                         { return len(*s) }
func (s *mfaRecordList) At(i int) model.Fielder           { return (*s)[i] }
func (s *mfaRecordList) Append() model.Fielder            { v := &mfaRecord{}; *s = append(*s, v); return v }
func (s *mfaRecordList) IsNil() bool                      { return s == nil }
func (s *mfaRecordList) EncodeFields(_ model.FieldWriter) {}
func (s *mfaRecordList) DecodeFields(_ model.FieldReader) {}

// fakeMFAModule registers an MFA op ("delete") and a non-MFA op ("read").
type fakeMFAModule struct{}

func (fakeMFAModule) ModelName() string { return "fake_mfa" }
func (fakeMFAModule) MountOperations(reg crud.Registry) {
	reg.Delete(&mfaRecord{}, func(c crud.Caller, ids []string) error {
		return nil
	}).RequiresMFA()
	reg.Read(&mfaRecord{}, nil, func(c crud.Caller, args crud.ReadArgs, filter model.Model) (model.ModelSlice, error) {
		out := mfaRecordList{}
		return &out, nil
	})
}

var _ crud.Module = fakeMFAModule{}

// M1: a module whose MountOperations registers delete with RequiresMFA;
// HarvestOps([]ToolName{"<module>.<entity>_d"}, m) returns an error containing `requires MFA`
func TestHarvestOps_ExposeMFAError(t *testing.T) {
	opDelete := crud.Op(&mfaRecord{}, model.Delete)
	_, err := mcp.HarvestOps([]mcp.ToolName{mcp.ToolNameOf("fake_mfa", opDelete)}, fakeMFAModule{})
	if err == nil {
		t.Fatal("expected error when exposing an MFA operation, got nil")
	}
	if !strings.Contains(err.Error(), "requires MFA") {
		t.Fatalf("error message %v does not contain 'requires MFA'", err)
	}
}

// M2: same module plus read without MFA; exposing only read succeeds and Tools() has exactly one tool
func TestHarvestOps_ExposeListOnly(t *testing.T) {
	opRead := crud.Op(&mfaRecord{}, model.Read)
	provider, err := mcp.HarvestOps([]mcp.ToolName{mcp.ToolNameOf("fake_mfa", opRead)}, fakeMFAModule{})
	if err != nil {
		t.Fatalf("HarvestOps: %v", err)
	}
	tools := provider.Tools()
	if len(tools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(tools))
	}
	want := "fake_mfa." + opRead
	if tools[0].Name != want {
		t.Errorf("expected tool name %q, got %s", want, tools[0].Name)
	}
}

// M3: exposing only read while delete (MFA) is registered: no error, delete absent from Tools()
func TestHarvestOps_MFAOpNotInExposeSilent(t *testing.T) {
	opRead := crud.Op(&mfaRecord{}, model.Read)
	provider, err := mcp.HarvestOps([]mcp.ToolName{mcp.ToolNameOf("fake_mfa", opRead)}, fakeMFAModule{})
	if err != nil {
		t.Fatalf("HarvestOps: %v", err)
	}
	tools := provider.Tools()
	opDelete := crud.Op(&mfaRecord{}, model.Delete)
	for _, tool := range tools {
		if tool.Name == "fake_mfa."+opDelete {
			t.Fatalf("delete (MFA) tool should not be exposed")
		}
	}
}
