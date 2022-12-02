package jsonrpc4go

import (
	"errors"
	"github.com/sunquakes/jsonrpc4go/client"
	"github.com/sunquakes/jsonrpc4go/discovery"
	"reflect"
)

func NewClient(name string, protocol string, nodes any) (client.Client, error) {
	var p client.Protocol
	var (
		address   string
		registrar discovery.Driver
	)
	if reflect.TypeOf(nodes).Kind() == reflect.String {
		address = nodes.(string)
	} else {
		registrar = nodes.(discovery.Driver)
	}

	switch protocol {
	case "http":
		p = &client.Http{name, protocol, address, registrar}
	case "tcp":
		p = &client.Tcp{name, protocol, address, registrar}
	default:
		return nil, errors.New("The protocol can not be supported")
	}
	return client.NewClient(p), nil
}
