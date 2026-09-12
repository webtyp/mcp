---
module: webtyp.com/mcp
version: see go.mod
protocol: MCP (Model Context Protocol) over JSON-RPC 2.0
---

# SKILL: webtyp/mcp

Lean Go MCP server + client library. Protocol-only, WASM-safe, no HTTP ownership.

## What it does

- Handles JSON-RPC 2.0 MCP messages: `initialize`, `ping`, `tools/list`, `tools/call`
- Mandatory RBAC on every tool call via `Authorize` function
- Optional SSE streaming via injected `SSEPublisher`
- Dual-use: server (backend) + client (WASM frontend via `webtyp/fetch`)

## Core types

```go
// Server
mcp.NewServer(Config{Name, Version, Authorize, SSE}, []ToolProvider) (*Server, error)
srv.HandleMessage(ctx, []byte) JSONRPCMessage   // main entry point
srv.AddTool(Tool)                                // register at runtime

// Tool definition
mcp.Tool{
    Name, Description string
    InputSchema string   // JSON schema from ormc: new(Args).Schema()
    Resource    string   // RBAC resource e.g. "catalog"
    Action      byte     // 'c','r','u','d'
    Public      bool     // explicit: accessible without identity
    Execute     func(ctx, Request) (*Result, error)
}

// Tool handler pattern
func handle(ctx *context.Context, req mcp.Request) (*mcp.Result, error) {
    var args MyArgs
    if err := req.Bind(&args); err != nil { return nil, err }
    return mcp.Text("ok"), nil
}

// Result helpers
mcp.Text("string")         // *Result with text content
mcp.JSON(&myFielder{})     // *Result with JSON content
mcp.GetText(result)        // extract text string

// Auth
mcp.AllowAll               // helper: always returns true (dev/tests)

// Client (WASM-safe, uses webtyp/fetch)
c := mcp.NewClient("http://host", "")   // authToken: "" = open/unauthenticated
caller := mcp.NewCaller(c) // recommended adapter for views (implements router.Caller)
caller.Call("search", &args, func(data []byte, err error){})
caller.Dispatch("search", &args)

// Context keys
mcp.CtxKeyUserID      // set before HandleMessage: identity resolved by host
mcp.CtxKeySessionID   // set by server on initialize
```

## ToolProvider pattern

```go
type MyProvider struct{ db *postgres.DB }

func (p *MyProvider) Tools() []mcp.Tool {
    return []mcp.Tool{{
        Name: "my_tool", Resource: "items", Action: 'r',
        InputSchema: new(MyArgs).Schema(),
        Execute: p.handle,
    }}
}

srv, _ := mcp.NewServer(config, []mcp.ToolProvider{&MyProvider{db: db}})
```

## HTTP integration (consumer owns routing)

```go
http.HandleFunc("/mcp", func(w http.ResponseWriter, r *http.Request) {
    body, _ := io.ReadAll(r.Body)
    ctx := context.New()
    ctx.Set(mcp.CtxKeyUserID, "user-123") // identity resolved by host middleware
    resp := srv.HandleMessage(&ctx, body)
    // encode resp (fmt.Fielder) to JSON and write
})
```

## Code generation (ormc)

```bash
go install webtyp.com/orm/cmd/ormc@latest
# annotate struct with // ormc:formonly
# run: go generate ./...
```

## Constraints

- `Config.Authorize` must not be nil — use `mcp.AllowAll` for open access
- Every `Tool` must have `Resource` and `Action` set
- No stdlib `encoding/json` — uses `webtyp/json` (TinyGo compatible)
- SSE is optional; when nil no streaming occurs
- WASM build excludes server-only files via `//go:build !wasm`

## Files

| File | Role |
|------|------|
| `harvest.go` | `HarvestOps(modules...)` — builds a `ToolProvider` from `router.OperationModule`s; panics at wiring time on a duplicate tool name |
| `server.go` | `Server`, `NewServer`, handlers (init/ping/list/call) |
| `request_handler.go` | `HandleMessage` dispatch, JSON extraction, context keys |
| `mcp_auth.go` | `Authorize` type + `AllowAll` helper |
| `tools.go` | `Tool`, `Request`, `Request.Bind`, `Text`, `ToolProvider` |
| `client.go` | `Client` for WASM frontend use |
| `caller.go` | `NewCaller`: `router.Caller` adapter for views |
| `publish_sse.go` | `SSEPublisher` interface |
| `model.go` | JSON-RPC 2.0 type definitions (ormc annotated) |
| `model_orm.go` | Generated: `Schema()`, `Pointers()`, `Validate()` |
| `utils.go` | `JSON`, `GetText`, `ParseResult`, response builders |
| `constants.go` | HTTP header constants, content type constants |
| `types.go` | MCP method constants, error codes, `JSONRPCMessage` interface |
