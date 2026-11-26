package jsonrpc4go

import (
	"errors"
	"strings"

	"github.com/sunquakes/jsonrpc4go/v2/server"
)

/**
 * @Description: Create a new JSON-RPC server
 * @Param protocol: Protocol type (http/https/tcp/streamable-http)
 * @Param port: Port number
 * @Return server.Server: Server interface
 * @Return error: Error message
 */
func NewServer(protocol string, port int) (server.Server, error) {
	var p server.Protocol
	switch strings.ToLower(protocol) {
	case "http":
		p = &server.Http{Port: port}
	case "https":
		p = &server.Http{Port: port, Secure: true}
	case "tcp":
		p = &server.Tcp{Port: port}
	case "streamable-http":
		p = &server.StreamableHttp{Port: port}
	default:
		return nil, errors.New("the protocol can not be supported")
	}
	return server.NewServer(p), nil
}
