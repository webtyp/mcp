package mcp

import (
	"webtyp.com/context"
	"webtyp.com/fmt"
	"webtyp.com/json"
	"webtyp.com/model"
	"webtyp.com/router"
)

type mcpCaller struct {
	client *Client
}

// NewCaller adapts *Client to the router.Caller contract. It is the single
// place that knows the tools/call envelope. Consumers (views) depend on
// router.Caller only and pass logical operation names (tool names).
func NewCaller(c *Client) router.Caller {
	return &mcpCaller{client: c}
}

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

	c.client.Call(context.Background(), string(MethodToolsCall), params, func(raw []byte, err error) {
		if err != nil {
			if done != nil {
				done(err)
			}
			return
		}

		res, err := ParseResult(raw)
		if err != nil {
			if done != nil {
				done(err)
			}
			return
		}

		if res.IsError {
			if done != nil {
				done(fmt.Err("mcp: tool execution failed: " + errorText(res)))
			}
			return
		}

		if into == nil {
			if done != nil {
				done(nil)
			}
			return
		}

		err = json.Decode([]byte(res.Content), into)
		if done != nil {
			done(err)
		}
	})
}

func (c *mcpCaller) Dispatch(op string, args model.Encodable) {
	argsJSON := "{}"
	if args != nil {
		json.Encode(args, &argsJSON)
	}

	params := &CallToolParams{
		Name:      op,
		Arguments: argsJSON,
	}

	c.client.Dispatch(context.Background(), string(MethodToolsCall), params)
}

// errorText is the message a failed call shows a person. A Result carries its
// failure as content BLOCKS ([{"type":"text","text":"..."}]) — the protocol's
// shape, produced by Text(...) — so pasting Content in raw leaks that envelope
// into the UI. GetText unwraps it; anything it cannot parse (a tool that built
// its own Content by hand) falls back to the raw string, because a message the
// caller cannot pretty-print is still better than no message at all.
func errorText(res *Result) string {
	if text, err := GetText(res); err == nil && text != "" {
		return text
	}
	return res.Content
}
