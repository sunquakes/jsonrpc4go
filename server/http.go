package server

import (
	"fmt"
	"github.com/iloveswift/go-jsonrpc/common"
	"io/ioutil"
	"log"
	"net/http"
)

type Http struct {
	Ip         string
	Port       string
	Server     common.Server
	BufferSize int
}

func NewHttpServer(ip string, port string) *Http {
	return &Http{
		ip,
		port,
		common.Server{},
		BufferSize,
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

func (p *Http) SetBuffer(bs int) {
	p.BufferSize = bs
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
