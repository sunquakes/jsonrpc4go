package server

import (
	"github.com/sunquakes/jsonrpc4go/discovery"
	"golang.org/x/time/rate"
)

const REGISTRY_RETRY_INTERVAL = 3000

type Protocol interface {
	NewServer() Server
}

type Server interface {
	SetBeforeFunc(func(id any, method string, params any) error)
	SetAfterFunc(func(id any, method string, result any) error)
	SetOptions(any)
	SetDiscovery(d discovery.Driver, hostname string)
	SetRateLimit(rate.Limit, int)
	Start()
	Register(s any)
	DiscoveryRegister(key, value interface{}) bool
	GetEvent() <-chan int
}

func NewServer[T Protocol](p T) Server {
	return p.NewServer()
}
