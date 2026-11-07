package mcp_test

import (
	"testing"

	"webtyp.com/context"
	"webtyp.com/crud"
	"webtyp.com/mcp"
	"webtyp.com/model"
)

type businessHours struct {
	ID string
}

func (b *businessHours) ModelName() string { return "business_hours" }
func (b *businessHours) IsNil() bool       { return b == nil }
func (b *businessHours) Schema() []model.Field {
	return []model.Field{{Name: "id", Type: model.Text()}}
}
func (b *businessHours) Pointers() []any                  { return []any{&b.ID} }
func (b *businessHours) EncodeFields(w model.FieldWriter) { w.String("id", b.ID) }
func (b *businessHours) DecodeFields(d model.FieldReader) { b.ID, _ = d.String("id") }

type businessHoursList []*businessHours

func (s *businessHoursList) Len() int                         { return len(*s) }
func (s *businessHoursList) At(i int) model.Fielder           { return (*s)[i] }
func (s *businessHoursList) Append() model.Fielder            { v := &businessHours{}; *s = append(*s, v); return v }
func (s *businessHoursList) IsNil() bool                      { return s == nil }
func (s *businessHoursList) EncodeFields(_ model.FieldWriter) {}
func (s *businessHoursList) DecodeFields(_ model.FieldReader) {}

// describedModule is a domain module that says what its operation does.
type describedModule struct{}

func (describedModule) ModelName() string { return "business_calendar" }
func (describedModule) MountOperations(reg crud.Registry) {
	reg.Read(&businessHours{}, nil, func(c crud.Caller, args crud.ReadArgs, filter model.Model) (model.ModelSlice, error) {
		out := businessHoursList{}
		return &out, nil
	}).Describe("Horario de atención del consultorio para cada día de la semana.")
}

// An AI agent chooses tools by what tools/list says they do: the operation's Describe text
// is the tool's description.
func TestHarvestOps_DescribeBecomesToolDescription(t *testing.T) {
	opRead := crud.Op(&businessHours{}, model.Read)
	provider, err := mcp.HarvestOps([]mcp.ToolName{mcp.ToolNameOf("business_calendar", opRead)}, describedModule{})
	if err != nil {
		t.Fatalf("HarvestOps: %v", err)
	}
	srv, err := mcp.NewServer(mcp.Config{Name: "test", Version: "1.0.0", Authorize: mcp.AllowAll},
		[]mcp.ToolProvider{provider})
	if err != nil {
		t.Fatal(err)
	}
	var ctx context.Context
	body := encodeResponse(srv.HandleMessage(&ctx, []byte(`{"jsonrpc":"2.0","id":"1","method":"tools/list","params":{}}`)))
	want := `"name":"business_calendar.` + opRead + `","description":"Horario de atención del consultorio para cada día de la semana."`
	if !contains(body, want) {
		t.Errorf("tools/list lacks the described tool:\n%s", body)
	}
}
