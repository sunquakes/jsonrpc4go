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
	Ip     string
	Port   string
	Server common.Server
	Options HttpOptions
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
	}
}

func (p *Http) Start() {
	mux := http.NewServeMux()
	mux.HandleFunc("/", p.handleFunc)
	var url = fmt.Sprintf("%s:%s", p.Ip, p.Port)
	log.Printf("Listening http://%s:%s", p.Ip, p.Port)
	http.ListenAndServe(url, mux)
}

func (p *Http) Register(s interface{}) {
	p.Server.Register(s)
}

func (p *Http) SetOptions(httpOptions interface{}) {
	p.Options = httpOptions.(HttpOptions)
}

func (p *Http) SetRateLimit(r rate.Limit, b int) {
	p.Server.RateLimiter = rate.NewLimiter(r, b);
}

func (p *Http) SetBeforeFunc(beforeFunc func(id interface{}, method string, params interface{}) error) {
	p.Server.Hooks.BeforeFunc = beforeFunc
}

func (p *Http) SetAfterFunc(afterFunc func(id interface{}, method string, result interface{}) error) {
	p.Server.Hooks.AfterFunc = afterFunc
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
	w.Write(res)
}
