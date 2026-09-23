package mcp_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"webtyp.com/context"
	"webtyp.com/json"
	"webtyp.com/mcp"
	"webtyp.com/model"
	"webtyp.com/router"
)

// plainTextErrorModule mimics the error-writing convention every domain
// module in this ecosystem already uses (see e.g.
// veltylabs/appointment_booking's own writeError): decode → business logic
// → on failure, ctx.WriteStatus(4xx/5xx) then ctx.Write([]byte(err.Error()))
// — PLAIN TEXT, never a JSON-encoded value. A tool harvested via HarvestOps
// must still be wire-compatible when that plain text reaches a real caller.
type plainTextErrorModule struct{}

func (plainTextErrorModule) ModelName() string { return "plaintext" }
func (plainTextErrorModule) MountOperations(r router.OperationRegistry) {
	r.Operation("fail", func(ctx router.Context) {
		ctx.WriteStatus(404)
		ctx.Write([]byte("patient not found"))
	}).Requires("plaintext", model.Read)
}

var _ router.OperationModule = plainTextErrorModule{}

// TestHarvestOps_PlainTextErrorSurvivesRealWireRoundTrip: harvestExecute
// builds Result.Content directly from the handler's raw response body
// (Content: string(oc.body)), and Result.EncodeFields embeds Content via
// w.Raw — verbatim, no JSON-string quoting. That is correct when the body
// is already-serialized JSON (ctx.Encode's normal success output), but a
// PLAIN TEXT error body — the ecosystem-wide error-writing convention — is
// not valid JSON on its own: embedding `patient not found` unquoted after
// `"content":` produces a syntactically broken JSON-RPC response. Every
// existing test that touches Result.Content on the error path
// (TestCaller_Call_ToolError) hand-constructs an already-valid
// `[{"type":"text","text":"..."}]` Content and never exercises
// harvestExecute's own construction — this is what closes that gap.
func TestHarvestOps_PlainTextErrorSurvivesRealWireRoundTrip(t *testing.T) {
	provider := mcp.HarvestOps(plainTextErrorModule{})
	srv, err := mcp.NewServer(mcp.Config{Name: "test", Version: "1.0.0", Authorize: mcp.AllowAll}, []mcp.ToolProvider{provider})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
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

	caller := mcp.NewCaller(mcp.NewClient(ts.URL, ""))

	done := make(chan error, 1)
	caller.Call("plaintext.fail", nil, nil, func(err error) { done <- err })
	err = <-done

	if err == nil {
		t.Fatal("expected an error from a tool that reports failure, got nil")
	}
	if !strings.Contains(err.Error(), "patient not found") {
		t.Fatalf("expected the real error message to survive the wire round trip, got: %v", err)
	}
	// The message is what a person reads in a UI — it must be the sentence the
	// handler wrote, not the content-block envelope that carried it. A caller
	// that pastes Content in raw shows `[{"type":"text","text":"patient not
	// found"}]` to the user, which is the transport leaking through the seam.
	if strings.Contains(err.Error(), `{"type"`) {
		t.Errorf("the caller must unwrap the content block, not paste it raw: %v", err)
	}
}
