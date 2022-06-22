package jsonrpc4go

import (
	"errors"
	"github.com/sunquakes/jsonrpc4go/client"
)

func NewClient(protocol string, ip string, port string) (client.Client, error) {
	switch protocol {
	case "http":
		return client.NewClinet(&client.Http{ip, port})
	case "tcp":
		return client.NewClinet(&client.Tcp{ip, port})
	default:
		return nil, errors.New("The protocol can not be supported")
	}
}
