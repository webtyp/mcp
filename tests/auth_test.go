package mcp_test

import (
	"webtyp.com/model"
	"testing"

	"webtyp.com/context"
	"webtyp.com/mcp"
)

func TestNewServer_NilAuthorize_ReturnsError(t *testing.T) {
	_, err := mcp.NewServer(mcp.Config{Name: "test", Version: "1.0.0", Authorize: nil}, nil)
	if err == nil {
		t.Fatal("expected error for nil Authorize")
	}
}

func TestHandleToolCall_PublicTool_NoAuth_Passes(t *testing.T) {
	srv, _ := mcp.NewServer(mcp.Config{Name: "test", Version: "1.0.0", Authorize: mcp.AllowAll}, nil)
	srv.AddTool(mcp.Tool{
		Name: "public-tool",
		// Un tool público NO declara recurso ni acción: un recurso o acción que nadie
		// comprueba parece protección y no la da. AddTool lo rechaza al arrancar.
		Access: model.AccessPublic,
		Execute: func(ctx *context.Context, req mcp.Request) (*mcp.Result, error) {
			return mcp.Text("ok"), nil
		},
	})

	var ctx context.Context
	req := []byte(`{"jsonrpc":"2.0","id":"1","method":"tools/call","params":{"name":"public-tool","arguments":{}}}`)
	resp := srv.HandleMessage(&ctx, req)

	respStr := encodeResponse(resp)
	if !contains(respStr, "ok") {
		t.Fatalf("expected success, got %s", respStr)
	}
}

func TestHandleToolCall_PrivateTool_NoAuth_Rejected(t *testing.T) {
	srv, _ := mcp.NewServer(mcp.Config{Name: "test", Version: "1.0.0", Authorize: mcp.AllowAll}, nil)
	srv.AddTool(mcp.Tool{
		Name:     "private-tool",
		Resource: "res",
		Action:   model.Read,
		// Sin Access declarado: el zero value es AccessGuarded — identidad Y permiso.
		Execute: func(ctx *context.Context, req mcp.Request) (*mcp.Result, error) {
			return mcp.Text("ok"), nil
		},
	})

	var ctx context.Context
	req := []byte(`{"jsonrpc":"2.0","id":"1","method":"tools/call","params":{"name":"private-tool","arguments":{}}}`)
	resp := srv.HandleMessage(&ctx, req)

	respStr := encodeResponse(resp)
	if !contains(respStr, "authentication required") {
		t.Fatalf("expected authentication required error, got %s", respStr)
	}
}
