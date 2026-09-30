package mcp_test

import (
	"testing"

	"webtyp.com/context"
	"webtyp.com/mcp"
	"webtyp.com/model"
)

// A client decides whether it may run a tool without asking the user from the MCP
// annotation readOnlyHint. Only a tool whose Action is exactly Read carries it; every other
// tool is announced without it, which MCP reads as "may modify".
func TestListTools_ReadOnlyHintOnlyForReadTools(t *testing.T) {
	srv, _ := mcp.NewServer(mcp.Config{Name: "test", Version: "1.0.0", Authorize: mcp.AllowAll}, nil)
	ok := func(ctx *context.Context, req mcp.Request) (*mcp.Result, error) { return mcp.Text("ok"), nil }
	for _, tool := range []mcp.Tool{
		{Name: "list_business_hours", Resource: "hours", Action: model.Read, Execute: ok},
		{Name: "book_appointment", Resource: "appointments", Action: model.Create, Execute: ok},
		{Name: "list_services", Access: model.AccessPublic, Execute: ok},
	} {
		if err := srv.AddTool(tool); err != nil {
			t.Fatalf("AddTool %s: %v", tool.Name, err)
		}
	}
	var ctx context.Context
	body := encodeResponse(srv.HandleMessage(&ctx, []byte(`{"jsonrpc":"2.0","id":"1","method":"tools/list","params":{}}`)))

	if !contains(body, `"name":"list_business_hours","inputSchema":{"type":"object","properties":{}},"annotations":{"readOnlyHint":true}`) {
		t.Errorf("the Read tool must carry readOnlyHint:\n%s", body)
	}
	if n := count(body, `"annotations"`); n != 1 {
		t.Errorf("only the Read tool may carry annotations, found %d:\n%s", n, body)
	}
}

func count(s, sub string) int {
	n := 0
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			n++
		}
	}
	return n
}
