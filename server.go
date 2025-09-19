package jsonrpc4go

import (
	"errors"
	"strings"

	"github.com/sunquakes/jsonrpc4go/server"
)

func NewServer(protocol string, port int) (server.Server, error) {
	var p server.Protocol
	switch strings.ToLower(protocol) {
	case "http":
		p = &server.Http{Port: port}
	case "https":
		p = &server.Http{Port: port, Secure: true}
	case "tcp":
		p = &server.Tcp{Port: port}
	default:
		return nil, errors.New("the protocol can not be supported")
	}
	return server.NewServer(p), nil
}
