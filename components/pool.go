package components

import (
	"net"
	"sync"
)

type Pool struct {
	mu      sync.Mutex
	minConn int
	maxConn int
	numConn int
	conns   chan *net.Conn
	close   bool
}

func NewPool(min, max int) *Pool {
	p := &Pool{
		minConn: min,
		maxConn: max,
		numConn: min,
		conns:   make(chan *net.Conn, max),
		close:   false,
	}
	for i := 0; i < min; i++ {
	}
	return p
}
