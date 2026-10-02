package mcp

import (
	"webtyp.com/context"
	"webtyp.com/fetch"
	"webtyp.com/fmt"
	"webtyp.com/json"
	"webtyp.com/model"
)

const (
	headerAuthorization = "Authorization"
	bearerPrefix        = "Bearer "
)

type Client struct {
	endpoint  string
	authToken string  // when non-empty, sent as "Authorization: Bearer <token>"
	local     *Server // non-nil: requests go to this server in this process, not over HTTP
	userID    string  // with local: the identity the server's tools see (CtxKeyUserID)
}

// NewLocalClient talks to s in this process: the same requests and answers as NewClient, without
// HTTP. Tools see userID as the caller, the identity an HTTP request takes from its session. Use
// it where the tools live next to their caller: a Web Worker that holds the application's modules,
// a demo without a server, a test.
func NewLocalClient(s *Server, userID string) *Client {
	return &Client{local: s, userID: userID}
}

// handleLocal answers body with the local server, as the HTTP transport would.
func (c *Client) handleLocal(body []byte) ([]byte, error) {
	ctx := context.Background()
	ctx.Set(CtxKeyUserID, c.userID)
	out, known, err := encodeResponse(c.local.HandleMessage(ctx, body))
	if !known {
		return nil, fmt.Err(errUnknownResponse)
	}
	if err != nil {
		return nil, err
	}
	return readEnvelope([]byte(out))
}

const errUnknownResponse = "mcp: unknown response type"

// NewClient targets baseURL + "/mcp". authToken is sent as a Bearer token on
// every request; pass "" for open/unauthenticated daemons.
func NewClient(baseURL, authToken string) *Client {
	return &Client{
		endpoint:  fmt.Convert(baseURL).TrimSuffix("/").String() + "/mcp",
		authToken: authToken,
	}
}

// newPost builds the POST request for this client, attaching the Authorization
// header when a token is configured.
func (c *Client) newPost(body []byte) *fetch.Request {
	r := fetch.Post(c.endpoint).ContentTypeJSON().Body(body)
	if c.authToken != "" {
		r = r.Header(headerAuthorization, bearerPrefix+c.authToken)
	}
	return r
}

func (c *Client) Call(ctx *context.Context, method string, params any, callback func([]byte, error)) {
	body := c.buildBody(method, params)
	if body == nil {
		if callback != nil {
			callback(nil, fmt.Err("mcp: failed to encode request"))
		}
		return
	}
	if c.local != nil {
		result, err := c.handleLocal(body)
		if callback != nil {
			callback(result, err)
		}
		return
	}
	c.newPost(body).Send(func(resp *fetch.Response, err error) {
		if err != nil {
			if callback != nil {
				callback(nil, err)
			}
			return
		}
		if callback == nil {
			return
		}
		callback(readEnvelope(resp.Body()))
	})
}

// readEnvelope returns the result of a JSON-RPC response, nil when it has none, or its error.
func readEnvelope(body []byte) ([]byte, error) {
	var envelope rpcResponse
	if err := json.Decode(body, &envelope); err != nil {
		return nil, err
	}
	if len(envelope.Error) != 0 {
		return nil, fmt.Err("mcp: " + string(envelope.Error))
	}
	if len(envelope.Result) == 0 {
		return nil, nil
	}
	return []byte(envelope.Result), nil
}

func (c *Client) Dispatch(ctx *context.Context, method string, params any) {
	body := c.buildBody(method, params)
	if body == nil {
		return
	}
	if c.local != nil {
		_, _ = c.handleLocal(body)
		return
	}
	c.newPost(body).Send(func(*fetch.Response, error) {})
}

func (c *Client) buildBody(method string, params any) []byte {
	var paramsJSON string
	if params != nil {
		if f, ok := params.(model.Encodable); ok {
			if err := json.Encode(f, &paramsJSON); err != nil {
				return nil
			}
		}
	}
	req := rpcRequest{Jsonrpc: "2.0", Id: RequestId("\"1\""), Method: method, Params: paramsJSON}
	var body []byte
	if err := json.Encode(&req, &body); err != nil {
		return nil
	}
	return body
}
