package client

import (
	"errors"
	"log"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/sunquakes/jsonrpc4go/discovery"
)

/**
 * @Description: Default number of idle connections
 */
const DEFAULT_MIN_IDLE = 5

/**
 * @Description: Default maximum number of active connections
 */
const DEFAULT_MAX_ACTIVE = 5

/**
 * @Description: Connection timeout
 */
const CONNECT_TIMEOUT = 3 * time.Second

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
 * @Field ActiveTotal: Total number of open connections
 * @Field Conns: Idle connection channel
 * @Field slots: Bounds the number of connections that are currently borrowed
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
	slots             chan struct{}
	next              uint64
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
	if option.MaxActive <= 0 {
		option.MaxActive = DEFAULT_MAX_ACTIVE
	}
	if option.MinIdle < 0 {
		option.MinIdle = 0
	}
	if option.MinIdle > option.MaxActive {
		option.MinIdle = option.MaxActive
	}
	pool := &Pool{
		Name:        name,
		Discovery:   dc,
		Address:     address,
		Options:     option,
		ActiveTotal: 0,
		Conns:       make(chan net.Conn, option.MaxActive),
		slots:       make(chan struct{}, option.MaxActive),
	}
	_, _ = pool.ActiveAddress()
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
	p.Lock.Lock()
	p.ActiveAddressList = addressList
	p.Lock.Unlock()
	return len(addressList), nil
}

/**
 * @Description: Get connection from connection pool
 * @Receiver p: Pool structure pointer
 * @Return net.Conn: Network connection
 * @Return error: Error message
 */
func (p *Pool) Borrow() (net.Conn, error) {
	// Wait until the pool has capacity for another borrowed connection,
	// without holding the lock so that releases are never blocked.
	p.slots <- struct{}{}

	// Prefer reusing an idle connection.
	select {
	case conn := <-p.Conns:
		return conn, nil
	default:
	}

	// Create the connection without holding the lock, since dialing blocks.
	conn, err := p.Create()
	if err != nil {
		<-p.slots
		return nil, err
	}
	p.Lock.Lock()
	p.ActiveTotal++
	p.Lock.Unlock()
	return conn, nil
}

/**
 * @Description: Release connection back to connection pool
 * @Receiver p: Pool structure pointer
 * @Param conn: Network connection
 */
func (p *Pool) Release(conn net.Conn) {
	if conn == nil {
		return
	}
	select {
	case p.Conns <- conn:
	default:
		// The pool is full; discard the connection instead of blocking.
		_ = conn.Close()
		p.Lock.Lock()
		if p.ActiveTotal > 0 {
			p.ActiveTotal--
		}
		p.Lock.Unlock()
	}
	<-p.slots
}

/**
 * @Description: Create new connection
 * @Receiver p: Pool structure pointer
 * @Return net.Conn: Network connection
 * @Return error: Error message
 */
func (p *Pool) Create() (net.Conn, error) {
	p.Lock.Lock()
	addresses := p.ActiveAddressList
	if len(addresses) == 0 {
		p.Lock.Unlock()
		if _, err := p.ActiveAddress(); err != nil {
			return nil, err
		}
		p.Lock.Lock()
		addresses = p.ActiveAddressList
	}
	if len(addresses) == 0 {
		p.Lock.Unlock()
		return nil, errors.New("no available service address")
	}
	start := int(p.next % uint64(len(addresses)))
	p.next++
	p.Lock.Unlock()

	// Try each address in turn to survive individual node failures.
	var lastErr error
	for i := 0; i < len(addresses); i++ {
		address := addresses[(start+i)%len(addresses)]
		conn, err := p.Connect(address)
		if err == nil {
			return conn, nil
		}
		lastErr = err
		log.Printf("Can not connect %s", address)
	}
	return nil, lastErr
}

/**
 * @Description: Connect to specified address
 * @Receiver p: Pool structure pointer
 * @Param address: Service address
 * @Return net.Conn: Network connection
 * @Return error: Error message
 */
func (p *Pool) Connect(address string) (net.Conn, error) {
	return net.DialTimeout("tcp", address, CONNECT_TIMEOUT)
}

/**
 * @Description: Get new connection after removing old one
 * @Receiver p: Pool structure pointer
 * @Param conn: Old connection
 * @Return net.Conn: New connection
 * @Return error: Error message
 */
func (p *Pool) BorrowAfterRemove(conn net.Conn) (net.Conn, error) {
	p.Remove(conn)
	return p.Borrow()
}

/**
 * @Description: Remove connection
 * @Receiver p: Pool structure pointer
 * @Param conn: Connection to remove
 */
func (p *Pool) Remove(conn net.Conn) {
	if conn == nil {
		return
	}
	_ = conn.Close()
	p.Lock.Lock()
	if p.ActiveTotal > 0 {
		p.ActiveTotal--
	}
	p.Lock.Unlock()
	<-p.slots
}

/**
 * @Description: Set connection pool options
 * @Receiver p: Pool structure pointer
 * @Param options: Connection pool options
 */
func (p *Pool) SetOptions(options PoolOptions) {
	p.Lock.Lock()
	defer p.Lock.Unlock()
	p.Options = options
}
