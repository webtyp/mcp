package mcp_test

import (
	"strings"
	"testing"

	"webtyp.com/context"
	"webtyp.com/mcp"
	"webtyp.com/model"
	"webtyp.com/router"
)

// A local client is the HTTP client without the network: the same requests reach the same
// server and come back the same way, so a router.Caller over it decodes ops exactly as over HTTP.
func TestLocalClient_SameAnswersAsHTTP(t *testing.T) {
	opRead := toolOf("fake", &fakeRecord{}, model.Read)
	provider, err := mcp.HarvestOps(nil, []mcp.ToolName{opRead}, fakeModule{})
	if err != nil {
		t.Fatalf("HarvestOps: %v", err)
	}
	srv, err := mcp.NewServer(mcp.Config{Name: "test-server", Version: "1.0.0", Authorize: mcp.AllowAll},
		[]mcp.ToolProvider{provider})
	if err != nil {
		t.Fatalf("mcp.NewServer: %v", err)
	}
	client := mcp.NewLocalClient(srv, "test-user")

	var list []byte
	client.Call(context.Background(), string(mcp.MethodToolsList), nil, func(body []byte, err error) {
		if err != nil {
			t.Fatalf("tools/list: %v", err)
		}
		list = body
	})
	wantTool := string(opRead)
	if !strings.Contains(string(list), wantTool) {
		t.Fatalf("tools/list = %s, want %s", list, wantTool)
	}

	var out fakeRecordList
	done := make(chan error, 1)
	mcp.NewCaller(client).Call(router.Operation{Module: "fake", Entity: "fake_rec", Action: model.Read}, nil, &out, func(err error) { done <- err })
	if err := <-done; err != nil {
		t.Fatalf("caller.Call: %v", err)
	}
	if len(out) != 1 || out[0].Value != "echo:" {
		t.Errorf("out = %+v, want echo:", out)
	}
}
