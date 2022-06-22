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
	Ip      string
	Port    string
	Server  common.Server
	Options HttpOptions
	Event   chan int
}

type HttpOptions struct {
}

func NewHttpServer(ip string, port string) *Http {
	options := HttpOptions{}
	return &Http{
		ip,
		port,
		common.Server{
			sync.Map{},
			common.Hooks{},
			nil,
		},
		options,
		make(chan int, 1),
	}
}

func (p *Http) Start() {
	mux := http.NewServeMux()
	mux.HandleFunc("/", p.handleFunc)
	var url = fmt.Sprintf("%s:%s", p.Ip, p.Port)
	log.Printf("Listening http://%s:%s", p.Ip, p.Port)
	p.Event <- 0
	err := http.ListenAndServe(url, mux)
	if err != nil {
		log.Panic(err.Error())
	}
}

func (p *Http) Register(s any) {
	err := p.Server.Register(s)
	if err != nil {
		log.Panic(err.Error())
	}
}

func (p *Http) SetOptions(httpOptions any) {
	p.Options = httpOptions.(HttpOptions)
}

func (p *Http) SetRateLimit(r rate.Limit, b int) {
	p.Server.RateLimiter = rate.NewLimiter(r, b)
}

func (p *Http) SetBeforeFunc(beforeFunc func(id any, method string, params any) error) {
	p.Server.Hooks.BeforeFunc = beforeFunc
}

func (p *Http) SetAfterFunc(afterFunc func(id any, method string, result any) error) {
	p.Server.Hooks.AfterFunc = afterFunc
}

func (p *Http) GetEvent() <-chan int {
	return p.Event
}

func (p *Http) handleFunc(w http.ResponseWriter, r *http.Request) {
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
	res := p.Server.Handler(data)
	_, err = w.Write(res)
	if err != nil {
		log.Panic(err.Error())
	}
}
