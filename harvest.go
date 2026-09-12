package mcp

import (
	"webtyp.com/context"
	"webtyp.com/json"
	"webtyp.com/model"
	"webtyp.com/router"
)

// HarvestOps builds a ToolProvider from one or more router.OperationModule implementations. It
// runs each module's MountOperations against an internal router.OperationRegistry — the
// transport-neutral surface a domain module registers against without importing mcp — and
// converts every harvested operation into a Tool. This is the ONLY supported way for a domain
// module to reach the MCP transport; NewServer keeps accepting []ToolProvider for MCP-native
// providers (mcp's own tools, or a raw ToolProvider a repo still hand-writes). A composition
// root passes both:
//
//	providers := []mcp.ToolProvider{mcp.HarvestOps(catalogModule, userModule), rawProvider}
func HarvestOps(modules ...router.OperationModule) ToolProvider {
	reg := &opRegistry{}
	for _, m := range modules {
		m.MountOperations(reg)
	}
	return staticProvider(reg.tools)
}

type staticProvider []Tool

func (s staticProvider) Tools() []Tool { return []Tool(s) }

// opRegistry implements router.OperationRegistry — a ONE-method surface. It does NOT implement
// (nor pretend to be) router.Router: an op-only transport must never carry Get/Post/… it can
// neither honour nor need. There is nothing to panic on, because there is nothing to leave
// unimplemented.
type opRegistry struct {
	tools []Tool
}

func (r *opRegistry) Operation(name string, h router.HandlerFunc) router.Route {
	for _, t := range r.tools {
		if t.Name == name {
			panic("mcp: duplicate tool name \"" + name + "\" — each tool must be harvested exactly once (a module passed to HarvestOps twice, or two modules claiming the same operation name)")
		}
	}
	idx := len(r.tools)
	r.tools = append(r.tools, Tool{Name: name, Execute: harvestExecute(name, h)})
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
func harvestExecute(_ string, h router.HandlerFunc) func(ctx *context.Context, req Request) (*Result, error) {
	return func(ctx *context.Context, req Request) (*Result, error) {
		var u string
		if ctx != nil {
			u = ctx.Value(CtxKeyUserID)
		}
		oc := &opContext{userID: u, body: []byte(req.Params.Arguments)}
		h(oc)
		return &Result{IsError: oc.status >= 400, Content: string(oc.body)}, nil
	}
}
