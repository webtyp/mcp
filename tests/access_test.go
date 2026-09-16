package mcp_test

import (
	"strings"
	"testing"

	"webtyp.com/context"
	"webtyp.com/fmt"
	"webtyp.com/json"
	"webtyp.com/mcp"
	"webtyp.com/model"
)

// callTool invoca un tool y devuelve error si la verja lo rechazó.
func callTool(srv *mcp.Server, ctx *context.Context, name string) (string, error) {
	req := []byte(`{"jsonrpc":"2.0","id":"1","method":"tools/call","params":{"name":"` + name + `","arguments":{}}}`)
	resp := srv.HandleMessage(ctx, req)

	var out []byte
	if enc, ok := resp.(model.Encodable); ok {
		json.Encode(enc, &out)
	}
	s := string(out)
	if strings.Contains(s, `"error"`) {
		return s, fmt.Err("denied:", s)
	}
	return s, nil
}

func okTool(name string, access model.Access, resource model.Resource) mcp.Tool {
	var action model.Action
	if access == model.AccessGuarded {
		action = model.Read
	}
	return mcp.Tool{
		Name:     name,
		Resource: resource,
		Action:   action,
		Access:   access,
		Execute: func(ctx *context.Context, req mcp.Request) (*mcp.Result, error) {
			return mcp.Text("ok"), nil
		},
	}
}

// EL test que impide que un refactor futuro abra la puerta: un tool que no declara nada cae
// en el zero value —AccessGuarded— y exige identidad Y permiso.
func TestZeroValueAccessIsGuarded(t *testing.T) {
	auth := &rbacAuth{denyResource: "secrets", denyAction: model.Read}
	srv, err := mcp.NewServer(mcp.Config{Name: "t", Version: "1", Authorize: auth.Can}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := srv.AddTool(okTool("guarded", model.AccessGuarded, "secrets")); err != nil {
		t.Fatal(err)
	}

	t.Run("anónimo denegado", func(t *testing.T) {
		ctx := context.Background()
		if _, err := callTool(srv, ctx, "guarded"); err == nil {
			t.Error("un tool guarded dejó pasar a un anónimo")
		}
	})

	t.Run("con identidad pero sin permiso, denegado", func(t *testing.T) {
		ctx := context.Background()
		ctx.Set(mcp.CtxKeyUserID, "u1")
		if _, err := callTool(srv, ctx, "guarded"); err == nil {
			t.Error("un tool guarded dejó pasar sin permiso")
		}
	})
}

// AccessAuthenticated: exige identidad y NO consulta Authorize. Esa es la diferencia que
// permite a `me` dejar de inventarse un recurso que la app nunca declaró.
func TestAuthenticatedDoesNotConsultAuthorize(t *testing.T) {
	consulted := false
	authorize := model.Authorizer(func(string, model.Resource, model.Action) bool {
		consulted = true
		return false // aunque deniegue TODO, un tool authenticated debe pasar
	})

	srv, err := mcp.NewServer(mcp.Config{Name: "t", Version: "1", Authorize: authorize}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := srv.AddTool(okTool("me", model.AccessAuthenticated, "")); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	if _, err := callTool(srv, ctx, "me"); err == nil {
		t.Error("un anónimo llegó a un tool authenticated")
	}

	ctx = context.Background()
	ctx.Set(mcp.CtxKeyUserID, "u1")
	if _, err := callTool(srv, ctx, "me"); err != nil {
		t.Errorf("una identidad válida fue rechazada: %v", err)
	}
	if consulted {
		t.Error("se consultó Authorize para un tool authenticated: no hay recurso que comprobar")
	}
}

func TestPublicNeedsNoIdentity(t *testing.T) {
	srv, err := mcp.NewServer(mcp.Config{Name: "t", Version: "1", Authorize: mcp.AllowAll}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := srv.AddTool(okTool("ping", model.AccessPublic, "")); err != nil {
		t.Fatal(err)
	}

	if _, err := callTool(srv, context.Background(), "ping"); err != nil {
		t.Errorf("un tool público rechazó a un anónimo: %v", err)
	}
}

// Fallo RUIDOSO al arrancar, no denegación silenciosa en runtime.
func TestAddToolRejectsContradictoryAccess(t *testing.T) {
	srv, err := mcp.NewServer(mcp.Config{Name: "t", Version: "1", Authorize: mcp.AllowAll}, nil)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("guarded sin recurso", func(t *testing.T) {
		// Autorizaba contra "" y denegaba TODAS las llamadas: parecía protegido y era
		// inalcanzable, sin un solo error.
		if err := srv.AddTool(okTool("broken", model.AccessGuarded, "")); err == nil {
			t.Error("se aceptó un tool guarded sin recurso: denegaría siempre, en silencio")
		}
	})

	t.Run("guarded con recurso y Action == 0", func(t *testing.T) {
		tool := mcp.Tool{
			Name:     "guarded_no_action",
			Resource: "secrets",
			Action:   0,
			Access:   model.AccessGuarded,
			Execute:  func(ctx *context.Context, req mcp.Request) (*mcp.Result, error) { return mcp.Text("ok"), nil },
		}
		if err := srv.AddTool(tool); err == nil {
			t.Error("se aceptó un tool guarded con Action == 0: no coincidiría con ningún permiso")
		}
	})

	t.Run("authenticated con recurso", func(t *testing.T) {
		tool := mcp.Tool{
			Name:     "auth_resource",
			Resource: "secrets",
			Action:   0,
			Access:   model.AccessAuthenticated,
			Execute:  func(ctx *context.Context, req mcp.Request) (*mcp.Result, error) { return mcp.Text("ok"), nil },
		}
		if err := srv.AddTool(tool); err == nil {
			t.Error("se aceptó un tool authenticated con Resource != ''")
		}
	})

	t.Run("authenticated con action", func(t *testing.T) {
		tool := mcp.Tool{
			Name:     "auth_action",
			Resource: "",
			Action:   model.Read,
			Access:   model.AccessAuthenticated,
			Execute:  func(ctx *context.Context, req mcp.Request) (*mcp.Result, error) { return mcp.Text("ok"), nil },
		}
		if err := srv.AddTool(tool); err == nil {
			t.Error("se aceptó un tool authenticated con Action != 0")
		}
	})

	t.Run("público con recurso", func(t *testing.T) {
		// Un recurso que nadie comprueba parece protección y no la da.
		if err := srv.AddTool(okTool("fake", model.AccessPublic, "secrets")); err == nil {
			t.Error("se aceptó un tool público que declara recurso: parece protegido y no lo está")
		}
	})

	t.Run("authenticated limpia aceptada", func(t *testing.T) {
		if err := srv.AddTool(okTool("auth_clean", model.AccessAuthenticated, "")); err != nil {
			t.Errorf("se rechazó un tool authenticated limpio: %v", err)
		}
	})

	t.Run("público limpia aceptada", func(t *testing.T) {
		if err := srv.AddTool(okTool("public_clean", model.AccessPublic, "")); err != nil {
			t.Errorf("se rechazó un tool público limpio: %v", err)
		}
	})

	t.Run("Tool vacia rechazada closed by default", func(t *testing.T) {
		if err := srv.AddTool(mcp.Tool{}); err == nil {
			t.Error("se aceptó una Tool vacía")
		}
	})
}
