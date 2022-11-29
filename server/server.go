package server

import (
	"github.com/sunquakes/jsonrpc4go/discovery"
	"golang.org/x/time/rate"
)

type Protocol interface {
	NewServer() Server
}

type Server interface {
	SetBeforeFunc(func(id any, method string, params any) error)
	SetAfterFunc(func(id any, method string, result any) error)
	SetOptions(any)
	SetRegister(d discovery.Driver)
	SetRateLimit(rate.Limit, int)
	Start()
	Register(s any)
	GetEvent() <-chan int
}

func NewServer[T Protocol](p T) Server {
	return p.NewServer()
}
