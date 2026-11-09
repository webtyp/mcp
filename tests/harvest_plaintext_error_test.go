package mcp_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"webtyp.com/context"
	"webtyp.com/crud"
	"webtyp.com/json"
	"webtyp.com/mcp"
	"webtyp.com/model"
	"webtyp.com/router"
)

type failRecord struct {
	ID string
}

func (r *failRecord) ModelName() string { return "fail_rec" }
func (r *failRecord) IsNil() bool       { return r == nil }
func (r *failRecord) Schema() []model.Field {
	return []model.Field{{Name: "id", Type: model.Text()}}
}
func (r *failRecord) Pointers() []any                  { return []any{&r.ID} }
func (r *failRecord) EncodeFields(w model.FieldWriter) { w.String("id", r.ID) }
func (r *failRecord) DecodeFields(d model.FieldReader) { r.ID, _ = d.String("id") }

type failRecordList []*failRecord

func (s *failRecordList) Len() int                         { return len(*s) }
func (s *failRecordList) At(i int) model.Fielder           { return (*s)[i] }
func (s *failRecordList) Append() model.Fielder            { v := &failRecord{}; *s = append(*s, v); return v }
func (s *failRecordList) IsNil() bool                      { return s == nil }
func (s *failRecordList) EncodeFields(_ model.FieldWriter) {}
func (s *failRecordList) DecodeFields(_ model.FieldReader) {}

type plainTextErrorModule struct{}

func (plainTextErrorModule) ModelName() string { return "plaintext" }
func (m plainTextErrorModule) Definition() model.ModuleDefinition {
	return readOnlyDefinition(m.ModelName(), &failRecord{})
}
func (plainTextErrorModule) MountOperations(reg crud.Registry) {
	reg.AddReadOnlyEntity(&failRecord{}, nil, func(c crud.CurrentUser, args crud.ReadArgs, filter model.Model) (model.ModelSlice, error) {
		return nil, model.NotFound("patient not found")
	})
}

var _ crud.Module = plainTextErrorModule{}

func TestHarvestOps_PlainTextErrorSurvivesRealWireRoundTrip(t *testing.T) {
	opRead := toolOf("plaintext", &failRecord{}, model.Read)
	provider, err := mcp.HarvestOps(nil, []mcp.ToolName{opRead}, plainTextErrorModule{})
	if err != nil {
		t.Fatalf("HarvestOps: %v", err)
	}
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
	caller.Call(router.Operation{Module: "plaintext", Entity: "fail_rec", Action: model.Read}, nil, nil, func(err error) { done <- err })
	err = <-done

	if err == nil {
		t.Fatal("expected an error from a tool that reports failure, got nil")
	}
	if !strings.Contains(err.Error(), "patient not found") {
		t.Fatalf("expected the real error message to survive the wire round trip, got: %v", err)
	}
	if strings.Contains(err.Error(), `{"type"`) {
		t.Errorf("the caller must unwrap the content block, not paste it raw: %v", err)
	}
}
