package client

import (
	"bytes"
	"fmt"
	"github.com/sunquakes/jsonrpc4go/common"
	"net"
	"strconv"
	"time"
)

type Tcp struct {
	Ip   string
	Port string
}

type TcpClient struct {
	Ip          string
	Port        string
	RequestList []*common.SingleRequest
	Options     TcpOptions
	Conn        net.Conn
}

type TcpOptions struct {
	PackageEof       string
	PackageMaxLength int64
}

func (c *Tcp) NewClient() (Client, error) {
	ip := c.Ip
	port := c.Port
	options := TcpOptions{
		"\r\n",
		1024 * 1024 * 2,
	}
	var addr = fmt.Sprintf("%s:%s", ip, port)
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return nil, err
	}
	return &TcpClient{
		ip,
		port,
		nil,
		options,
		conn,
	}, err
}

func NewTcpClient(ip string, port string) (*TcpClient, error) {
	options := TcpOptions{
		"\r\n",
		1024 * 1024 * 2,
	}
	var addr = fmt.Sprintf("%s:%s", ip, port)
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return nil, err
	}
	return &TcpClient{
		ip,
		port,
		nil,
		options,
		conn,
	}, err
}

func (p *TcpClient) BatchAppend(method string, params any, result any, isNotify bool) *error {
	singleRequest := &common.SingleRequest{
		method,
		params,
		result,
		new(error),
		isNotify,
	}
	p.RequestList = append(p.RequestList, singleRequest)
	return singleRequest.Error
}

func (p *TcpClient) BatchCall() error {
	var (
		err error
		br  []any
	)
	for _, v := range p.RequestList {
		var (
			req any
		)
		if v.IsNotify == true {
			req = common.Rs(nil, v.Method, v.Params)
		} else {
			req = common.Rs(strconv.FormatInt(time.Now().Unix(), 10), v.Method, v.Params)
		}
		br = append(br, req)
	}
	bReq := common.JsonBatchRs(br)
	bReq = append(bReq, []byte(p.Options.PackageEof)...)
	err = p.handleFunc(bReq, p.RequestList)
	p.RequestList = make([]*common.SingleRequest, 0)
	return err
}

func (p *TcpClient) SetOptions(tcpOptions any) {
	p.Options = tcpOptions.(TcpOptions)
}

func (p *TcpClient) Call(method string, params any, result any, isNotify bool) error {
	var (
		err error
		req []byte
	)
	if isNotify {
		req = common.JsonRs(nil, method, params)
	} else {
		req = common.JsonRs(strconv.FormatInt(time.Now().Unix(), 10), method, params)
	}
	req = append(req, []byte(p.Options.PackageEof)...)
	err = p.handleFunc(req, result)
	return err
}

func (p *TcpClient) handleFunc(b []byte, result any) error {
	var err error
	_, err = p.Conn.Write(b)
	if err != nil {
		return err
	}

	eofb := []byte(p.Options.PackageEof)
	eofl := len(eofb)
	var (
		data []byte
	)
	l := 0
	for {
		var buf = make([]byte, p.Options.PackageMaxLength)
		n, err := p.Conn.Read(buf)
		if err != nil {
			if n == 0 {
				return err
			}
			common.Debug(err.Error())
		}
		l += n
		data = append(data, buf[:n]...)
		if bytes.Equal(data[l-eofl:], eofb) {
			break
		}
	}
	err = common.GetResult(data[:l-eofl], result)
	return err
}
