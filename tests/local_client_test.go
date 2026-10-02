package mcp_test

import (
	"strings"
	"testing"

	"webtyp.com/context"
	"webtyp.com/mcp"
)

// A local client is the HTTP client without the network: the same requests reach the same
// server and come back the same way, so a router.Caller over it decodes ops exactly as over HTTP.
func TestLocalClient_SameAnswersAsHTTP(t *testing.T) {
	srv, err := mcp.NewServer(mcp.Config{Name: "test-server", Version: "1.0.0", Authorize: mcp.AllowAll},
		[]mcp.ToolProvider{mcp.HarvestOps(fakeModule{})})
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
	if !strings.Contains(string(list), "fake.do_thing") {
		t.Fatalf("tools/list = %s, want fake.do_thing", list)
	}

	var out fakeArgs
	done := make(chan error, 1)
	mcp.NewCaller(client).Call("fake.do_thing", nil, &out, func(err error) { done <- err })
	if err := <-done; err != nil {
		t.Fatalf("caller.Call: %v", err)
	}
	if out.Value != "echo:" {
		t.Errorf("out.Value = %q, want %q", out.Value, "echo:")
	}
}
