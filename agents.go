package mcp

import (
	"webtyp.com/crud"
	"webtyp.com/events"
	"webtyp.com/model"
	"webtyp.com/router"
)

// serverVersion is the version every Tools server announces to a client.
const serverVersion = "0.0.1"

// Tools lets an agent call the operations the app allows, at MCPPath. The app names it where it chooses its
// transports; appkit only sees the small interface it declares. Allowed empty = the agent can call nothing.
//
//	appkit.Backend{Agents: mcp.Tools{Allowed: MCPTools()}}
type Tools struct {
	Allowed []ToolName // the operations an agent may call
}

// MountTools builds the MCP server named name with the allowed operations of ops and mounts it on r.
// authorize decides each call; pub is where those operations announce after they commit.
func (t Tools) MountTools(r router.Router, name string, authorize model.Authorizer, pub events.Publisher, ops []crud.Module) error {
	provider, err := HarvestOps(pub, t.Allowed, ops...)
	if err != nil {
		return err
	}
	s, err := NewServer(Config{Name: name, Version: serverVersion, Authorize: authorize}, []ToolProvider{provider})
	if err != nil {
		return err
	}
	s.MountAPI(r)
	return nil
}
