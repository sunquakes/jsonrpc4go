package client

import (
	"fmt"
	"net"
	"sync"
)

type Option struct {
	MaxIdle   int
	MaxActive int
	MinIdle   int
}

type Pool struct {
	Lock      sync.Mutex
	Config    Option
	ConnTotal int
	Conns     chan net.Conn
}

func NewPool(ip string, port string, option Option) *Pool {
	ch := make(chan net.Conn, 10)
	var addr = fmt.Sprintf("%s:%s", ip, port)
	for i := 0; i < option.MinIdle; i++ {
		conn, err := net.Dial("tcp", addr)
		if err == nil {
			ch <- conn
		}
	}
	return &Pool{
		sync.Mutex{},
		option,
		0,
		ch,
	}
}

func (p *Pool) Borrow() net.Conn {
	return <-p.Conns
}

func (p Pool) Release(conn net.Conn) {
	p.Conns <- conn
}
