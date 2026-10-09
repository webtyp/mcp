module webtyp.com/mcp

go 1.26.8

require (
	webtyp.com/base64 v0.0.6
	webtyp.com/context v0.0.23
	webtyp.com/fetch v0.1.29
	webtyp.com/fmt v1.0.0
	webtyp.com/json v0.5.29
	webtyp.com/model v0.2.2
	webtyp.com/router v0.3.2
	webtyp.com/unixid v0.3.0
)

require webtyp.com/time v0.5.7

require (
	webtyp.com/escape v0.1.0 // indirect
	webtyp.com/filepath v0.1.0 // indirect
)

// Local dev: Encode/Decode (standard, padded base64) were just added and
// aren't in the v0.0.3 tag yet.
