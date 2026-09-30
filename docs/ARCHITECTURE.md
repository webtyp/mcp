# Architecture: webtyp/mcp

## Purpose

Lean Go implementation of MCP (Model Context Protocol) over JSON-RPC 2.0. Protocol-only, WASM-safe, no HTTP ownership. The consumer owns HTTP routing and injects optional SSE streaming.

## Design Principles

- **No HTTP ownership** — `mcp` never owns HTTP routing. Only exposes `HandleMessage()`.
- **RBAC Required** — `NewServer` returns error if `Config.Authorize == nil`.
- **Mandatory RBAC** — Every tool call passes through `Authorize()` before execution.
- **SSE via DI** — Optional `SSEPublisher` injected by consumer for streaming notifications.
- **WASM-safe** — Protocol core compiles with TinyGo. Server-only files use `//go:build !wasm`.

## Architecture

```
Consumer (webtyp/app)
    │
    ├─ owns HTTP routing
    ├─ handles authentication (middleware)
    ├─ optionally creates *sse.SSEServer → SSEPublisher
    │
    └─ NewServer(Config, []ToolProvider) → (*Server, error)
              │
                ├─ MountAPI(router) → POST /mcp
                │
              ▼
        ┌─────────────────────────────────────┐
        │  Server                             │
        │  HandleMessage(ctx, []byte) → msg   │
        │                                     │
        │  Auth flow:                         │
        │  1. UserID from ctx                 │
        │  2. Authorize(userID, res, act)     │
        │                                     │
        │  SSE: Publish(data, channel)        │
        └─────────────────────────────────────┘
              │
              ▼
          MCP Client (LLM)

View (Browser/WASM)
    │
    └─ NewCaller(*Client) → router.Caller
              │
              ├─ translates logical op to "tools/call"
              ├─ unwraps MCP Result.Content
              └─ propagates errors (Transport/RPC/Tool)
```

## Version Negotiation

When a client sends an `initialize` request, the server negotiates the protocol version. If the requested version is supported, the server responds with that version. If the requested version is not supported, the server responds with its latest supported version (`2025-11-25`). The client then decides whether to proceed or disconnect.

Supported versions:
- `2025-11-25` (Latest)
- `2025-06-18`
- `2025-03-26`
- `2024-11-05`

## Auth + RBAC Flow

![Auth + RBAC Flow](img/auth_flow.svg)

```
HandleMessage(ctx, raw)
    │
    ├─ userID := ctx.Value(CtxKeyUserID)
    │
    └─ tools/call:
        ├─ lookup tool by name
        ├─ if !tool.Public && userID == "": Forbidden
        ├─ authorize(userID, tool.Resource, tool.Action)
        │   └─ false? → -32001 Forbidden
        └─ tool.Execute(ctx, req)
```

## SSE Flow (Streamable HTTP)

```
Consumer creates *sse.SSEServer (satisfies SSEPublisher)
    │
    └─ Config.SSE = sseServer
          │
          ▼
    Server.AddTool(tool)
        └─ SSE.Publish(notifications/tools/list_changed, "mcp")

    Server.handleNotification(ctx, notification)
        └─ SSE.Publish(notification, "mcp")
```

SSE is optional — when `Config.SSE` is nil, no streaming occurs.

## Files

