package mcp_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"webtyp.com/context"
	"webtyp.com/json"
	"webtyp.com/mcp"
	"webtyp.com/model"
)

// TestCaller_Call_NilArgsAgainstOpThatDecodes reproduces the bug found while
// debugging why every list screen in a downstream app (mjosefa-cms,
// "Funcionarios"/"Equipos") showed "0 / 0" even for a tenant with real rows
// in the database — see docs/PLAN.md.
//
// webtyp.com/view's callerLister.list() (the code every list-driven Presenter
// runs on Reload) calls Caller.Call(op, nil, dec, done) — nil args is the
// documented, legitimate way to say "this call has nothing to send" (see
// router.Caller's own doc: "into may be nil when..."; the same convention
// applies to args). The op on the other end still declares .Accepts(&SomeArgs{})
// (list ops filter by tenant, so they always accept a real args struct — see
// AGENTS.md "A no-args op declares .Accepts(nil)", which by exclusion means an
// op that DOES accept an args struct must tolerate an absent/empty one), and
// its handler calls ctx.Decode(&args) expecting an empty struct, not an
// error, when nothing was sent — this is exactly fakeModule's do_thing below.
//
// mcpCaller.Call, when args == nil, leaves argsJSON as the Go zero value ""
// (see caller.go: `if args != nil { encode into argsJSON }` — never sets it
// otherwise) and sends CallToolParams{Arguments: ""}. CallToolParams.
// EncodeFields writes it with w.Raw("arguments", m.Arguments) — and
// jsonWriter.Raw (encode.go) turns an EMPTY string into the JSON literal
// `null`, not `{}` and not an omitted key. On the server, opContext.body
// becomes the four bytes `null`, and ctx.Decode (json.Decode) rejects it
// with "expected object, got n" because `null` is valid JSON but not an
// object — so the op fails with a 400/500 for every caller that (correctly,
// per the interface's own doc) uses `nil` to mean "no args", even though the
// op's handler was written to tolerate exactly that case.
//
// The fix belongs in mcpCaller.Call: encode nil args as the literal string
// "{}" (a valid empty JSON object), never as the empty Go string "" — see
// docs/PLAN.md.
func TestCaller_Call_NilArgsAgainstOpThatDecodes(t *testing.T) {
	provider := mcp.HarvestOps(fakeModule{})
	srv, err := mcp.NewServer(mcp.Config{
		Name:      "test-server",
		Version:   "1.0.0",
		Authorize: mcp.AllowAll,
	}, []mcp.ToolProvider{provider})
	if err != nil {
		t.Fatalf("mcp.NewServer: %v", err)
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body := make([]byte, r.ContentLength)
		r.Body.Read(body)

		var ctx context.Context
		ctx.Set(mcp.CtxKeyUserID, "test-user")
		resp := srv.HandleMessage(&ctx, body)

		var b []byte
		if f, ok := resp.(model.Encodable); ok {
			json.Encode(f, &b)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(b)
	}))
	defer ts.Close()

	client := mcp.NewClient(ts.URL, "")
	caller := mcp.NewCaller(client)

	done := make(chan error, 1)
	var out fakeArgs
	// This is EXACTLY what webtyp.com/view's callerLister.list() does on
	// every Reload(): pass nil for args because there is nothing this call
	// needs to send, and decode the (possibly empty) result into a real,
	// non-nil destination.
	caller.Call("fake.do_thing", nil, &out, func(err error) {
		done <- err
	})

	if err := <-done; err != nil {
		t.Fatalf("caller.Call with nil args against an op that calls ctx.Decode failed: %v — "+
			"this is the root cause of the \"0 / 0\" list bug: view.NewCallerLister always "+
			"passes nil for args on List, and nil must round-trip as an empty JSON object, "+
			"never as the JSON literal null", err)
	}
	if out.Value != "echo:" {
		t.Errorf("out.Value = %q, want %q (fakeModule.do_thing echoes empty input back)", out.Value, "echo:")
	}
}
