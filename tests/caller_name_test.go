package mcp_test

import (
	"testing"

	"webtyp.com/mcp"
	"webtyp.com/model"
	"webtyp.com/router"
)

func TestCaller_SendsTheQualifiedToolName(t *testing.T) {
	provider, err := mcp.HarvestOps(nil, []mcp.ToolName{toolOf("fake", &fakeRecord{}, model.Read)}, fakeModule{})
	if err != nil {
		t.Fatalf("HarvestOps: %v", err)
	}
	srv, err := mcp.NewServer(mcp.Config{Name: "test-server", Version: "1.0.0", Authorize: mcp.AllowAll}, []mcp.ToolProvider{provider})
	if err != nil {
		t.Fatalf("mcp.NewServer: %v", err)
	}
	client := mcp.NewLocalClient(srv, "test-user")
	caller := mcp.NewCaller(client)

	var out fakeRecordList
	done := make(chan error, 1)
	caller.Call(router.Operation{Module: "fake", Entity: "fake_rec", Action: model.Read}, nil, &out, func(err error) { done <- err })
	if err := <-done; err != nil {
		t.Fatalf("caller.Call: %v", err)
	}
	if len(out) != 1 || out[0].Value != "echo:" {
		t.Errorf("out = %+v, want echo:", out)
	}
}
