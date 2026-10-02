package mcp

import (
	"webtyp.com/context"
	"webtyp.com/json"
	"webtyp.com/router"
)

// MCPPath is the canonical route for the JSON-RPC endpoint.
const MCPPath = "/mcp"

// ModelName identifies the module (identity contract reused by APIModule).
func (s *Server) ModelName() string { return s.name }

// MountAPI publishes the MCP endpoint on the host's Router.
//
// The TRANSPORT route is public; the gate is per-tool, inside HandleMessage, where
// Tool.Access decides — and its zero value is AccessGuarded, so a tool that declares
// nothing is already closed. Two reasons it cannot be otherwise:
//
//   - A guarded route would need a Resource, and there is none: /mcp is an envelope
//     carrying calls to MANY tools, each with its own resource. Naming one would be a
//     lie, and leaving it empty makes the route deny everything, silently.
//   - It would make mcp.AccessPublic unreachable over HTTP and break the `initialize`
//     handshake, which by protocol precedes any identity.
//
// Declaring it explicitly is what keeps httpd's startup validation happy: an
// unannotated route falls into AccessGuarded with no Resource, and that is a
// contradiction the server rightly refuses to start with.
func (s *Server) MountAPI(r router.Router) {
	r.Post(MCPPath, func(ctx router.Context) {
		reqCtx := context.Background()
		reqCtx.Set(CtxKeyUserID, ctx.UserID())
		resp := s.HandleMessage(reqCtx, ctx.Body())
		ctx.SetHeader(headerContentType, mimeJSON)
		out, known, err := encodeResponse(resp)
		if !known {
			ctx.WriteStatus(500)
			ctx.Write([]byte(`{"error":"mcp: unknown response type"}`))
			return
		}
		if err != nil {
			ctx.WriteStatus(500)
			ctx.Write([]byte(`{"error":"mcp: encode failed"}`))
			return
		}
		ctx.Write([]byte(out))
	}).Public()
}

// encodeResponse writes what HandleMessage returned as the JSON the transport sends. known is
// false for a message that is neither a response nor an error.
func encodeResponse(resp JSONRPCMessage) (out string, known bool, err error) {
	switch m := resp.(type) {
	case *JSONRPCResponseStruct:
		return out, true, json.Encode(m, &out)
	case *JSONRPCError:
		return out, true, json.Encode(m, &out)
	}
	return "", false, nil
}
