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
	return []model.Field{{Name: "id", Type: model.Text(), DB: &model.FieldDB{PK: true}}}
}
func (r *mfaRecord) Pointers() []any                  { return []any{&r.ID} }
func (r *mfaRecord) Allow() model.Action { return model.Read | model.Delete }
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
func (m fakeMFAModule) Definition() model.ModuleDefinition {
	return model.ModuleDefinition{Name: m.ModelName(), Entities: []model.Definition{{Name: (&mfaRecord{}).ModelName(), Allow: model.Read | model.Delete, Rules: model.Rules{Read: model.HaveNoBusinessRule, Delete: model.AddMyBusinessRule}}}}
}
func (fakeMFAModule) MountOperations(reg crud.Registry) {
	reg.AddStoredEntity(crud.StoredEntity{
		Model: &mfaRecord{},
		Store: &memStore{data: make(map[string]string)},
		NewList: func() model.ModelSlice { return &mfaRecordList{} },
		Rules: model.Rules{Read: model.HaveNoBusinessRule, Delete: model.AddMyBusinessRule},
		MFA: model.Delete,
		Delete: func(c crud.CurrentUser, tx crud.Tx, rec model.Model) (crud.RuleAnswer, error) { return crud.ApplyAndPublish, nil },
	})
}

var _ crud.Module = fakeMFAModule{}

// M1: a module whose MountOperations registers delete with RequiresMFA;
// HarvestOps([]ToolName{"<module>.<entity>_d"}, m) returns an error containing `requires MFA`
func TestHarvestOps_ExposeMFAError(t *testing.T) {

	_, err := mcp.HarvestOps(nil, []mcp.ToolName{toolOf("fake_mfa", &mfaRecord{}, model.Delete)}, fakeMFAModule{})
	if err == nil {
		t.Fatal("expected error when exposing an MFA operation, got nil")
	}
	if !strings.Contains(err.Error(), "requires MFA") {
		t.Fatalf("error message %v does not contain 'requires MFA'", err)
	}
}

// M2: same module plus read without MFA; exposing only read succeeds and Tools() has exactly one tool
func TestHarvestOps_ExposeListOnly(t *testing.T) {

	provider, err := mcp.HarvestOps(nil, []mcp.ToolName{toolOf("fake_mfa", &mfaRecord{}, model.Read)}, fakeMFAModule{})
	if err != nil {
		t.Fatalf("HarvestOps: %v", err)
	}
	tools := provider.Tools()
	if len(tools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(tools))
	}
	want := string(toolOf("fake_mfa", &mfaRecord{}, model.Read))
	if tools[0].Name != want {
		t.Errorf("expected tool name %q, got %s", want, tools[0].Name)
	}
}

// M3: exposing only read while delete (MFA) is registered: no error, delete absent from Tools()
func TestHarvestOps_MFAOpNotInExposeSilent(t *testing.T) {

	provider, err := mcp.HarvestOps(nil, []mcp.ToolName{toolOf("fake_mfa", &mfaRecord{}, model.Read)}, fakeMFAModule{})
	if err != nil {
		t.Fatalf("HarvestOps: %v", err)
	}
	tools := provider.Tools()

	for _, tool := range tools {
		if tool.Name == string(toolOf("fake_mfa", &mfaRecord{}, model.Delete)) {
			t.Fatalf("delete (MFA) tool should not be exposed")
		}
	}
}
