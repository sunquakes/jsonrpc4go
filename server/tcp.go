package server

import (
	"fmt"
	"github.com/iloveswift/go-jsonrpc/common"
	"log"
	"net"
)

const (
	BufferSize = 1024
)

type Tcp struct {
	Ip         string
	Port       string
	Server     common.Server
	BufferSize int
}

func NewTcpServer(ip string, port string) *Tcp {
	return &Tcp{
		ip,
		port,
		common.Server{},
		BufferSize,
	}
}

func (p *Tcp) Start() {
	var addr = fmt.Sprintf("%s:%s", p.Ip, p.Port)
	listener, _ := net.Listen("tcp", addr)
	log.Printf("Listening tcp://%s:%s", p.Ip, p.Port)
	for {
		conn, err := listener.Accept()
		if err != nil {
			common.Debug(err.Error())
			continue
		}
		go p.handleFunc(conn)
	}
}

func (p *Tcp) Register(s interface{}) {
	p.Server.Register(s)
}

func (p *Tcp) SetBuffer(bs int) {
	p.BufferSize = bs
}

func (p *Tcp) handleFunc(conn net.Conn) {
	var (
		err error
	)

	var buf = make([]byte, p.BufferSize)
	n, err := conn.Read(buf)
	if err != nil {
		common.Debug(err.Error())
	}
	res := p.Server.Handler(buf[:n])
	conn.Write(res)
}
