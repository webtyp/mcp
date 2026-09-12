---
PLAN: "fix: HarvestOps panics loudly on a duplicate tool name instead of registering it twice"
EXECUTOR: jules
REVIEWER: none
---

> This plan is dispatched via the CodeJob workflow. See skill: agents-workflow.
>
> **Phase C (parallel, non-gating)** of
> [`LAN_RUT_AUTH_MASTER_PLAN.md`](https://github.com/tinywasm/app/blob/main/docs/LAN_RUT_AUTH_MASTER_PLAN.md).
> Doctrine: `CONSTRUCTION_HARNESS.md` in `tinywasm/app` docs — "the only
> possible failure modes must be a compile error or a loud development
> diagnostic — never a runtime mystery."

# Plan — `webtyp.com/mcp`: duplicate tools fail at wiring time

## 0. Context

`HarvestOps` (`harvest.go`) appends every harvested operation into one flat
`[]Tool` with **no name check**. Harvesting two modules that register the same
op name (e.g. `authority.Module` and an app-local module both exposing
`"me"`) silently produces two tools with one name; which answers is a
runtime mystery. The mjosefa-cms plan that this wave supersedes had to carry
a prose rule ("do not harvest `authMod` twice") plus a `grep` acceptance
criterion to guard against it — the exact "thing you have to remember" the
harness checklist calls a hole.

This is a behavior change only — no new exported symbol, no signature
changes. The design gate is answered in short form: the failure mode moves
from *silent runtime ambiguity* to *loud startup panic*, which is the
harness's prescribed order (compile error → loud development diagnostic →
never silent failure). A panic is correct here because harvesting happens
once, at composition-root wiring, during startup — never per request.

## Stage 1 — the check

**File:** `harvest.go`, `opRegistry.Operation`.

Before appending, scan `r.tools` for the name; on a hit, panic with exactly:

```go
panic("mcp: duplicate tool name \"" + name + "\" — each tool must be harvested exactly once (a module passed to HarvestOps twice, or two modules claiming the same operation name)")
```

The scan is linear over already-registered tools at wiring time — no
performance concern, no map needed.

## Stage 2 — consumer-shaped test

**File:** `harvest_test.go` (new, or extend the existing harvest tests if
present — check first; do not duplicate a suite).

Two tiny `router.OperationModule` fakes both registering `"me"` →
`HarvestOps(a, b)` panics with a message containing `duplicate tool name
"me"`; one module registering two distinct names → both tools present, no
panic; the same module instance passed twice → panics (that is the real-world
footgun).

## Stage 3 — docs

`docs/SKILL.md` / `README.md`: one line under `HarvestOps` — duplicate names
panic at wiring time. VERIFY against the implementation.

## Acceptance criteria

1. `go build ./...`, `go vet ./...`, `gotest ./...` green.
2. `grep -rn "duplicate tool name" harvest.go` → exactly one hit (the panic).
3. The test proves the panic message names the duplicated tool.

| Stage | File | Action |
|---|---|---|
| 1 | `harvest.go` | duplicate-name panic in `opRegistry.Operation` |
| 2 | `harvest_test.go` | consumer-shaped proof |
| 3 | `README.md`/`docs/SKILL.md` | verify docs |
