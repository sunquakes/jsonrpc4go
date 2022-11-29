package jsonrpc4go

import (
	"errors"
	"github.com/sunquakes/jsonrpc4go/server"
)

func NewServer(protocol string, hostname string, port int) (server.Server, error) {
	var p server.Protocol
	switch protocol {
	case "http":
		p = &server.Http{hostname, port}
	case "tcp":
		p = &server.Tcp{hostname, port}
	default:
		return nil, errors.New("The protocol can not be supported")
	}
	return server.NewServer(p), nil
}