| File | Responsibility |
|------|---------------|
| `server.go` | `Server`, `Config`, `NewServer`, `HandleMessage` dispatch to handlers, `AddTool`, tool call/list/init/ping handlers |
| `request_handler.go` | `HandleMessage` JSON-RPC dispatch, `ExtractJSONValue`, `extractJSONString`, context keys |
| `mount.go` | `router.APIModule` implementation (`ModelName`, `MountAPI`) |
| `mcp_auth.go` | `Authorize` type + `AllowAll` helper |
| `server_sse.go` | `SSEPublisher` interface (build `!wasm`) |
| `tools.go` | `Tool` (Args is typed model source of inputSchema), `Request`, `Request.Bind`, `Text`, `FilterFunc`, `ToolProvider` |
| `tool_schema.go` | `inputSchemaOf`, `jsonSchemaType`, `EmptyInputSchema` — generates valid JSON Schema from `model.Fielder` |
| `types.go` | JSON-RPC 2.0 types: `JSONRPCMessage`, `JSONRPCRequest`, `JSONRPCNotification`, `JSONRPCResponseStruct`, `JSONRPCError`, `Result`, error codes, MCP methods |
| `model.go` | ORM model definitions: `rpcRequest`, `rpcResponse`, `initializeParams`, `callToolParams`, etc. |
| `model_orm.go` | Generated ORM code: `Schema()`, `Pointers()`, `Validate()` for all models |
| `client.go` | `Client`: MCP client (`Call`, `Dispatch`) using `webtyp/fetch` |
| `caller.go` | `NewCaller`: `router.Caller` adapter for views |
| `provider.go` | `Loggable` interface |
| `constants.go` | HTTP header constants, content type constants |
| `errors.go` | Error variables and `NewError` helper |
| `utils.go` | Result helpers: `JSON`, `GetText`, `ParseResult`, response builders |
| `logger.go` | `Logger` interface, `DefaultLogger` (stdlib wrapper) |

## Tool Arguments & Schema Generation

`Tool.Args` is a typed `model.Fielder` (ormc-generated model) that serves as the **single source of truth** for tool argument schemas. At runtime, `tools/list` calls `inputSchemaOf(tool.Args)` to generate a valid JSON Schema object from the model's metadata (`Schema() []model.Field`). This centralizes schema generation in `mcp` and prevents invalid schemas from escaping to MCP clients (e.g., Claude Code).

**Schema precedence:** Only `Args` is used. If `Args` is nil, schema defaults to `EmptyInputSchema` (`{"type":"object","properties":{}}`). Never a raw string or null.

## Key Interfaces

```go
// From webtyp.com/router (matches mcp.Authorize)
type Authorize func(userID, resource, action string) bool

type SSEPublisher interface {  // build !wasm
    Publish(data []byte, channel string)
}

type ToolProvider interface {
    Tools() []Tool
}
```

## Constraints

- No global state
- No external HTTP dependencies in protocol core
- WASM build excludes `server_sse.go` via build tag
- All tools require `Resource` and `Action` fields (RBAC mandatory)
- `NewServer` rejects nil `Authorize`

## Access: one declaration, guarded by default

A tool declares `Access` (`model.Access`), and the **zero value is `AccessGuarded`**.

| State | Requires | Use it for |
|---|---|---|
| `AccessGuarded` (zero) | identity **and** `Authorize(userID, Resource, Action)` | everything that touches data. A tool that declares nothing lands here. |
| `AccessAuthenticated` | identity only, **no resource check** | operations on the CALLER themselves (`me`): authentication already *is* the check. |
| `AccessPublic` | nothing | the rare tool reachable with no identity. Write it on purpose. |

`AddTool` **fails at startup**, never silently at runtime, when the declaration contradicts
itself:

- **guarded with no `Resource`** — it would authorize against `""` and deny every call: a
  tool that looks protected and is in fact unreachable.
- **non-guarded with a `Resource`** — a resource nobody checks reads as protection and gives
  none.

`AccessAuthenticated` exists because without it a `me` tool had to invent a resource
(`"profile"`) that the app never declared: a hole in this contract turning into policy
inside a library.

`Resource` and `Action` are `model.Resource` / `model.Action` — this package **types** the
vocabulary and never declares it. Actions are a closed CRUD set; `Request.Action` stays a
`byte` only because it is handed to the `Validate(action byte)` of the ormc-generated
models. That is the boundary, and the refactor does not cross it.

### Read-only tools are announced as such

An AI agent calling these tools must know which ones change data, so it can ask the person
before running them (`webtyp/agent` does). MCP has a standard field for that in `tools/list`:
the tool annotation `readOnlyHint`. This server sets it only when a tool's `Action` is exactly
`model.Read`:

```json
{"name":"list_business_hours","inputSchema":{...},"annotations":{"readOnlyHint":true}}
```

Every other tool is listed without annotations, which MCP reads as "may modify". That includes
public and authenticated tools, which carry no `Action`: closed by default, so a client asks
before running a tool nobody declared harmless.
