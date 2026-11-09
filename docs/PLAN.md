---
PLAN: "refactor!: tools are named by router.Operation (<module>.<entity>.<verb>); HarvestOps takes the publisher crud announces on"
EXECUTOR: jules
REVIEWER: none
---

> Read AGENTS.md at the repo root before any change.
> **Finish by yourself:** when the delivery check passes, commit after every stage, push and open the pull request. Never ask whether to
> proceed or finalize — nobody answers, and a session that waits ends without your work.
> Never run `git reset`, `git restore`, `git checkout -- <file>` or `git stash` on your own work.
> The other `docs/PLAN_*.md` files of this repo are separate, parked topics: do not execute or edit them.

## Goal

A tool today is named `"<module>.<entity>_c|_r|_u|_d"`, built by `crud.Op` and glued with `mcp.ToolNameOf(module, op string)`. After this plan the name comes from the typed operation:
`router.Operation{Module: "booking", Entity: "reservation", Action: model.Read}` → tool `booking.reservation.read` (`Operation.String()`: one canonical text for an operation, also used by the offline queue).
A tool that writes goes through `crud` like any other, so it must publish the change after the commit: `HarvestOps` receives the `events.Publisher` and hands it to `crud.Mount`.
The mcp client adapter (`NewCaller`) sends the same name.

## Ecosystem search

Searched `ToolNameOf`, `HarvestOps`, `crud.Op`, `qualified` over `webtyp/*` and `veltylabs/*`: `mcp` is the only builder of tool names (`harvest.go:17`, `caller.go`); `appkit` calls `HarvestOps`. **Extend** `mcp`; reuse `router.Operation.String()` (router plan) instead of a second naming function.

## Design gate

1. **Prior art** (unverified in this session): the MCP specification lets a tool name contain letters, digits, `_`, `-` and `.`; this repo already used a dot in `module.op`.
2. **Novice-name test**: `booking.reservation.read` reads as "booking module, reservation, read".
3. **Ledger**: concepts 0; string building sites −2 (`ToolNameOf`'s concatenation, `opRegistry.Operation`'s `module + "." + name`); ways to name a tool 2 → 1.
4. **Where**: `mcp` owns tool names; the canonical text of an operation is `router.Operation.String()`.
5. **Deletes**: `opRegistry.module`, `reg.module = name`, the `(module, op string)` form of `ToolNameOf`.

## Tools

```bash
go install webtyp.com/devflow/cmd/gotest@latest
```

Dependencies (all published, no `replace`): `go get webtyp.com/model@v0.2.18 webtyp.com/router@v0.4.20 webtyp.com/crud@v0.0.8 webtyp.com/events@v0.1.8 && go mod tidy`.

## APIs used

| symbol | exact signature | file |
|---|---|---|
| `router.Operation` | `struct{ Module, Entity string; Action model.Action }`; `String() string` → `"<module>.<entity>.<verb>"` | `router/op.go` (router plan) |
| `router.Caller` | `Call(op Operation, args model.Encodable, into model.Decodable, done func(err error))`; `Dispatch(op Operation, args model.Encodable)` | `router/caller.go` (router plan) |
| `router.OperationRegistry` | `Operation(op Operation, h HandlerFunc) Route` | `router/router.go` (router plan) |
| `crud.Mount` | `func Mount(reg router.OperationRegistry, public []Public, pub events.Publisher, m Module) error` | `crud/mount.go` (crud plan) |
| `crud.Module` | `ModelName() string; Definition() model.ModuleDefinition; MountOperations(reg Registry)` | `crud/contract.go` (crud plan) |
| `events.Publisher` | `Publish(e Event)` | `events/events.go:43` |

If a signature differs from this table, stop and name the symbol in the PR; never adapt it by guessing.

## Stage 1 — tests first (red)

Fixture modules (`fakeModule`, `fakeUnnamedModule`, the modules in `harvest_describe_test.go`, `harvest_plaintext_error_test.go`, `harvest_mfa_test.go`, `caller_nilargs_test.go`, `local_client_test.go`:
`grep -n "MountOperations" tests/*.go`) implement `Definition()` returning their `ModelName()` and one handler per entity they mount: `Wire: true, Allow: model.Read, Rules: model.Rules{Read: model.AddMyBusinessRule}`
for a module that mounts through `reg.Read`. `fakeUnnamedModule` returns `model.ModuleDefinition{}` (empty name).

New helper in `tests/helpers_test.go`:

```go
// toolOf is the tool name of one action on an entity served by a module.
func toolOf(module string, rec model.ModuleNaming, a model.Action) mcp.ToolName {
	return mcp.ToolNameOf(router.Operation{Module: module, Entity: rec.ModelName(), Action: a})
}
```

Mechanical edits in `tests/`: every `HarvestOps(x, mods…)` becomes `HarvestOps(nil, x, mods…)` (nil publisher); every pair
`opRead := crud.Op(&R{}, model.Read)` + `mcp.ToolNameOf("mod", opRead)` becomes `toolOf("mod", &R{}, model.Read)` (delete the `opX :=` lines); the expected text
`"duplicate tool name fake." + crud.Op(&fakeRecord{}, model.Read)` becomes `"duplicate tool name " + string(toolOf("fake", &fakeRecord{}, model.Read))`.

New tests:

