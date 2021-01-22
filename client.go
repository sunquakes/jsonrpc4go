package go_jsonrpc

import (
	"errors"
	"github.com/iloveswift/go-jsonrpc/client"
)

type ClientInterface interface {
	Call(string, interface{}, interface{}, bool) error
	BatchAppend(string, interface{}, interface{}, bool) *error
	BatchCall() error
}

func NewClient(protocol string, ip string, port string) (ClientInterface, error) {
	var err error
	switch protocol {
	case "http":
		return &client.Http{
			ip,
			port,
			nil,
		}, err
	case "tcp":
		return &client.Tcp{
			ip,
			port,
			nil,
		}, err
	}
	return nil, errors.New("The protocol can not be supported")
}
