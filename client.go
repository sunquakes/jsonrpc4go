package jsonrpc4go

import (
	"errors"
	"github.com/sunquakes/jsonrpc4go/client"
)

func NewClient(protocol string, ip string, port string) (client.Client, error) {
	var p client.Protocol
	switch protocol {
	case "http":
		p = &client.Http{ip, port}
	case "tcp":
		p = &client.Tcp{ip, port}
	default:
		return nil, errors.New("The protocol can not be supported")
	}
	return client.NewClient(p)
}
