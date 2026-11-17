package mcp_test

import (
	"testing"

	"webtyp.com/mcp"
	"webtyp.com/router/mock"
)

func TestTools_MountsTheMCPEndpoint(t *testing.T) {
	r := &mock.Router{}
	if err := (mcp.Tools{}).MountTools(r, "app", mcp.AllowAll, nil, nil); err != nil {
		t.Fatalf("Tools with nothing allowed still mounts the endpoint, got %v", err)
	}
	routes := r.Routes()
	if len(routes) != 1 || routes[0].Path != mcp.MCPPath {
		t.Fatalf("expected POST %s to be registered, got %+v", mcp.MCPPath, routes)
	}
}

func TestTools_NeedsAnAuthorizer(t *testing.T) {
	if err := (mcp.Tools{}).MountTools(&mock.Router{}, "app", nil, nil, nil); err == nil {
		t.Fatal("a nil Authorize must be an error: the server would decide nothing")
	}
}
