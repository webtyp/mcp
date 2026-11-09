package mcp_test

import (
	"testing"

	"webtyp.com/mcp"
	"webtyp.com/model"
	"webtyp.com/router"
)

func TestCaller_SendsTheQualifiedToolName(t *testing.T) {
	opRead := toolOf("fake", &fakeRecord{}, model.Read)
	provider, err := mcp.HarvestOps(nil, []mcp.ToolName{opRead}, fakeModule{})
	if err != nil {
		t.Fatalf("HarvestOps: %v", err)
	}
	srv, err := mcp.NewServer(mcp.Config{Name: "test-server", Version: "1.0.0", Authorize: mcp.AllowAll}, []mcp.ToolProvider{provider})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	var out fakeRecordList
	done := make(chan error, 1)
	op := router.Operation{Module: "fake", Entity: "fake_rec", Action: model.Read}
	mcp.NewCaller(mcp.NewLocalClient(srv, "u1")).Call(op, nil, &out, func(err error) { done <- err })
	if err := <-done; err != nil {
		t.Fatalf("the tool %s must be found by the name the caller sends: %v", opRead, err)
	}
}
