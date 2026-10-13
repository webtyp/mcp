# Agent Guide — `webtyp/mcp`

Constraints for agents working on this library. Read this before any change.

## What this library is

A lean Model Context Protocol (MCP) server and client over JSON-RPC 2.0.
Protocol-only, **WASM-safe**: it compiles for native Go and for WebAssembly
(TinyGo, Cloudflare Workers), so binary size matters.

## Rules

- **No standard library** for strings, numbers or errors: use `webtyp.com/fmt`
  (not `strings`, `strconv`, `errors`, `fmt`).
- **No `map` in code that compiles to WASM.** Use slices with a linear scan.
- **Typed over `any`.** Tool arguments are `model.Fielder` (ormc-generated),
  never `map[string]any`.
- **Closed by default.** `HarvestOps` exposes only the operations the app
  names in `expose`. A tool's zero `Access` is `model.AccessGuarded`. Never add
  a path that exposes a tool nobody named.
- **The access vocabulary is the router's.** `Tool.Access`, `Resource` and
  `Action` mirror `router.RouteInfo`. Do not invent a parallel one.
- Every repeated string is a named constant (see `constants.go`).

## Testing

- Tests live in `tests/` (external package, public API only). Never export a
  symbol only so a test can reach it.
- Run `gotest` (from `webtyp.com/devflow/cmd/gotest`), never `go test`. It runs
  vet, race, the stdlib suite and the WASM suite.
