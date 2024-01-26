package server

import (
	"errors"
	"fmt"
	"github.com/sunquakes/jsonrpc4go/common"
	"github.com/sunquakes/jsonrpc4go/discovery"
	"golang.org/x/time/rate"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"sync"
	"time"
)

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
		return hostname, errors.New("Failed to get hostname.")
	}
	return hostname, nil
}

type Http struct {
	Port int
}

type HttpServer struct {
	Hostname  string
	Port      int
	Server    common.Server
	Options   HttpOptions
	Event     chan int
	Discovery discovery.Driver
}

type HttpOptions struct {
}

func (p *Http) NewServer() Server {
	options := HttpOptions{}
	return &HttpServer{
		"",
		p.Port,
		common.Server{
			sync.Map{},
			common.Hooks{},
			nil,
		},
		options,
		make(chan int, 1),
		nil,
	}
}

func (s *HttpServer) Start() {
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
	log.Printf("Listening http://0.0.0.0:%d", s.Port)
	s.Event <- 0
	err := http.ListenAndServe(url, mux)
	if err != nil {
		log.Panic(err.Error())
	}
}

func (s *HttpServer) DiscoveryRegister(key, value interface{}) bool {
	err := s.Discovery.Register(key.(string), "http", s.Hostname, s.Port)
	if err == nil {
		return true
	}
	time.Sleep(REGISTRY_RETRY_INTERVAL * time.Millisecond)
	s.DiscoveryRegister(key, value)
	return false
}

func (s *HttpServer) Register(m any) {
	err := s.Server.Register(m)
	if err != nil {
		log.Panic(err.Error())
	}
}

func (s *HttpServer) SetOptions(httpOptions any) {
	s.Options = httpOptions.(HttpOptions)
}

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

func (s *HttpServer) SetRateLimit(r rate.Limit, b int) {
	s.Server.RateLimiter = rate.NewLimiter(r, b)
}

func (s *HttpServer) SetBeforeFunc(beforeFunc func(id any, method string, params any) error) {
	s.Server.Hooks.BeforeFunc = beforeFunc
}

func (s *HttpServer) SetAfterFunc(afterFunc func(id any, method string, result any) error) {
	s.Server.Hooks.AfterFunc = afterFunc
}

func (s *HttpServer) GetEvent() <-chan int {
	return s.Event
}

func (s *HttpServer) handleFunc(w http.ResponseWriter, r *http.Request) {
	var (
		err  error
		data []byte
	)
	w.Header().Set("Content-Type", "application/json")
	if data, err = ioutil.ReadAll(r.Body); err != nil {
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
