package mcp_test

import (
	"testing"

	"webtyp.com/mcp"
	"webtyp.com/router"
	"webtyp.com/router/mock"
)

func TestServerImplementsAPIModule(t *testing.T) {
	var _ router.APIModule = (*mcp.Server)(nil)
}

type mockContext struct {
	router.Context
	body        []byte
	headers     map[string]string
	status      int
	writtenData []byte
	userID      string
}

func (m *mockContext) RawQuery() string { return "" }
func (m *mockContext) Body() []byte { return m.body }
func (m *mockContext) GetHeader(key string) string {
	if m.headers == nil {
		return ""
	}
	return m.headers[key]
}
func (m *mockContext) SetHeader(key, value string) {
	if m.headers == nil {
		m.headers = make(map[string]string)
	}
	m.headers[key] = value
}
func (m *mockContext) WriteStatus(code int) { m.status = code }
func (m *mockContext) Write(data []byte) (int, error) {
	m.writtenData = append(m.writtenData, data...)
	return len(data), nil
}
func (m *mockContext) UserID() string { return m.userID }

func TestMountAPI(t *testing.T) {
	server, _ := mcp.NewServer(mcp.Config{
		Name:      "test-server",
		Version:   "1.0.0",
		Authorize: mcp.AllowAll,
	}, nil)

	// The canonical mock, not a hand-rolled one: the local mockRouter used to return a
	// nil Route from Post — a lie no real router tells, and it hid the fact that MountAPI
	// never declared its access level.
	mr := &mock.Router{}
	server.MountAPI(mr)

	routes := mr.Routes()
	if len(routes) != 1 || routes[0].Path != mcp.MCPPath {
		t.Fatalf("expected POST %s to be registered, got %+v", mcp.MCPPath, routes)
	}

	// Test a simple initialize request
	ctx := &mockContext{
		body: []byte(`{"jsonrpc":"2.0","id":"1","method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test-client","version":"1.0.0"}}}`),
	}

	mr.Invoke("POST", mcp.MCPPath, ctx)

	if ctx.headers["Content-Type"] != "application/json" {
		t.Errorf("expected Content-Type application/json, got %s", ctx.headers["Content-Type"])
	}

	if !contains(string(ctx.writtenData), `"result"`) {
		t.Errorf("expected result in response, got %s", string(ctx.writtenData))
	}
	if !contains(string(ctx.writtenData), `"protocolVersion"`) {
		t.Errorf("expected protocolVersion in response, got %s", string(ctx.writtenData))
	}
}

func TestModelName(t *testing.T) {
	server, _ := mcp.NewServer(mcp.Config{
		Name:      "my-mcp-server",
		Authorize: mcp.AllowAll,
	}, nil)

	if server.ModelName() != "my-mcp-server" {
		t.Errorf("expected ModelName my-mcp-server, got %s", server.ModelName())
	}
}
