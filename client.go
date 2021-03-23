package jsonrpc4go

import (
	"errors"
	"github.com/sunquakes/jsonrpc4go/client"
)

type ClientInterface interface {
	SetOptions(interface{})
	Call(string, interface{}, interface{}, bool) error
	BatchAppend(string, interface{}, interface{}, bool) *error
	BatchCall() error
}

func NewClient(protocol string, ip string, port string) (ClientInterface, error) {
	var err error
	switch protocol {
	case "http":
		return client.NewHttpClient(ip, port), err
	case "tcp":
		return client.NewTcpClient(ip, port)
	}
	return nil, errors.New("The protocol can not be supported")
}
