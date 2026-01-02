package server

import (
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/sunquakes/jsonrpc4go/common"
	"github.com/sunquakes/jsonrpc4go/discovery"
	"golang.org/x/time/rate"
)

/*
 * GetHostname gets the hostname of the machine
 * @return string - The hostname
 * @return error - An error if hostname could not be retrieved
 */
func GetHostname() (string, error) {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "", err
	}
	hostname := ""
	for _, address := range addrs {
		if ipnet, ok := address.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				hostname = ipnet.IP.String()
				break
			}
		}
	}
	if hostname == "" {
		return hostname, errors.New("failed to get hostname")
	}
	return hostname, nil
}

/*
 * Http represents the HTTP protocol implementation
 * @property Port - The port to listen on
 * @property Secure - Whether to use HTTPS
 */
type Http struct {
	Port   int
	Secure bool
}

/*
 * HttpServer represents the HTTP server implementation
 * @property Hostname - The hostname of the server
 * @property Port - The port to listen on
 * @property Server - The common server implementation
 * @property Options - The HTTP server options
 * @property Event - The event channel for server notifications
 * @property Discovery - The service discovery driver
 * @property Secure - Whether to use HTTPS
 */
type HttpServer struct {
	Hostname  string
	Port      int
	Server    common.Server
	Options   HttpOptions
	Event     chan int
	Discovery discovery.Driver
	Secure    bool
}

/*
 * HttpOptions represents the options for the HTTP server
 * @property CertPath - The path to the certificate file
 * @property KeyPath - The path to the key file
 */
type HttpOptions struct {
	CertPath string
	KeyPath  string
}

/*
 * NewServer creates a new HTTP server
 * @return Server - The new HTTP server
 */
func (p *Http) NewServer() Server {
	options := HttpOptions{}
	return &HttpServer{
		Hostname: "",
		Port:     p.Port,
		Server: common.Server{
			Sm:          sync.Map{},
			Hooks:       common.Hooks{},
			RateLimiter: nil,
		},
		Options:   options,
		Event:     make(chan int, 1),
		Discovery: nil,
		Secure:    p.Secure,
	}
}

/*
 * Start starts the HTTP server
 */
func (s *HttpServer) Start() {
	if s.Secure && (s.Options.CertPath == "" || s.Options.KeyPath == "") {
		log.Panic("CertPath or KeyPath is empty.")
	}
	// Register services
	if s.Discovery != nil {
		register := func(key, value interface{}) bool {
			go s.DiscoveryRegister(key, value)
			return true
		}
		s.Server.Sm.Range(register)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleFunc)
	var url = fmt.Sprintf("0.0.0.0:%d", s.Port)
	if s.Secure {
		log.Printf("Listening https://0.0.0.0:%d", s.Port)
	} else {
		log.Printf("Listening http://0.0.0.0:%d", s.Port)
	}
	// Notify successful start: send 0 to the Event channel after 1 second to indicate the service is ready
	go func() {
		time.Sleep(time.Second)
		select {
		case s.Event <- 0:
		default:
			// Drop if the channel is full to avoid blocking
		}
	}()
	var err error
	if s.Secure {
		err = http.ListenAndServeTLS(url, s.Options.CertPath, s.Options.KeyPath, mux)
	} else {
		err = http.ListenAndServe(url, mux)
	}
	if err != nil {
		log.Panic(err.Error())
	}
}

/*
 * DiscoveryRegister registers a service to the discovery service
 * @param key - The service key
 * @param value - The service value
 * @return bool - True if registration succeeded
 */
func (s *HttpServer) DiscoveryRegister(key, value interface{}) bool {
	err := s.Discovery.Register(key.(string), "http", s.Hostname, s.Port)
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
func (s *HttpServer) Register(m any) {
	err := s.Server.Register(m)
	if err != nil {
		log.Panic(err.Error())
	}
}

/*
 * SetOptions sets the HTTP server options
 * @param httpOptions - The HTTP server options
 */
func (s *HttpServer) SetOptions(httpOptions any) {
	s.Options = httpOptions.(HttpOptions)
}

/*
 * SetDiscovery sets the service discovery driver
 * @param d - The service discovery driver
 * @param hostname - The hostname to register
 */
func (s *HttpServer) SetDiscovery(d discovery.Driver, hostname string) {
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
func (s *HttpServer) SetRateLimit(r rate.Limit, b int) {
	s.Server.RateLimiter = rate.NewLimiter(r, b)
}

/*
 * SetBeforeFunc sets the before function
 * @param beforeFunc - The before function
 */
func (s *HttpServer) SetBeforeFunc(beforeFunc func(id any, method string, params any) error) {
	s.Server.Hooks.BeforeFunc = beforeFunc
}

/*
 * SetAfterFunc sets the after function
 * @param afterFunc - The after function
 */
func (s *HttpServer) SetAfterFunc(afterFunc func(id any, method string, result any) error) {
	s.Server.Hooks.AfterFunc = afterFunc
}

/*
 * GetEvent returns the event channel
 * @return <-chan int - The event channel
 */
func (s *HttpServer) GetEvent() <-chan int {
	return s.Event
}

/*
 * handleFunc handles incoming HTTP requests
 * @param w - The response writer
 * @param r - The request
 */
func (s *HttpServer) handleFunc(w http.ResponseWriter, r *http.Request) {
	var (
		err  error
		data []byte
	)
	w.Header().Set("Content-Type", "application/json")
	if data, err = io.ReadAll(r.Body); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	res := s.Server.Handler(data)
	_, err = w.Write(res)
	if err != nil {
		log.Panic(err.Error())
	}
}
