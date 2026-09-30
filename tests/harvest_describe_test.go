package mcp_test

import (
	"testing"

	"webtyp.com/context"
	"webtyp.com/mcp"
	"webtyp.com/model"
	"webtyp.com/router"
)

// describedModule is a domain module that says what its operation does.
type describedModule struct{}

func (describedModule) ModelName() string { return "business_calendar" }
func (describedModule) MountOperations(r router.OperationRegistry) {
	r.Operation("list_business_hours", func(ctx router.Context) { _ = ctx.Encode(&fakeArgs{Value: "ok"}) }).
		Requires("business_hours", model.Read).
		Describe("Horario de atención del consultorio para cada día de la semana.")
}

// An AI agent chooses tools by what tools/list says they do: the operation's Describe text
// is the tool's description.
func TestHarvestOps_DescribeBecomesToolDescription(t *testing.T) {
	srv, err := mcp.NewServer(mcp.Config{Name: "test", Version: "1.0.0", Authorize: mcp.AllowAll},
		[]mcp.ToolProvider{mcp.HarvestOps(describedModule{})})
	if err != nil {
		t.Fatal(err)
	}
	var ctx context.Context
	body := encodeResponse(srv.HandleMessage(&ctx, []byte(`{"jsonrpc":"2.0","id":"1","method":"tools/list","params":{}}`)))
	want := `"name":"business_calendar.list_business_hours","description":"Horario de atención del consultorio para cada día de la semana."`
	if !contains(body, want) {
		t.Errorf("tools/list lacks the described tool:\n%s", body)
	}
}
