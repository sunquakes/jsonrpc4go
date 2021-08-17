package jsonrpc4go

import (
	"errors"
	"github.com/sunquakes/jsonrpc4go/common"
	"github.com/sunquakes/jsonrpc4go/server"
)

type Error common.Error

type ServerInterface interface {
	SetBeforeFunc(func(id interface{}, method string, params interface{}) error)
	SetAfterFunc(func(id interface{}, method string, result interface{}) error)
	SetOptions(interface{})
	SetRateLimit(float64, int64)
	Start()
	Register(s interface{})
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
