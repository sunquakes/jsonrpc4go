package client

import (
	"fmt"
	"net"
	"sync"
)

type PoolOptions struct {
	MinIdle   int
	MaxActive int
}

type Pool struct {
	Address     string
	Lock        sync.Mutex
	Options     PoolOptions
	ActiveTotal int
	Conns       chan net.Conn
}

func NewPool(ip string, port string, option PoolOptions) *Pool {
	ch := make(chan net.Conn, option.MaxActive)
	var addr = fmt.Sprintf("%s:%s", ip, port)
	pool := &Pool{
		addr,
		sync.Mutex{},
		option,
		0,
		ch,
	}
	for i := 0; i < option.MinIdle; i++ {
		err := pool.Create()
		if err != nil {
			fmt.Errorf("Can not connect %s", addr)
		}
	}
	return pool
}

func (p *Pool) Borrow() net.Conn {
	p.Lock.Lock()
	defer p.Lock.Unlock()
	if p.ActiveTotal >= p.Options.MaxActive {
		return <-p.Conns
	}
	p.Create()
	return <-p.Conns
}

func (p *Pool) Release(conn net.Conn) {
	p.Lock.Lock()
	defer p.Lock.Unlock()
	p.Conns <- conn
}

func (p *Pool) Create() error {
	conn, err := net.Dial("tcp", p.Address)
	if err == nil {
		p.ActiveTotal++
		p.Conns <- conn
	}
	return err
}

func (p *Pool) Remove() {
	p.ActiveTotal--
}

func (p *Pool) SetOptions(options PoolOptions) {
	p.Options = options
}
