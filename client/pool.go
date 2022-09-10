package client

import (
	"errors"
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

func (p *Pool) Borrow() (net.Conn, error) {
	p.Lock.Lock()
	defer p.Lock.Unlock()
	if p.ActiveTotal <= 0 {
		return nil, errors.New("Unable to connect to the server.")
	}
	if p.ActiveTotal >= p.Options.MaxActive {
		return <-p.Conns, nil
	}
	p.Create()
	return <-p.Conns, nil
}

func (p *Pool) Release(conn net.Conn) {
	p.Lock.Lock()
	defer p.Lock.Unlock()
	p.Conns <- conn
}

func (p *Pool) Create() error {
	conn, err := p.Connect()
	if err == nil {
		p.ActiveTotal++
		p.Conns <- conn
	}
	return err
}

func (p *Pool) Connect() (net.Conn, error) {
	return net.Dial("tcp", p.Address)
}

func (p *Pool) BorrowAfterRemove(conn net.Conn) (net.Conn, error) {
	p.Lock.Lock()
	defer p.Lock.Unlock()
	return p.Connect()
}

func (p *Pool) Remove(conn net.Conn) {
	p.ActiveTotal--
}

func (p *Pool) SetOptions(options PoolOptions) {
	p.Options = options
}
