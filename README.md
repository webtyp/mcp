# webtyp/mcp
<img src="docs/img/badges.svg">

Lean Go implementation of the [Model Context Protocol](https://modelcontextprotocol.io/) (MCP) over JSON-RPC 2.0. Protocol-only, WASM-safe, minimal public API.

## Installation

```bash
go get webtyp.com/mcp
```

### ormc (code generation)

Tools use `ormc` for automatic `Schema()`, `Pointers()`, and `Validate()` generation:

```bash
go install webtyp.com/orm/cmd/ormc@latest
```

## Quickstart

### 1. Define tool arguments with validation (using ormc)

```go
// Tool arguments are plain structs with validation tags
// ormc generates Schema(), Pointers(), Validate() automatically
type SearchArgs struct {
    Query string `input:"required,min=1,max=255"`
    Limit int64  `input:"min=1,max=100"`
}
```

### 2. Write the handler

```go
func handleSearch(ctx *context.Context, req mcp.Request) (*mcp.Result, error) {
    var args SearchArgs
    if err := req.Bind(&args); err != nil {
        return nil, err // server wraps as JSON-RPC error
    }
    return mcp.Text("found 3 results"), nil
}
```

### 3. Register and serve

```go
srv, err := mcp.NewServer(mcp.Config{
    Name:      "my-server",
    Version:   "1.0.0",
    Authorize: mcp.AllowAll, // or your RBAC function
}, nil)
if err != nil {
    log.Fatal(err)
}

srv.AddTool(mcp.Tool{
    Name:        "search",
    Description: "Search items by query",
    InputSchema: new(SearchArgs).Schema(),
    Resource:    "items",
    Action:      'r',
    Execute:     handleSearch,
})

// 1. Mount on a router (recommended)
import "webtyp.com/router"

r := router.New()
srv.MountAPI(r)
r.ListenAndServe(":8080")

// 2. Or manual handling if needed
http.HandleFunc(mcp.MCPPath, func(w http.ResponseWriter, r *http.Request) {
    body, _ := io.ReadAll(r.Body)
    ctx := context.New()
    ctx.Set(mcp.CtxKeyUserID, "user-123") // identity resolved by host
    resp := srv.HandleMessage(&ctx, body)
    // encode resp as JSON and write to w
})
```

---

## Calling tools (Client)

Views should depend on `router.Caller` and use `mcp.NewCaller` to invoke tools. This adapter handles the MCP JSON-RPC envelope, allowing the view to pass logical tool names and receive unwrapped result bytes.

```go
import (
    "webtyp.com/mcp"
    "webtyp.com/router"
)

// 1. Composition root: wire the client and adapter
client := mcp.NewClient("https://api.myserver.com", "my-bearer-token")
caller := mcp.NewCaller(client)

// 2. View: depend on router.Caller only
func MyView(c router.Caller) {
    c.Call("search", &SearchArgs{Query: "mcp"}, func(result []byte, err error) {
        if err != nil {
            // handles transport, protocol and tool errors
            return
        }
        // result contains unwrapped tool content bytes
    })
}
```

---

## Authorization (RBAC)

`mcp` delegates identity resolution to the host and only handles per-tool RBAC.

```go
// The Authorize function signature expected by mcp
type Authorize func(userID, resource, action string) bool

// Example implementation (e.g. using webtyp/user)
srv, _ := mcp.NewServer(mcp.Config{
    Authorize: user.Can,
}, providers)
```

Authorization is required — `NewServer` returns error if `Config.Authorize` is nil. Use `mcp.AllowAll` for open access during development.

---

## SSE (Streamable HTTP)

Optional streaming via `SSEPublisher` interface. The consumer creates the SSE server and injects it:

```go
import "webtyp.com/sse"

sseServer := sse.New()

srv, err := mcp.NewServer(mcp.Config{
    Name:      "my-server",
    Version:   "1.0.0",
    Authorize: mcp.AllowAll,
    SSE:       sseServer, // *sse.SSEServer satisfies mcp.SSEPublisher
}, providers)
```

---

## WASM / Browser

The protocol core compiles with TinyGo. Server-only files (`//go:build !wasm`) are excluded automatically.

In browser mode, call the handler directly — no HTTP server needed:

```go
srv, err := mcp.NewServer(config, providers)
response := srv.HandleMessage(&ctx, message)
```

---

## API Reference

| Symbol | Description |
|--------|-------------|
| `HarvestOps(modules...)` | Build a `ToolProvider` from `router.OperationModule`s — panics at wiring time on a duplicate tool name |
| `NewServer(config, providers)` | Create MCP server — returns `(*Server, error)` |
| `NewClient(baseURL, authToken)` | Create MCP client |
| `NewCaller(client)` | Adapt `*Client` to `router.Caller` (recommended for views) |
| `Server.AddTool(tool)` | Register a single tool |
| `Server.HandleMessage(ctx, msg)` | Process JSON-RPC message (WASM-safe) |
| `Server.MountAPI(router)` | Mount MCP endpoint on a `router.Router` |
| `Tool{Name, Description, InputSchema, Resource, Action, Public, Execute}` | Tool definition |
| `AllowAll` | Helper for open access (dev/tests) |
| `Request.Bind(target)` | Decode + validate arguments |
| `Text(s)` | Create text result |
| `JSON(data)` | Create JSON result |
| `GetText(result)` | Extract text from result |
| `Config` | Server configuration: `Name`, `Version`, `Authorize`, `SSE` |
| `CtxKeySessionID` | Context key for session ID |
| `CtxKeyUserID` | Context key for authenticated user ID |

---

## Documentation

| Document | Description |
|----------|-------------|
| [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) | What & Why — abstract design, constraints, file map, key interfaces |
| [docs/SKILL.md](docs/SKILL.md) | LLM-friendly condensed reference — types, patterns, constraints |
| [docs/WHY_ARQ.md](docs/WHY_ARQ.md) | Architecture decisions, pros/cons, key trade-offs |
| [docs/PLAN.md](docs/PLAN.md) | Pending: migrate to `FieldRaw` for inline JSON fields |
| [docs/diagrams/architecture.md](docs/diagrams/architecture.md) | Architecture overview diagram |
| [docs/diagrams/request_flow.md](docs/diagrams/request_flow.md) | Request dispatch + auth flow diagram |

---
This project is a minimal Go implementation of the Model Context Protocol, inspired by and with some internal models adapted from [github.com/mark3labs/mcp-go](https://github.com/mark3labs/mcp-go).
