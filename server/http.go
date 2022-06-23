package server

import (
	"fmt"
	"github.com/sunquakes/jsonrpc4go/common"
	"golang.org/x/time/rate"
	"io/ioutil"
	"log"
	"net/http"
	"sync"
)

type Http struct {
	Ip   string
	Port string
}

type HttpServer struct {
	Ip      string
	Port    string
	Server  common.Server
	Options HttpOptions
	Event   chan int
}

type HttpOptions struct {
}

func (p *Http) NewServer() Server {
	options := HttpOptions{}
	return &HttpServer{
		p.Ip,
		p.Port,
		common.Server{
			sync.Map{},
			common.Hooks{},
			nil,
		},
		options,
		make(chan int, 1),
	}
}

func (s *HttpServer) Start() {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleFunc)
	var url = fmt.Sprintf("%s:%s", s.Ip, s.Port)
	log.Printf("Listening http://%s:%s", s.Ip, s.Port)
	s.Event <- 0
	err := http.ListenAndServe(url, mux)
	if err != nil {
		log.Panic(err.Error())
	}
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
