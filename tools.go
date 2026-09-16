package mcp

import (
	"webtyp.com/context"
	"webtyp.com/json"
	"webtyp.com/model"
)

type Request struct {
	Params CallToolParams
	// Action is the CRUD letter ('c','r','u','d'). It stays a byte because it is handed to
	// Validate(action byte) on the ormc-generated models — that is the boundary, and this
	// package does not drag the refactor across it.
	Action byte
}

type Tool struct {
	Name        string
	Description string
	Args        model.Fielder  // model of tool arguments (ormc-generated); nil = no args
	Resource    model.Resource // required with AccessGuarded; must be empty otherwise
	// Action es obligatoria con AccessGuarded (es la mitad del permiso que se
	// comprueba) y debe ser cero con AccessAuthenticated/AccessPublic, donde
	// no hay recurso sobre el que actuar. Ver AddTool.
	Action      model.Action
	Access      model.Access   // zero = model.AccessGuarded: identity AND permission
	Execute     func(ctx *context.Context, req Request) (*Result, error)
}

// actionByte is the CRUD letter of a single action, for the ormc Validate boundary.
func (t Tool) actionByte() byte {
	s := t.Action.String()
	if len(s) == 0 {
		return 0
	}
	return s[0]
}

// DecodableFields combines Decodable (codec) with Validate (validation).
// ormc-generated model types satisfy this interface.
type DecodableFields interface {
	model.Decodable
	Validate(action byte) error
}

func (r *Request) Bind(target DecodableFields) error {
	if err := json.Decode([]byte(r.Params.Arguments), target); err != nil {
		return err
	}
	return target.Validate(r.Action)
}

func Text(text string) *Result {
	return NewResult(TextBlock(text))
}

func Image(data []byte, mimeType string) *Result {
	return NewResult(ImageBlock(data, mimeType))
}

type ToolProvider interface {
	Tools() []Tool
}
