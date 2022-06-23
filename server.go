package jsonrpc4go

import (
	"errors"
	"github.com/sunquakes/jsonrpc4go/server"
)

func NewServer(protocol string, ip string, port string) (server.Server, error) {
	var p server.Protocol
	switch protocol {
	case "http":
		p = &server.Http{ip, port}
	case "tcp":
		p = &server.Tcp{ip, port}
	default:
		return nil, errors.New("The protocol can not be supported")
	}
	return server.NewServer(p), nil
}
