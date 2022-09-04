package client

import (
	"fmt"
	"net"
	"sync"
)

type Option struct {
	MinIdle   int
	MaxActive int
	MaxIdle   int
}

type Pool struct {
	Address     string
	Lock        sync.Mutex
	Option      Option
	ActiveTotal int
	Conns       chan net.Conn
}

func NewPool(ip string, port string, option Option) *Pool {
	ch := make(chan net.Conn, 10)
	var addr = fmt.Sprintf("%s:%s", ip, port)
	pool := &Pool{
		addr,
		sync.Mutex{},
		option,
		0,
		ch,
	}
	for i := 0; i < option.MinIdle; i++ {
		err := pool.NewConn()
		if err != nil {
			fmt.Errorf("Can not connect %s", addr)
		}
	}
	return pool
}

func (p *Pool) Borrow() net.Conn {
	p.Lock.Lock()
	defer p.Lock.Unlock()
	if p.ActiveTotal >= p.Option.MaxActive-p.Option.MinIdle {
		return <-p.Conns
	}
	return <-p.Conns
}

func (p Pool) Release(conn net.Conn) {
	p.Lock.Lock()
	defer p.Lock.Unlock()
	p.Conns <- conn
}

func (p Pool) NewConn() error {
	p.Lock.Lock()
	defer p.Lock.Unlock()
	conn, err := net.Dial("tcp", p.Address)
	if err == nil {
		p.ActiveTotal++
		p.Conns <- conn
	}
	return err
}
