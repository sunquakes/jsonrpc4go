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
	AddressList []string
	Lock        sync.Mutex
	Options     PoolOptions
	ActiveTotal int
	Conns       chan net.Conn
}

func NewPool(addressList []string, option PoolOptions) *Pool {
	ch := make(chan net.Conn, option.MaxActive)
	pool := &Pool{
		addressList,
		sync.Mutex{},
		option,
		0,
		ch,
	}
	for i := 0; i < option.MinIdle; i++ {
		pool.Create()
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

func (p *Pool) Create() (net.Conn, error) {
	size := len(p.AddressList)
	key := p.ActiveTotal % size
	address := p.AddressList[key]
	conn, err := p.Connect(address)
	if err == nil {
		p.ActiveTotal++
		p.Conns <- conn
	} else {
		p.AddressList = append(p.AddressList[:key], p.AddressList[key+1:]...)
		fmt.Errorf("Can not connect %s", address)
	}
	return conn, err
}

func (p *Pool) Connect(address string) (net.Conn, error) {
	return net.Dial("tcp", address)
}

func (p *Pool) BorrowAfterRemove(conn net.Conn) (net.Conn, error) {
	p.Lock.Lock()
	defer p.Lock.Unlock()
	if conn != nil {
		p.ActiveTotal--
	}
	conn, err := p.Create()
	if err == nil {
		p.ActiveTotal++
	}
	return conn, err
}

func (p *Pool) Remove(conn net.Conn) {
	if conn != nil {
		p.ActiveTotal--
	}
}

func (p *Pool) SetOptions(options PoolOptions) {
	p.Options = options
}
