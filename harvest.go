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
//
// Every harvested Tool.Name is qualified by its module's ModelName() as
// "<ModelName>.<name>" — an operation name has no owner on its own, and two
// modules that independently pick the same bare name (e.g. both calling
// something "get_day_bounds") is not a defect in either: it is exactly the
// collision qualification exists to make unrepresentable. A module whose
// ModelName() is "" cannot mount operations at all — HarvestOps panics
// rather than harvest an unqualified (and therefore collision-prone) name.
func HarvestOps(modules ...router.OperationModule) ToolProvider {
	reg := &opRegistry{}
	for _, m := range modules {
		name := m.ModelName()
		if name == "" {
			panic("mcp: HarvestOps: a module returned an empty ModelName() — every operation must be qualified by its owning module")
		}
		reg.module = name
		m.MountOperations(reg)
	}
	return staticProvider(reg.tools)
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
}

func (r *opRegistry) Operation(name string, h router.HandlerFunc) router.Route {
	qualified := r.module + "." + name
	for _, t := range r.tools {
		if t.Name == qualified {
			panic("mcp: duplicate tool name \"" + qualified + "\" — each tool must be harvested exactly once (a module passed to HarvestOps twice, or the same module registering the same operation name twice)")
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
// The success and failure branches build Content differently, and that split is deliberate, not
// an oversight: on success, oc.body is whatever the handler wrote via ctx.Encode — already
// serialized JSON — so it is safe to embed verbatim (Result.EncodeFields does exactly that via
// w.Raw). On failure, oc.body is PLAIN TEXT: every domain module in this ecosystem's own
// convention is ctx.WriteStatus(4xx/5xx) followed by ctx.Write([]byte(err.Error())), never a
// JSON-encoded value. Embedding plain text verbatim after "content": produces a syntactically
// broken JSON-RPC response the moment the message contains a space (any real error message) —
// confirmed by a real client-server round trip, not a caller misusing IsError. Text(...) is the
// same helper handleToolCall's own recovered-error branch already uses for an identical reason;
// this mirrors it instead of introducing a second way to wrap an error string.
func harvestExecute(_ string, h router.HandlerFunc) func(ctx *context.Context, req Request) (*Result, error) {
	return func(ctx *context.Context, req Request) (*Result, error) {
		var u string
		if ctx != nil {
			u = ctx.Value(CtxKeyUserID)
		}
		oc := &opContext{userID: u, body: []byte(req.Params.Arguments)}
		h(oc)
		if oc.status >= 400 {
			return &Result{IsError: true, Content: Text(string(oc.body)).Content}, nil
		}
		return &Result{Content: string(oc.body)}, nil
	}
}