| Test (file) | setup → call → expected |
|---|---|
| `TestToolNameOf_IsModuleEntityVerb` (`tests/tool_name_test.go`) | `mcp.ToolNameOf(router.Operation{Module:"booking", Entity:"reservation", Action: model.Read})=="booking.reservation.read"`; `Create`→`…create`, `Update`→`…update`, `Delete`→`…delete` |
| `TestHarvest_ToolPerAllowedAction` | a module with one table allowing `Create\|Read` (store below) → `HarvestOps(nil, [create, read tools], mod)` returns two tools; exposing the `update` tool → error "is not registered by any module" |
| `TestHarvest_WriteToolPublishesAfterCommit` | module with a `HaveNoBusinessRule` `Create` table over `memStore` (below), `pub := &recorder{}`; `HarvestOps(pub, [create], mod)`; run the tool with `{"records":[{"id":"a","name":"x"}]}` → `len(pub.got)==1`, `pub.got[0].Topic==crud.Topic("fake", "fake_record", model.Create)` |
| `TestHarvest_NilPublisherIsFine` | same module, `pub == nil` → the tool runs and stores, no panic |
| `TestCaller_SendsTheQualifiedToolName` (`tests/caller_name_test.go`) | `NewCaller(client).Call(router.Operation{Module:"fake", Entity:"fake_record", Action: model.Read}, nil, &out, done)` with the local client of `local_client_test.go` → the server handled the tool `fake.fake_record.read` |

`memStore` and `recorder` in `tests/helpers_test.go`: `recorder` is `type recorder struct{ got []events.Event }` with `func (r *recorder) Publish(e events.Event) { r.got = append(r.got, e) }`;
`memStore` implements `crud.Store` for the one record type of the test with a `map[string]…` — copy `crud/tests/memstore_test.go` of the `crud` repo (restated here: `NewID() string`, `ReadOne(rec model.Model, id string) error`,
`ReadAll(newRow func() model.Model, q crud.Query, onRow func(model.Model)) error`, `Create(rec model.Model) error`, `UpdateFields(rec model.Model, id string, fields []string) error`, `Delete(rec model.Model, id string) error`);
only `NewID` and `Create` need real bodies for these tests, the others return `model.NotFound("not found")`.

Run `gotest`: it does not compile. Commit: `test: tool names from router.Operation (red)`.

## Stage 2 — the code

`harvest.go`:

```go
// ToolName is the qualified name of one tool: "<module>.<entity>.<verb>", the text of its router.Operation.
type ToolName string

// ToolNameOf is the tool name of one operation.
//
//	mcp.ToolNameOf(router.Operation{Module: "booking", Entity: "reservation", Action: model.Read}) // "booking.reservation.read"
func ToolNameOf(op router.Operation) ToolName { return ToolName(op.String()) }
```

`HarvestOps` gets the publisher first and no longer sets a module:

```go
func HarvestOps(pub events.Publisher, expose []ToolName, modules ...crud.Module) (ToolProvider, error) {
	reg := &opRegistry{}
	for _, m := range modules {
		if m.ModelName() == "" {
			return nil, errEmptyModelName
		}
		if err := crud.Mount(reg, nil, pub, m); err != nil {
			return nil, err
		}
	}
	// …the rest (reg.err, expose, seen, finalTools) is unchanged…
```

`opRegistry`: delete the `module` field and its comment; the method becomes

```go
func (r *opRegistry) Operation(op router.Operation, h router.HandlerFunc) router.Route {
	qualified := string(ToolNameOf(op))
	// …the body is unchanged from here (duplicate check, append Tool{Name: qualified, Execute: harvestExecute(qualified, h)})…
```

`caller.go`: `Call(op router.Operation, …)` and `Dispatch(op router.Operation, …)`; `Name: string(ToolNameOf(op))` in both `CallToolParams`; the doc of `NewCaller` says "Consumers pass a typed router.Operation".
Import `webtyp.com/events` in `harvest.go`. `gotest` passes. Commit: `refactor!: tool names from router.Operation; HarvestOps takes the publisher`.

## Stage 3 — documentation

- `README.md` (≤ 150 lines, four ordered sections): the guide uses `mcp.HarvestOps(broker, []mcp.ToolName{mcp.ToolNameOf(router.Operation{…})}, mod)`; *Why This Exists* unchanged except the tool-name example.
- `docs/ARCHITECTURE.md` and `docs/SKILL.md`: replace every `<module>.<entity>_c|_r|_u|_d` and `crud.Op` with `<module>.<entity>.<create|read|update|delete>`; state that a harvested write tool announces through the publisher.
- `docs/TESTS.md`: one row per new test.
- Delete this file's stale predecessor: the old `docs/PLAN.md` ("HarvestOps takes crud.Module through crud.Mount") was executed (commit `459c536`); this file replaces it.
- Do not link `docs/PLAN*.md` from permanent docs.

Commit: `docs: tool names`.

## Delivery check

1. `gotest` (builds, vets, runs; a PR that does not compile, or whose tests could not run, is a failed delivery).
2. `grep -rn "crud\.Operation(\|ToolNameOf(\"" --include=*.go --include=*.md .` → empty.
3. `git diff --stat main` lists only `harvest.go`, `caller.go`, `go.mod`, `go.sum`, `tests/*`, `README.md`, `docs/ARCHITECTURE.md`, `docs/SKILL.md`, `docs/TESTS.md`, `docs/PLAN.md`.

| Stage | Files | Commit |
|---|---|---|
| 1 | `tests/*` | `test: … (red)` |
| 2 | `harvest.go`, `caller.go` | `refactor!: …` |
| 3 | `README.md`, `docs/*` | `docs: …` |
