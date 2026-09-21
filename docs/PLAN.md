---
PLAN: "fix: mcp.Caller encodes nil args as JSON null, breaking every op whose handler decodes an (optionally empty) args struct"
EXECUTOR: jules
REVIEWER: none
STATUS: review
SESSION: 9324426338688325668
PR: https://github.com/webtyp/mcp/pull/30
---

> This plan is dispatched via the CodeJob workflow. See skill: agents-workflow.

# PLAN — `mcpCaller.Call`: nil args must round-trip as `{}`, never as `null`

You are an external agent with **zero prior context** about this project. Everything you need is
in this file. Read it fully before writing code.

## 0. Prerequisite — run this first

```bash
go install webtyp.com/devflow/cmd/gotest@latest
```

All tests run with `gotest`, never `go test` directly.

## 1. The bug, proven by a failing test already in this repo

`tests/caller_nilargs_test.go` (already committed on this branch) reproduces a real,
user-visible production bug found in a downstream app: every list screen in an app built on
`webtyp.com/view` + `webtyp.com/layout/crudview` showed **zero rows even when the database held
real data for the current tenant**. Run it now and confirm it is RED:

```bash
gotest
# --- FAIL: TestCaller_Call_NilArgsAgainstOpThatDecodes
#     caller_nilargs_test.go:87: caller.Call with nil args against an op that calls
#     ctx.Decode failed: mcp: tool execution failed: null
```

This is the acceptance criterion for this plan: **that test must go GREEN, and no other test may
regress.**

## 2. Root cause — traced end to end, do not re-derive it

1. `webtyp.com/view`'s `callerLister.list()` (`caller_lister.go`) calls
   `b.caller.Call(b.ops.List, nil, dec, callback)` — it passes **`nil`** for args. This is
   correct and by design: `router.Caller.Call`'s own doc says "args ... may be nil",
   and a List op filters by tenant via a fallback the server already applies
   (e.g. `if tenantID == "" { tenantID = m.tenantID }`), so the op is written to tolerate an
   absent/empty args object — it must NOT require one.
2. In this file, `caller.go`, function `(c *mcpCaller) Call`:
   ```go
   func (c *mcpCaller) Call(op string, args model.Encodable, into model.Decodable, done func(err error)) {
       var argsJSON string
       if args != nil {
           if err := json.Encode(args, &argsJSON); err != nil {
               if done != nil {
                   done(err)
               }
               return
           }
       }
       params := &CallToolParams{
           Name:      op,
           Arguments: argsJSON,
       }
       ...
   ```
   When `args == nil`, `argsJSON` is never assigned — it stays the Go zero value `""`.
   `CallToolParams.Arguments` is therefore sent as the empty string.
3. `CallToolParams.EncodeFields` (generated, `model_orm.go`) writes it as
   `w.Raw("arguments", m.Arguments)`.
4. `webtyp.com/json`'s `jsonWriter.Raw` (`encode.go`) has this behavior:
   ```go
   func (w *jsonWriter) Raw(name, val string) {
       w.maybeComma()
       w.writeKey(name)
       if val == "" {
           w.b.WriteString("null")
           return
       }
       w.b.WriteString(val)
   }
   ```
   An empty raw string is written as the **JSON literal `null`**. So the wire request becomes
   `{"name":"<op>","arguments":null}`.
5. On the server, `opContext.body` (`harvest.go`) is set to
   `[]byte(req.Params.Arguments)` — after decode, this is the 4 bytes `null`.
6. The op's handler calls `ctx.Decode(&args)` → `json.Decode(c.body, into)`. `webtyp.com/json`'s
   `Decode` requires the top-level value to be a JSON object for a non-slice destination
   (`if p.peek() != '{' { return fmt.Err("json", "decode", "expected object, got "+...) }`).
   `null` is valid JSON but is not `{`, so decode fails, and the handler correctly (per its own
   contract) responds with a 4xx/5xx status.
7. `Result.Content` stays whatever `oc.body` was before the handler ran (it never got the chance
   to write anything) — literally the 4 bytes `null`. `mcpCaller.Call`'s callback sees
   `res.IsError == true` and returns `fmt.Err("mcp: tool execution failed: " + res.Content)`,
   i.e. the exact message the failing test captures: `mcp: tool execution failed: null`.
