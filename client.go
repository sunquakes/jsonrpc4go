package jsonrpc4go

import (
	"errors"
	"github.com/sunquakes/jsonrpc4go/client"
	"github.com/sunquakes/jsonrpc4go/discovery"
	"reflect"
)

func NewClient(name string, protocol string, server any) (client.Client, error) {
	var p client.Protocol
	var (
		address string
		dc      discovery.Driver
	)
	if reflect.TypeOf(server).Kind() == reflect.String {
		address = server.(string)
	} else {
		dc = server.(discovery.Driver)
	}

	switch protocol {
	case "http":
		p = &client.Http{name, protocol, address, dc}
	case "tcp":
		p = &client.Tcp{name, protocol, address, dc}
	default:
		return nil, errors.New("The protocol can not be supported")
	}
	return client.NewClient(p), nil
}
