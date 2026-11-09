package mcp_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"webtyp.com/context"
	"webtyp.com/json"
	"webtyp.com/mcp"
	"webtyp.com/model"
	"webtyp.com/router"
	"webtyp.com/time"
)

// mockEncodable is a simple model.Encodable for testing
type mockEncodable struct {
	Foo string
}

func (m *mockEncodable) Schema() []model.Field {
	return []model.Field{{Name: "foo", Type: model.Text()}}
}

func (m *mockEncodable) Pointers() []any {
	return []any{&m.Foo}
}

func (m *mockEncodable) EncodeFields(w model.FieldWriter) {
	w.String("foo", m.Foo)
}

func (m *mockEncodable) DecodeFields(r model.FieldReader) {
	if v, ok := r.String("foo"); ok {
		m.Foo = v
	}
}

func (m *mockEncodable) IsNil() bool {
	return m == nil
}

func TestCaller_Call_Success(t *testing.T) {
	srv, _ := mcp.NewServer(mcp.Config{Name: "test", Version: "1.0.0", Authorize: mcp.AllowAll}, nil)
	srv.AddTool(mcp.Tool{
		Name:     string(toolOf("test", &fakeRecord{}, model.Read)),
		Resource: "test",
		Action:   model.Read,
		Execute: func(ctx *context.Context, req mcp.Request) (*mcp.Result, error) {
			// req.Params.Arguments is already unwrapped from the envelope
			return mcp.Text("echo: " + req.Params.Arguments), nil
		},
	})

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

	args := &mockEncodable{Foo: "bar"}
	done := make(chan bool)
	var result mcp.TextContentList
	caller.Call(router.Operation{Module: "test", Entity: "fake_rec", Action: model.Read}, args, &result, func(err error) {
		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
		if len(result) != 1 {
			t.Fatalf("expected 1 item, got %d", len(result))
		}
		expected := `echo: {"foo":"bar"}`
		if result[0].Text != expected {
			t.Errorf("got %s, expected %s", result[0].Text, expected)
		}
		done <- true
	})
	<-done
}

func TestCaller_Call_ToolError(t *testing.T) {
	srv, _ := mcp.NewServer(mcp.Config{Name: "test", Version: "1.0.0", Authorize: mcp.AllowAll}, nil)
	srv.AddTool(mcp.Tool{
		Name:     string(toolOf("fail", &fakeRecord{}, model.Read)),
		Resource: "test",
		Action:   model.Read,
		Execute: func(ctx *context.Context, req mcp.Request) (*mcp.Result, error) {
			return &mcp.Result{IsError: true, Content: `[{"type":"text","text":"something went wrong"}]`}, nil
		},
	})

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

	done := make(chan bool)
	caller.Call(router.Operation{Module: "fail", Entity: "fake_rec", Action: model.Read}, nil, nil, func(err error) {
		if err == nil {
			t.Error("expected error, got nil")
		} else if !contains(err.Error(), "something went wrong") {
			t.Errorf("error message mismatch: %v", err)
		}
		done <- true
	})
	<-done
}

func TestCaller_Call_RPCError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"jsonrpc":"2.0","id":"1","error":{"code":-32601,"message":"Method not found"}}`))
	}))
	defer ts.Close()

	client := mcp.NewClient(ts.URL, "")
	caller := mcp.NewCaller(client)

	done := make(chan bool)
	caller.Call(router.Operation{Module: "any", Entity: "any_rec", Action: model.Read}, nil, nil, func(err error) {
		if err == nil {
			t.Error("expected error, got nil")
		} else if !contains(err.Error(), "Method not found") {
			t.Errorf("error message mismatch: %v", err)
		}
		done <- true
	})
	<-done
}

func TestCaller_Dispatch(t *testing.T) {
	called := make(chan bool, 1)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called <- true
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	client := mcp.NewClient(ts.URL, "")
	caller := mcp.NewCaller(client)

	caller.Dispatch(router.Operation{Module: "any", Entity: "any_rec", Action: model.Read}, nil)

	finished := make(chan bool)
	time.AfterFunc(1000, func() {
		finished <- true
	})

	select {
	case <-called:
		// success
	case <-finished:
		t.Fatal("Dispatch was not called")
	}
}
