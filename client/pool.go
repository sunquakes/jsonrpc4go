package client

import (
	"errors"
	"log"
	"net"
	"strings"
	"sync"

	"slices"

	"github.com/sunquakes/jsonrpc4go/discovery"
)

/**
 * @Description: Connection pool options structure
 * @Field MinIdle: Minimum number of idle connections
 * @Field MaxActive: Maximum number of active connections
 */
type PoolOptions struct {
	MinIdle   int
	MaxActive int
}

/**
 * @Description: Main connection pool structure
 * @Field Name: Service name
 * @Field Discovery: Service discovery driver
 * @Field Address: Service address
 * @Field ActiveAddressList: Active address list
 * @Field Lock: Mutex lock
 * @Field Options: Connection pool options
 * @Field ActiveTotal: Total number of active connections
 * @Field Conns: Connection channel
 */
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

/**
 * @Description: Create a new connection pool instance
 * @Param name: Service name
 * @Param address: Service address
 * @Param dc: Service discovery driver
 * @Param option: Connection pool options
 * @Return *Pool: Connection pool instance pointer
 */
func NewPool(name, address string, dc discovery.Driver, option PoolOptions) *Pool {
	ch := make(chan net.Conn, option.MaxActive)
	pool := &Pool{
		Name:              name,
		Discovery:         dc,
		Address:           address,
		ActiveAddressList: nil,
		Lock:              sync.Mutex{},
		Options:           option,
		ActiveTotal:       0,
		Conns:             ch,
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

/**
 * @Description: Get active address list
 * @Receiver p: Pool structure pointer
 * @Return int: Number of active addresses
 * @Return error: Error message
 */
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

/**
 * @Description: Get connection from connection pool
 * @Receiver p: Pool structure pointer
 * @Return net.Conn: Network connection
 * @Return error: Error message
 */
func (p *Pool) Borrow() (net.Conn, error) {
	p.Lock.Lock()
	defer p.Lock.Unlock()
	if p.ActiveTotal <= 0 {
		return nil, errors.New("unable to connect to the server")
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

/**
 * @Description: Release connection back to connection pool
 * @Receiver p: Pool structure pointer
 * @Param conn: Network connection
 */
func (p *Pool) Release(conn net.Conn) {
	p.Lock.Lock()
	defer p.Lock.Unlock()
	p.Conns <- conn
}

/**
 * @Description: Create new connection
 * @Receiver p: Pool structure pointer
 * @Return net.Conn: Network connection
 * @Return error: Error message
 */
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
		p.ActiveAddressList = slices.Delete(p.ActiveAddressList, key, key+1)
		log.Printf("Can not connect %s", address)
	}
	return conn, err
}

/**
 * @Description: Connect to specified address
 * @Receiver p: Pool structure pointer
 * @Param address: Service address
 * @Return net.Conn: Network connection
 * @Return error: Error message
 */
func (p *Pool) Connect(address string) (net.Conn, error) {
	return net.Dial("tcp", address)
}

/**
 * @Description: Get new connection after removing old one
 * @Receiver p: Pool structure pointer
 * @Param conn: Old connection
 * @Return net.Conn: New connection
 * @Return error: Error message
 */
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

/**
 * @Description: Remove connection
 * @Receiver p: Pool structure pointer
 * @Param conn: Connection to remove
 */
func (p *Pool) Remove(conn net.Conn) {
	p.Lock.Lock()
	defer p.Lock.Unlock()
	if conn != nil {
		p.ActiveTotal--
	}
}

/**
 * @Description: Set connection pool options
 * @Receiver p: Pool structure pointer
 * @Param options: Connection pool options
 */
func (p *Pool) SetOptions(options PoolOptions) {
	p.Options = options
}
