package jsonrpc4go

import (
	"errors"
	"github.com/sunquakes/jsonrpc4go/common"
	"github.com/sunquakes/jsonrpc4go/server"
	"golang.org/x/time/rate"
)

type Error common.Error

type ServerInterface interface {
	SetBeforeFunc(func(id any, method string, params any) error)
	SetAfterFunc(func(id any, method string, result any) error)
	SetOptions(any)
	SetRateLimit(rate.Limit, int)
	Start()
	Register(s any)
	GetEvent() <-chan int
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
