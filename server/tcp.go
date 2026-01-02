package server

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	"github.com/sunquakes/jsonrpc4go/common"
	"github.com/sunquakes/jsonrpc4go/discovery"
	"golang.org/x/time/rate"
)

/*
 * Tcp represents the TCP protocol implementation
 * @property Port - The port to listen on
 */
type Tcp struct {
	Port int
}

/*
 * TcpServer represents the TCP server implementation
 * @property Hostname - The hostname of the server
 * @property Port - The port to listen on
 * @property Server - The common server implementation
 * @property Options - The TCP server options
 * @property Event - The event channel for server notifications
 * @property Discovery - The service discovery driver
 */
type TcpServer struct {
	Hostname  string
	Port      int
	Server    common.Server
	Options   TcpOptions
	Event     chan int
	Discovery discovery.Driver
}

/*
 * TcpOptions represents the options for the TCP server
 * @property PackageEof - The end-of-file marker for packages
 * @property PackageMaxLength - The maximum length of a package
 */
type TcpOptions struct {
	PackageEof       string
	PackageMaxLength int64
}

/*
 * NewServer creates a new TCP server
 * @return Server - The new TCP server
 */
func (p *Tcp) NewServer() Server {
	options := TcpOptions{
		PackageEof:       "\r\n",
		PackageMaxLength: 1024 * 1024 * 2,
	}
	return &TcpServer{
		"",
		p.Port,
		common.Server{
			Sm:          sync.Map{},
			Hooks:       common.Hooks{},
			RateLimiter: nil,
		},
		options,
		make(chan int, 1),
		nil,
	}
}

/*
 * Start starts the TCP server
 */
func (s *TcpServer) Start() {
	// Register services
	if s.Discovery != nil {
		register := func(key, value interface{}) bool {
			go s.DiscoveryRegister(key, value)
			return true
		}
		s.Server.Sm.Range(register)
	}
	// Start the server
	var addr = fmt.Sprintf("0.0.0.0:%d", s.Port)
	tcpAddr, err := net.ResolveTCPAddr("tcp", addr)
	if err != nil {
		log.Panic(err.Error())
	}
	listener, _ := net.ListenTCP("tcp", tcpAddr)
	log.Printf("Listening tcp://0.0.0.0:%d", s.Port)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	// Notify successful start: send 0 to the Event channel after 1 second to indicate the service is ready
	go func() {
		time.Sleep(time.Second)
		select {
		case s.Event <- 0:
		default:
			// Drop if the channel is full to avoid blocking
		}
	}()
	for {
		conn, err := listener.AcceptTCP()
		if err != nil {
			log.Panic(err.Error())
		}
		go s.handleFunc(ctx, conn)
	}
}

/*
 * DiscoveryRegister registers a service to the discovery service
 * @param key - The service key
 * @param value - The service value
 * @return bool - True if registration succeeded
 */
func (s *TcpServer) DiscoveryRegister(key, value interface{}) bool {
	err := s.Discovery.Register(key.(string), "tcp", s.Hostname, s.Port)
	if err == nil {
		return true
	}
	time.Sleep(REGISTRY_RETRY_INTERVAL * time.Millisecond)
	s.DiscoveryRegister(key, value)
	return false
}

/*
 * Register registers a service
 * @param m - The service to register
 */
func (s *TcpServer) Register(m any) {
	s.Server.Register(m)
}

/*
 * SetOptions sets the TCP server options
 * @param tcpOptions - The TCP server options
 */
func (s *TcpServer) SetOptions(tcpOptions any) {
	s.Options = tcpOptions.(TcpOptions)
}

/*
 * SetDiscovery sets the service discovery driver
 * @param d - The service discovery driver
 * @param hostname - The hostname to register
 */
func (s *TcpServer) SetDiscovery(d discovery.Driver, hostname string) {
	s.Discovery = d
	s.Hostname = hostname
	var err error
	if s.Hostname == "" {
		s.Hostname, err = GetHostname()
		if err != nil {
			common.Debug(err.Error())
		}
	}
}

/*
 * SetRateLimit sets the rate limiter
 * @param r - The rate limit
 * @param b - The burst limit
 */
func (s *TcpServer) SetRateLimit(r rate.Limit, b int) {
	s.Server.RateLimiter = rate.NewLimiter(r, b)
}

/*
 * SetBeforeFunc sets the before function
 * @param beforeFunc - The before function
 */
func (s *TcpServer) SetBeforeFunc(beforeFunc func(id any, method string, params any) error) {
	s.Server.Hooks.BeforeFunc = beforeFunc
}

/*
 * SetAfterFunc sets the after function
 * @param afterFunc - The after function
 */
func (s *TcpServer) SetAfterFunc(afterFunc func(id any, method string, result any) error) {
	s.Server.Hooks.AfterFunc = afterFunc
}

/*
 * GetEvent returns the event channel
 * @return <-chan int - The event channel
 */
func (s *TcpServer) GetEvent() <-chan int {
	return s.Event
}

/*
 * handleFunc handles incoming TCP connections
 * @param ctx - The context for the connection
 * @param conn - The TCP connection
 */
func (s *TcpServer) handleFunc(ctx context.Context, conn net.Conn) {
	defer conn.Close()
	select {
	case <-ctx.Done():
		return
	default:
		//	do nothing
	}
	eofb := []byte(s.Options.PackageEof)
	eofl := len(eofb)
	for {
		var (
			data []byte
		)
		l := 0
		for {
			var buf = make([]byte, s.Options.PackageMaxLength)
			n, err := conn.Read(buf)
			if err != nil {
				if n == 0 {
					return
				}
				common.Debug(err.Error())
			}
			l += n
			data = append(data, buf[:n]...)
			if bytes.Equal(data[l-eofl:], eofb) {
				break
			}
		}
		res := s.Server.Handler(data[:l-eofl])
		res = append(res, eofb...)
		conn.Write(res)
	}
}
