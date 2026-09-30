package mcp

import "webtyp.com/model"

var rpcRequestModel = model.Definition{
	Name: "rpc_request",
	Fields: model.Fields{
		{Name: "jsonrpc", Type: model.Text()},
		{Name: "id", Type: model.Raw()},
		{Name: "method", Type: model.Text()},
		// JSON-RPC 2.0 requires params to be a structured value (object/array),
		// never a JSON string — FieldRaw embeds it inline instead of
		// double-encoding it as a quoted string the server can't decode.
		{Name: "params", Type: model.Raw()},
	},
}

var rpcResponseModel = model.Definition{
	Name: "rpc_response",
	Fields: model.Fields{
		{Name: "jsonrpc", Type: model.Text()},
		{Name: "id", Type: model.Raw()},
		{Name: "result", Type: model.Raw(), OmitEmpty: true},
		// The server's error is a nested {code,message,data} object
		// (JSONRPCErrorModel), never a plain string — FieldRaw so the
		// client can decode it instead of silently failing to match a string.
		{Name: "error", Type: model.Raw(), OmitEmpty: true},
	},
}

var jsonRPCErrorModel = model.Definition{
	Name: "json_rpcerror",
	Fields: model.Fields{
		{Name: "code", Type: model.Int()},
		{Name: "message", Type: model.Text()},
		{Name: "data", Type: model.Text(), OmitEmpty: true},
	},
}

var initializeParamsModel = model.Definition{
	Name: "initialize_params",
	Fields: model.Fields{
		{Name: "protocolVersion", Type: model.Text()},
		{Name: "clientInfo", Type: model.Struct(&implementationInfoModel)},
	},
}

var implementationInfoModel = model.Definition{
	Name: "implementation_info",
	Fields: model.Fields{
		{Name: "name", Type: model.Text()},
		{Name: "version", Type: model.Text()},
	},
}

var initializeResultModel = model.Definition{
	Name: "initialize_result",
	Fields: model.Fields{
		{Name: "protocolVersion", Type: model.Text()},
		{Name: "serverInfo", Type: model.Struct(&implementationInfoModel)},
		{Name: "capabilities", Type: model.Raw(), OmitEmpty: true},
	},
}

var CallToolParamsModel = model.Definition{
	Name: "call_tool_params",
	Fields: model.Fields{
		{Name: "name", Type: model.Text()},
		{Name: "arguments", Type: model.Raw()},
	},
}

var ResultModel = model.Definition{
	Name: "result",
	Fields: model.Fields{
		{Name: "isError", Type: model.Bool(), OmitEmpty: true},
		{Name: "content", Type: model.Raw(), OmitEmpty: true},
	},
}

var TextContentModel = model.Definition{
	Name: "text_content",
	Fields: model.Fields{
		{Name: "type", Type: model.Text()},
		{Name: "text", Type: model.Text()},
	},
}

var ImageContentModel = model.Definition{
	Name: "image_content",
	Fields: model.Fields{
		{Name: "type", Type: model.Text()},
		{Name: "data", Type: model.Text()},
		{Name: "mimeType", Type: model.Text()},
	},
}

var toolEntryModel = model.Definition{
	Name: "tool_entry",
	Fields: model.Fields{
		{Name: "name", Type: model.Text()},
		{Name: "description", Type: model.Text(), OmitEmpty: true},
		{Name: "inputSchema", Type: model.Raw()},
		{Name: "annotations", Type: model.Raw(), OmitEmpty: true},
	},
}

var listToolsResultModel = model.Definition{
	Name: "list_tools_result",
	Fields: model.Fields{
		{Name: "tools", Type: model.Raw()},
		{Name: "nextCursor", Type: model.Text(), OmitEmpty: true},
	},
}

var errorResponseModel = model.Definition{
	Name: "error_response",
	Fields: model.Fields{
		{Name: "jsonrpc", Type: model.Text()},
		{Name: "id", Type: model.Raw()},
		{Name: "error", Type: model.Struct(&jsonRPCErrorModel)},
	},
}

var MetaModel = model.Definition{
	Name: "meta",
	Fields: model.Fields{
		{Name: "progressToken", Type: model.Text(), OmitEmpty: true},
	},
}

var NotificationParamsModel = model.Definition{
	Name: "notification_params",
	Fields: model.Fields{
		{Name: "meta", Type: model.Text(), OmitEmpty: true},
	},
}

var EmptyResultModel = model.Definition{
	Name: "empty_result",
	Fields: model.Fields{
		{Name: "result", Type: model.Text(), OmitEmpty: true},
	},
}

var JSONRPCRequestModel = model.Definition{
	Name: "jsonrpcrequest",
	Fields: model.Fields{
		{Name: "jsonrpc", Type: model.Text()},
		{Name: "id", Type: model.Raw()},
		{Name: "method", Type: model.Text()},
		{Name: "params", Type: model.Raw(), OmitEmpty: true},
	},
}

var JSONRPCNotificationModel = model.Definition{
	Name: "jsonrpcnotification",
	Fields: model.Fields{
		{Name: "jsonrpc", Type: model.Text()},
		{Name: "method", Type: model.Text()},
		{Name: "params", Type: model.Raw(), OmitEmpty: true},
	},
}

var JSONRPCResponseStructModel = model.Definition{
	Name: "jsonrpcresponse_struct",
	Fields: model.Fields{
		{Name: "jsonrpc", Type: model.Text()},
		{Name: "id", Type: model.Raw()},
		{Name: "result", Type: model.Raw(), OmitEmpty: true},
		{Name: "error", Type: model.Raw(), OmitEmpty: true},
	},
}

// jsonrpcMessage marks the response variants HandleMessage can return.
// The type itself is declared in model_orm.go (generated from
// JSONRPCResponseStructModel above); the marker method lives here since it
// isn't part of the generated codec.
func (r *JSONRPCResponseStruct) jsonrpcMessage() {}

var JSONRPCErrorModel = model.Definition{
	Name: "jsonrpcerror",
	Fields: model.Fields{
		{Name: "jsonrpc", Type: model.Text()},
		{Name: "id", Type: model.Raw()},
		{Name: "error", Type: model.Raw()},
	},
}

func (e *JSONRPCError) jsonrpcMessage() {}

var JSONRPCErrorDetailsModel = model.Definition{
	Name: "jsonrpcerror_details",
	Fields: model.Fields{
		{Name: "code", Type: model.Int()},
		{Name: "message", Type: model.Text()},
		{Name: "data", Type: model.Text(), OmitEmpty: true},
	},
}
