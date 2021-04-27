package common

const (
	WithoutError   = 0
	ParseError     = -32700
	InvalidRequest = -32600
	MethodNotFound = -32601
	InvalidParams  = -32602
	InternalError  = -32603
	CustomError    = -32000
)

var CodeMap = map[int]string{
	ParseError:     "Parse error",
	InvalidRequest: "Invalid request",
	MethodNotFound: "Method not found",
	InvalidParams:  "Invalid params",
	InternalError:  "Internal error",
}
