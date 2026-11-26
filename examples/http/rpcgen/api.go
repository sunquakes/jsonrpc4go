package main

//go:generate go run github.com/sunquakes/jsonrpc4go/v2/cmd/rpcgen -iface IntRpcApi -service IntRpc -out intrpc_rpc.go

// Result mirrors the value returned by IntRpc.Add2 on the server.
type Result struct {
	C int `json:"c"`
}

// IntRpcApi describes the typed client surface of the IntRpc service served
// by examples/http/server. Run `go generate ./...` to produce intrpc_rpc.go.
type IntRpcApi interface {
	Add(a, b int) (int, error)
	Add2(a, b int) (Result, error)
}
