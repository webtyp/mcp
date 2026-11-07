---
PLAN: "refactor!: HarvestOps takes crud.Module through crud.Mount"
EXECUTOR: antigravity
REVIEWER: none
---

> Read AGENTS.md at the repo root before any change.
> **Finish by yourself:** when the delivery check passes, commit after every stage and stop. Never ask whether to
> proceed. This plan runs **locally**: do not tag, push or publish anything; the owner reviews first.
> Never run `git reset`, `git restore`, `git checkout -- <file>` or `git stash` on your own work.

# Plan: `mcp.HarvestOps` accepts only `crud.Module`

Context: master `webtyp/docs/ARCHITECTURE_MASTER_PLAN.md` D7, D14. Runs after the router and crud plans.
Other plans parked in this repo (`docs/PLAN_*.md`) are not part of this change: do not touch them.

## Tools

```bash
go install webtyp.com/devflow/cmd/gotest@latest
```
`go.mod`: `replace` for `webtyp.com/model`, `webtyp.com/router`, `webtyp.com/crud` to `../model`, `../router`, `../crud`; `go get webtyp.com/crud`; `go mod tidy`.

## APIs used

| Symbol | Signature |
|---|---|
| `crud.Module` | `interface{ model.ModuleNaming; MountOperations(reg crud.Registry) }` |
| `crud.Mount` | `func Mount(reg router.OperationRegistry, public []model.ModuleNaming, m crud.Module) error` |
| `crud.Op` | `func Op(m model.ModuleNaming, action model.Action) string` → `"<ModelName>_c\|_r\|_u\|_d"` |

## Stage 1 — `harvest.go`

```go
func HarvestOps(expose []ToolName, modules ...crud.Module) (ToolProvider, error)
```
Replace `m.MountOperations(reg)` with `if err := crud.Mount(reg, nil, m); err != nil { return nil, err }`
(MCP never mounts public reads: every tool needs an identity). `opRegistry`/`opRoute` keep implementing
`router.OperationRegistry`/`router.Route`. Update the `ToolNameOf` comment example to
`mcp.ToolNameOf(booking.ModelName, crud.Op(&booking.Reservation{}, model.Read))`.

## Stage 2 — tests

`tests/harvest_test.go`, `tests/harvest_mfa_test.go`, `tests/harvest_plaintext_error_test.go`: their fake modules become
`crud.Module`; tool names in expectations become `"<module>.<entity>_r"` etc. Keep every existing rule (closed by default,
duplicate, typo in expose, MFA tool refused). New: `TestHarvestOps_CrudWiringError` — module calling
`reg.Create(nil, nil)` → `HarvestOps` returns `crud.ErrNilFunction`. Rows in `docs/TESTS.md`.

## Stage 3 — documentation

`README.md` / `docs/*`: `HarvestOps` with a `crud.Module`; remove `OperationModule`.

## Delivery check

`gotest` passes; `grep -rn "OperationModule" --include=*.go .` → empty.

| Stage | Files |
|---|---|
| 1 | `harvest.go` |
| 2 | `tests/harvest*_test.go`, `docs/TESTS.md` |
| 3 | `README.md`, `docs/*` |
