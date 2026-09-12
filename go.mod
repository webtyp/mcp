module webtyp.com/mcp

go 1.25.2

require (
	webtyp.com/base64 v0.0.6
	webtyp.com/context v0.0.23
	webtyp.com/fetch v0.1.28
	webtyp.com/fmt v1.0.0
	webtyp.com/json v0.5.25
	webtyp.com/model v0.1.8
	webtyp.com/router v0.1.37
	webtyp.com/unixid v0.2.28
)

require webtyp.com/time v0.5.5

// Local dev: Encode/Decode (standard, padded base64) were just added and
// aren't in the v0.0.3 tag yet.
