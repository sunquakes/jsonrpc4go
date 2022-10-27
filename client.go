package jsonrpc4go

import (
	"errors"
	"github.com/sunquakes/jsonrpc4go/client"
	"strings"
)

func NewClient(protocol string, address string) (client.Client, error) {
	var p client.Protocol
	addressList := strings.Split(address, ",")
	switch protocol {
	case "http":
		p = &client.Http{protocol, addressList}
	case "tcp":
		p = &client.Tcp{protocol, addressList}
	default:
		return nil, errors.New("The protocol can not be supported")
	}
	return client.NewClient(p), nil
}
