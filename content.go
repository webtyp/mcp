package mcp

import (
	"webtyp.com/base64"
	"webtyp.com/json"
	"webtyp.com/model"
)

// ContentBlock is one item of an MCP result's content array. TextContent and
// ImageContent (ormc-generated in model_orm.go) satisfy it automatically —
// both already implement Fielder + Encodable.
type ContentBlock interface {
	model.Fielder
	model.Encodable
}

// contentBlockList is the wire-encoding adapter for Result.Content: the MCP
// spec's content array is heterogeneous by definition, so this holds mixed
// ContentBlock values instead of routing through an ormc list type (which is
// generated homogeneous, one struct per list).
type contentBlockList []ContentBlock

func (s *contentBlockList) Len() int               { return len(*s) }
func (s *contentBlockList) At(i int) model.Fielder { return (*s)[i] }
func (s *contentBlockList) Append() model.Fielder  { panic("mcp: contentBlockList is encode-only") }
func (s *contentBlockList) IsNil() bool            { return s == nil }
func (s *contentBlockList) EncodeFields(_ model.FieldWriter) {}
func (s *contentBlockList) DecodeFields(_ model.FieldReader) {}

// NewResult is the one construction path for Result.Content, whether it
// carries one block or several mixed types.
func NewResult(blocks ...ContentBlock) *Result {
	list := contentBlockList(blocks)
	var s string
	_ = json.Encode(&list, &s)
	return &Result{Content: s}
}

func TextBlock(text string) ContentBlock {
	return &TextContent{Type: "text", Text: text}
}

// ImageBlock base64-encodes data itself: FieldWriter.Bytes() does NOT
// base64-encode (it JSON-escapes raw bytes as a string), which is not what
// the MCP spec wants for binary image data, so ImageContent.Data must arrive
// already base64-encoded.
func ImageBlock(data []byte, mimeType string) ContentBlock {
	return &ImageContent{Type: "image", Data: base64.Encode(data), MimeType: mimeType}
}
