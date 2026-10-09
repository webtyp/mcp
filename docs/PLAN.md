---
PLAN: "feat!: HarvestOps exposes only the operations the app names — closed by default"
TAG: v0.3.0
EXECUTOR: jules
REVIEWER: none
STATUS: review
SESSION: 17827328770497671939
PR: https://github.com/webtyp/mcp/pull/31
---

> This plan is dispatched via the CodeJob workflow. See skill: agents-workflow.

# Plan — `mcp`: an agent sees only the tools the application lists

## 0. Context (read first)

`mcp.HarvestOps(modules ...router.OperationModule) ToolProvider` (`harvest.go`) turns **every**
operation of every module into an MCP tool. Apps used MCP as their browser↔server channel too,
so in the clinic app (`veltylabs/mjosefa-cms`) one `POST /mcp` announces **115 tools** — create,
delete and admin operations included — to any agent that connects. That is noise for a small
model choosing a tool and attack surface for everything else.

Browser apps are moving to a plain HTTP binding (`webtyp.com/rpc`, separate plan). After that,
MCP is **only** for agents, and an agent must see only what the application deliberately lists.
The application declares that list in a file of its own (`config/mcp.go`, next to
`config/routes.go`), and passes it here.

This is a **breaking change** (decision of the owner: no backward compatibility, no deprecated
path). Consumers update when they bump.

## Design gate

### 1. Prior art
- **OpenAI / Anthropic tool use**: the caller sends the explicit list of tools for each request;
  nothing is implicit.
- **MCP clients (Claude Desktop, Cursor)**: per-server tool allow-lists configured by the user.
- **GraphQL persisted queries / Kubernetes RBAC "default deny"**: anything not listed is not
  reachable. Our zero value — an empty list — exposes nothing.
- **Spring Boot Actuator** `management.endpoints.web.exposure.include`: endpoints exist, and only
  the ones named in the exposure list are published.

### 2. Novice-name test
`mcp.HarvestOps(config.AgentTools(), booking, patients)` — "harvest from these modules the tools
the app lists for agents". `mcp.ToolNameOf(booking.ModelName, booking.OpListReservationsByStaff)` —
"the tool named booking.list_reservations_by_staff". `mcp.ToolName` — a qualified tool name.

### 3. Complexity ledger
```
Concepts the developer must learn   +1 (ToolName / the exposure list)
Files they must touch to do X        +1 (config/mcp.go in the app)
Lines at the call site               +1 argument
Ways to do the same thing            0 (HarvestOps keeps being the only way modules reach MCP)
```

### 4. Where it belongs
Here: which harvested operations become tools is the MCP transport's decision. The list itself
is the application's (composition root), never the module's — a module does not know which
agents exist.

### 5. What this deletes
The implicit "every operation is a tool" behaviour, and the two `panic` calls of `HarvestOps`
(empty `ModelName()`, duplicate name): they become returned errors.

## 1. Target API

```go
// ToolName is a qualified tool name: "<ModelName>.<operation>".
type ToolName string

// ToolNameOf builds the qualified name of one operation, from the module's constants:
// mcp.ToolNameOf(booking.ModelName, booking.OpListReservationsByStaff).
func ToolNameOf(module, op string) ToolName

// HarvestOps runs each module's MountOperations and returns a ToolProvider with ONLY the
// operations named in expose. An empty expose exposes nothing (closed by default).
// Errors (nothing is returned): a module with an empty ModelName(), the same qualified name
// registered twice, a name repeated in expose, or a name in expose that no module registered
// (a typo must fail at startup, not silently hide a tool).
func HarvestOps(expose []ToolName, modules ...router.OperationModule) (ToolProvider, error)
```

Error texts: unexported typed constants (`type harvestError string` + `Error()`):
`mcp: module with empty ModelName()`, `mcp: duplicate tool name <name>`,
`mcp: tool <name> listed twice in expose`, `mcp: exposed tool <name> is not registered by any module`.

## 2. Stages

| Stage | Files | Content |
|---|---|---|
| 1 | `harvest.go` | `ToolName`, `Tool`, new `HarvestOps`. Harvest everything into the internal registry as today (the `Route` methods keep writing into the `Tool` by index), then keep only the exposed ones, in the order of `expose`. Replace both `panic`s with the errors of §1. No `map`: linear scans |
| 2 | `tests/*.go` | Update the 5 test files that call `HarvestOps` (`harvest_test.go`, `harvest_plaintext_error_test.go`, `harvest_describe_test.go`, `local_client_test.go`, `caller_nilargs_test.go`) to pass an explicit expose list. Add the cases of §3 |
| 3 | `README.md`, `docs/ARCHITECTURE.md`, `docs/SKILL.md` | Every `HarvestOps` example takes the expose list; one paragraph: "MCP is the agents' surface; browsers call operations through `webtyp.com/rpc`; list in the app's `config/mcp.go` exactly what an agent may call" |

## 3. Tests (`tests/`, external package; `gotest`)

With a test module registering `a` (read) and `b` (update):
1. `HarvestOps(nil, mod)` → provider with **zero** tools; `tools/list` through a real `NewServer`
   returns an empty list.
2. `HarvestOps([]ToolName{Tool("testmod", "a")}, mod)` → exactly one tool, `testmod.a`, keeping
   the access, args and description declared by the module.
3. Calling `testmod.b` (not exposed) through the server → the "unknown tool" error the server
   already returns for an unregistered tool.
4. Errors: unknown name in expose; name listed twice; empty `ModelName()`; a module registering
   the same op twice — each returns an error and a nil provider, no panic.

## 4. Code rules (non-negotiable)
- Compiles to TinyGo WASM: `webtyp.com/fmt`; no `errors`, `strconv`, `strings` in library code.
- No `map` in library code. Repeated strings are constants.
- No exported symbol beyond §1 plus what already exists. Tests in `tests/`; never export for a test.

## 5. Acceptance criteria
- `gotest ./...` green.
- `grep -n "panic(" harvest.go` → empty.
- `grep -rn "HarvestOps(" --include=*.go . | grep -v "func HarvestOps"` → every call passes an expose list.

**Known consumers (not this plan's job):** `webtyp/view` tests, `webtyp/auth`, the veltylabs
modules' docs/tests and `veltylabs/mjosefa-cms` call `HarvestOps(modules...)`; each updates when it
bumps (mjosefa-cms in its integration plan, with `config/mcp.go`).

## Executor notes
- `mcp.Tool` couldn't be used as a function name because `mcp.Tool` is already a struct type. I used `mcp.ToolNameOf` instead.
