package client

import (
	"errors"
	"fmt"
	"github.com/sunquakes/jsonrpc4go/discovery"
	"net"
	"strings"
	"sync"
)

type PoolOptions struct {
	MinIdle   int
	MaxActive int
}

type Pool struct {
	Name              string
	Discovery         discovery.Driver
	Address           string
	ActiveAddressList []string
	Lock              sync.Mutex
	Options           PoolOptions
	ActiveTotal       int
	Conns             chan net.Conn
}

func NewPool(name, address string, dc discovery.Driver, option PoolOptions) *Pool {
	ch := make(chan net.Conn, option.MaxActive)
	pool := &Pool{
		name,
		dc,
		address,
		nil,
		sync.Mutex{},
		option,
		0,
		ch,
	}
	pool.ActiveAddress()
	pool.Lock.Lock()
	defer pool.Lock.Unlock()
	for i := 0; i < option.MinIdle; i++ {
		conn, err := pool.Create()
		if err == nil {
			pool.ActiveTotal++
			pool.Conns <- conn
		}
	}
	return pool
}

func (p *Pool) ActiveAddress() (int, error) {
	var (
		address string
		err     error
	)
	if p.Discovery != nil {
		address, err = p.Discovery.Get(p.Name)
		if err != nil {
			return 0, err
		}
	} else {
		address = p.Address
	}
	addressList := strings.Split(address, ",")
	p.ActiveAddressList = addressList
	return len(addressList), nil
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
	conn, err := p.Create()
	if err == nil {
		p.ActiveTotal++
	}
	return conn, err
}

func (p *Pool) Release(conn net.Conn) {
	p.Lock.Lock()
	defer p.Lock.Unlock()
	p.Conns <- conn
}

func (p *Pool) Create() (net.Conn, error) {
	var err error
	size := len(p.ActiveAddressList)
	if size == 0 {
		size, err = p.ActiveAddress()
		if err != nil {
			return nil, err
		}
	}
	key := p.ActiveTotal % size
	address := p.ActiveAddressList[key]
	conn, err := p.Connect(address)
	if err != nil {
		p.ActiveAddressList = append(p.ActiveAddressList[:key], p.ActiveAddressList[key+1:]...)
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
	// When disconnected, reconnect instead of fetch from pool.
	conn, err := p.Create()
	if err == nil {
		p.ActiveTotal++
	}
	return conn, err
}

func (p *Pool) Remove(conn net.Conn) {
	p.Lock.Lock()
	defer p.Lock.Unlock()
	if conn != nil {
		p.ActiveTotal--
	}
}

func (p *Pool) SetOptions(options PoolOptions) {
	p.Options = options
}
