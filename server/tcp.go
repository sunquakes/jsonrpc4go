package server

import (
	"context"
	"fmt"
	"github.com/sunquakes/jsonrpc4go/common"
	"golang.org/x/time/rate"
	"log"
	"net"
	"sync"
)

type Tcp struct {
	Ip      string
	Port    string
	Server  common.Server
	Options TcpOptions
}

type TcpOptions struct {
	PackageEof       string
	PackageMaxLength int32
}

func NewTcpServer(ip string, port string) *Tcp {
	options := TcpOptions{
		"\r\n",
		1024 * 1024 * 2,
	}
	return &Tcp{
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

func (p *Tcp) Start() {
	var addr = fmt.Sprintf("%s:%s", p.Ip, p.Port)
	tcpAddr, err := net.ResolveTCPAddr("tcp", addr) //解析tcp服务
	if err != nil {
		common.Debug(err.Error())
	}
	listener, _ := net.ListenTCP("tcp", tcpAddr)
	log.Printf("Listening tcp://%s:%s", p.Ip, p.Port)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	for {
		conn, err := listener.AcceptTCP()
		if err != nil {
			common.Debug(err.Error())
			continue
		}
		go p.handleFunc(ctx, conn)
	}
}

func (p *Tcp) Register(s interface{}) {
	p.Server.Register(s)
}

func (p *Tcp) SetOptions(tcpOptions interface{}) {
	p.Options = tcpOptions.(TcpOptions)
}

func (p *Tcp) SetRateLimit(r rate.Limit, b int) {
	p.Server.RateLimiter = rate.NewLimiter(r, b);
}

func (p *Tcp) SetBeforeFunc(beforeFunc func(id interface{}, method string, params interface{}) error) {
	p.Server.Hooks.BeforeFunc = beforeFunc
}

func (p *Tcp) SetAfterFunc(afterFunc func(id interface{}, method string, result interface{}) error) {
	p.Server.Hooks.AfterFunc = afterFunc
}

func (p *Tcp) handleFunc(ctx context.Context, conn net.Conn) {
	defer conn.Close()
	select {
	case <-ctx.Done():
		return
	default:
		//	do nothing
	}
	for {
		var buf = make([]byte, p.Options.PackageMaxLength)
		n, err := conn.Read(buf)
		if err != nil {
			if n == 0 {
				continue
			}
			common.Debug(err.Error())
			break
		}
		l := len([]byte(p.Options.PackageEof))
		res := p.Server.Handler(buf[:n-l])
		res = append(res, []byte(p.Options.PackageEof)...)
		conn.Write(res)
	}
}
