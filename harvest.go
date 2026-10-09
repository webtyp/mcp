package mcp

import (
	"webtyp.com/context"
	"webtyp.com/fmt"
	"webtyp.com/json"
	"webtyp.com/model"
	"webtyp.com/router"
)

// ToolName is a qualified tool name: "<ModelName>.<operation>".
type ToolName string

// ToolNameOf builds the qualified name of one operation, from the module's constants:
// mcp.ToolNameOf(booking.ModelName, booking.OpListReservationsByStaff).
func ToolNameOf(module, op string) ToolName {
	return ToolName(module + "." + op)
}

type harvestError string

func (e harvestError) Error() string { return string(e) }

const errEmptyModelName = harvestError("mcp: module with empty ModelName()")

// HarvestOps runs each module's MountOperations and returns a ToolProvider with ONLY the
// operations named in expose. An empty expose exposes nothing (closed by default).
// Errors (nothing is returned): a module with an empty ModelName(), the same qualified name
// registered twice, a name repeated in expose, or a name in expose that no module registered
// (a typo must fail at startup, not silently hide a tool).
func HarvestOps(expose []ToolName, modules ...router.OperationModule) (ToolProvider, error) {
	reg := &opRegistry{}
	for _, m := range modules {
		name := m.ModelName()
		if name == "" {
			return nil, errEmptyModelName
		}
		reg.module = name
		m.MountOperations(reg)
	}

	if reg.err != nil {
		return nil, reg.err
	}

	if len(expose) == 0 {
		return staticProvider(nil), nil
	}

	var finalTools []Tool
	seen := make([]ToolName, 0, len(expose))

	for _, exp := range expose {
		for _, s := range seen {
			if s == exp {
				return nil, harvestError("mcp: tool " + string(exp) + " listed twice in expose")
			}
		}
		seen = append(seen, exp)

		found := false
		for _, t := range reg.tools {
			if t.Name == string(exp) {
				finalTools = append(finalTools, t)
				found = true
				break
			}
		}
		if !found {
			return nil, harvestError("mcp: exposed tool " + string(exp) + " is not registered by any module")
		}
	}

	return staticProvider(finalTools), nil
}

type staticProvider []Tool

func (s staticProvider) Tools() []Tool { return []Tool(s) }

// opRegistry implements router.OperationRegistry — a ONE-method surface. It does NOT implement
// (nor pretend to be) router.Router: an op-only transport must never carry Get/Post/… it can
// neither honour nor need. There is nothing to panic on beyond a genuine duplicate, because
// there is nothing to leave unimplemented.
type opRegistry struct {
	tools []Tool
	// module is the ModelName() of whichever module HarvestOps is currently
	// mounting — set once per module, before that module's MountOperations
	// runs, and is what qualifies every name Operation registers.
	module string
	err    error
}

func (r *opRegistry) Operation(name string, h router.HandlerFunc) router.Route {
	qualified := r.module + "." + name
	for _, t := range r.tools {
		if t.Name == qualified {
			r.err = harvestError("mcp: duplicate tool name " + qualified)
			idx := len(r.tools)
			r.tools = append(r.tools, Tool{Name: qualified})
			return &opRoute{owner: r, idx: idx}
		}
	}
	idx := len(r.tools)
	r.tools = append(r.tools, Tool{Name: qualified, Execute: harvestExecute(qualified, h)})
	return &opRoute{owner: r, idx: idx}
}

var _ router.OperationRegistry = (*opRegistry)(nil)

// opRoute implements router.Route, writing straight into the Tool opRegistry already appended —
// no copy-then-writeback: Requires/Accepts/etc mutate the SAME Tool by index.
type opRoute struct {
	owner *opRegistry
	idx   int
}

func (rt *opRoute) Requires(resource model.Resource, action model.Action) router.Route {
	t := &rt.owner.tools[rt.idx]
	t.Access, t.Resource, t.Action = model.AccessGuarded, resource, action
	return rt
}
func (rt *opRoute) Authenticated() router.Route {
	rt.owner.tools[rt.idx].Access = model.AccessAuthenticated
	return rt
}
func (rt *opRoute) Public() router.Route {
	rt.owner.tools[rt.idx].Access = model.AccessPublic
	return rt
}
func (rt *opRoute) Accepts(args model.Fielder) router.Route {
	rt.owner.tools[rt.idx].Args = args
	return rt
}

// Describe records what the route does; a transport that lists routes publishes it.
func (rt *opRoute) Describe(text string) router.Route {
	rt.owner.tools[rt.idx].Description = text
	return rt
}

var _ router.Route = (*opRoute)(nil)

// opContext adapts one mcp.Request into router.Context so a router.HandlerFunc (registered via
// Operation) can run unmodified against the MCP transport — the same handler would run verbatim
// under any future op transport that harvests the SAME module's MountOperations.
type opContext struct {
	userID string
	body   []byte // request: raw Arguments; after the handler runs: what it wrote via Encode/Write
	status int
}

func (c *opContext) Method() string                          { return "POST" }
func (c *opContext) Path() string                             { return "" }
func (c *opContext) Body() []byte                             { return c.body }
func (c *opContext) GetHeader(string) string                  { return "" }
func (c *opContext) SetHeader(string, string)                 {}
func (c *opContext) WriteStatus(code int)                     { c.status = code }
func (c *opContext) Write(b []byte) (int, error)              { c.body = append([]byte{}, b...); return len(b), nil }
func (c *opContext) SetValue(string, string)                  {}
func (c *opContext) Value(string) string                      { return "" }
func (c *opContext) Param(string) string                      { return "" }
func (c *opContext) SetCookie(router.Cookie)                  {}
func (c *opContext) Cookie(string) (router.Cookie, bool)      { return router.Cookie{}, false }
func (c *opContext) SetUserID(id string)                      { c.userID = id }
func (c *opContext) UserID() string                           { return c.userID }
func (c *opContext) Decode(into model.Decodable) error        { return json.Decode(c.body, into) }
func (c *opContext) Encode(v model.Encodable) error {
	var out []byte
	if err := json.Encode(v, &out); err != nil {
		return err
	}
	c.body = out
	return nil
}

var _ router.Context = (*opContext)(nil)

// harvestExecute wraps a router.HandlerFunc as a Tool.Execute. name is unused today (kept for a
// future error-message improvement) — silence the unused-param lint with _ if your toolchain
// complains, do not delete the parameter (keeps the call site self-documenting).
//
// A status >= 400 becomes the ERROR return, never a Result: the body a handler writes on failure
// is PLAIN TEXT — ctx.WriteStatus(4xx/5xx) then ctx.Write([]byte(err.Error())) is the convention
// every domain module follows, and router/loopback (in the repo that owns router.Context) reads
// it back exactly that way. Result.Content is the opposite: EncodeFields embeds it with w.Raw,
// verbatim, so it must already BE valid JSON. Putting plain text there broke the JSON-RPC
// response the moment the message contained a space, i.e. for every real error message.
// Returning it as an error hands it to handleToolCall, which already wraps a failed Execute in
// Text(...) — the one place that knows how to turn an error string into Content.
func harvestExecute(_ string, h router.HandlerFunc) func(ctx *context.Context, req Request) (*Result, error) {
	return func(ctx *context.Context, req Request) (*Result, error) {
		var u string
		if ctx != nil {
			u = ctx.Value(CtxKeyUserID)
		}
		oc := &opContext{userID: u, body: []byte(req.Params.Arguments)}
		h(oc)
		if oc.status >= 400 {
			return nil, fmt.Err(string(oc.body))
		}
		return &Result{Content: string(oc.body)}, nil
	}
}
