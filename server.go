package go_jsonrpc

import (
	"errors"
	"github.com/iloveswift/go-jsonrpc/server"
)

type ServerInterface interface {
	Start()
	Register(s interface{})
	SetBuffer(bs int)
}

func NewServer(protocol string, ip string, port string) (ServerInterface, error) {
	var err error
	switch protocol {
	case "http":
		return server.NewHttpServer(ip, port), err
	case "tcp":
		return server.NewTcpServer(ip, port), err
	}
	return nil, errors.New("The protocol can not be supported")
}