8. Upstream of all this, `webtyp.com/layout/crudview`'s `Reload()` only does
   `Log(err.Error())` on failure — there is no default visible error surface — which is why this
   was never noticed in the browser: the list simply renders empty, silently. (That silent-failure
   gap is a **separate** defect, tracked in `webtyp.com/layout`'s own `docs/PLAN.md` — do not fix
   it here, this plan is scoped to the wire-encoding bug only.)

**Net effect:** every op that legitimately accepts an args struct but is called with `nil` (the
correct thing to do when the caller has nothing to send) fails, unconditionally, across the whole
ecosystem — this is not specific to any one downstream app or module.

## 3. The fix

In `caller.go`, `(c *mcpCaller) Call`: when `args == nil`, set `argsJSON` to the literal string
`"{}"` (a valid, empty JSON object) instead of leaving it as the Go zero-value empty string.
`{}` decodes successfully into any struct destination, leaving every field at its zero value —
exactly the "no args" semantics the caller intended.

```go
func (c *mcpCaller) Call(op string, args model.Encodable, into model.Decodable, done func(err error)) {
	argsJSON := "{}"
	if args != nil {
		if err := json.Encode(args, &argsJSON); err != nil {
			if done != nil {
				done(err)
			}
			return
		}
	}

	params := &CallToolParams{
		Name:      op,
		Arguments: argsJSON,
	}
	...
```

Apply the same change to `(c *mcpCaller) Dispatch`, for the identical reason (it has the same
`var argsJSON string; if args != nil { ... }` shape) — `Dispatch` is fire-and-forget so there is
no test-visible symptom today, but it carries the exact same latent bug and must not be left
inconsistent with `Call`.

### What NOT to change

- **Do not touch `webtyp.com/json`'s `jsonWriter.Raw`.** Its "empty string → `null`" behavior is
  relied upon elsewhere in this very repo — `Result.EncodeFields` (`model_orm.go`) deliberately
  guards it: `if len(m.Content) != 0 { w.Raw("content", m.Content) }`. That guard is proof the
  author already knew `Raw("")` becomes `null` and designed around it. The fix belongs at the
  call site that has an "empty means absent, and absent must mean `{}`" contract — that is
  `mcpCaller.Call`/`Dispatch`, not the generic `Raw` primitive.
- **Do not change `webtyp.com/view` or `webtyp.com/layout`.** `callerLister.list()` passing `nil`
  is correct per `router.Caller`'s own documented contract; the bug is entirely in how this
  package turns that `nil` into wire bytes.
- **Do not add a workaround in any downstream app** (e.g. a consumer passing a fake non-nil args
  struct to dodge this). The fix is here, once, for every caller.

## 4. Verification

```bash
gotest
# vet ✅, race ✅, tests ✅ — TestCaller_Call_NilArgsAgainstOpThatDecodes now PASSES,
# and every other existing test (TestCaller_Call_Success, TestCaller_Call_ToolError,
# TestCaller_Call_RPCError, TestCaller_Dispatch, all of harvest_test.go, etc.) still passes.
```

Do not weaken or delete `tests/caller_nilargs_test.go` — it is the regression guard for this
exact bug and must remain exactly as strict (asserting `err == nil` and the decoded value) after
the fix as it is today while red.

## 5. Downstream consumers (informational — not part of this plan's scope)

Once this ships as a new tagged version, every app pinning `webtyp.com/mcp` needs a version bump
to pick up the fix (e.g. `github.com/veltylabs/mjosefa-cms` currently pins `v0.2.28`). That bump
is each consumer's own job, tracked in that repo, not here.

## Stages

| # | Stage | File(s) | Acceptance |
|---|---|---|---|
| 1 | Fix `Call` | `caller.go` | `argsJSON := "{}"` default, same `if args != nil` overwrite logic |
| 2 | Fix `Dispatch` | `caller.go` | Same default applied for consistency, even though untested today |
| 3 | Verify | — | `gotest` green, `TestCaller_Call_NilArgsAgainstOpThatDecodes` passes, no other test changed |
