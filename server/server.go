package server

import "golang.org/x/time/rate"

type Protocol interface {
	NewServer() Server
}

type Server interface {
	SetBeforeFunc(func(id any, method string, params any) error)
	SetAfterFunc(func(id any, method string, result any) error)
	SetOptions(any)
	SetRateLimit(rate.Limit, int)
	Start()
	Register(s any)
	GetEvent() <-chan int
}

func NewServer[T Protocol](p T) Server {
	return p.NewServer()
}
