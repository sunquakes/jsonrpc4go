package client

import (
	"bytes"
	"errors"
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
	Pool        *Pool
}

type TcpOptions struct {
	PackageEof       string
	PackageMaxLength int64
}

func (p *Tcp) NewClient() Client {
	ip := p.Ip
	port := p.Port
	return NewTcpClient(ip, port)
}

func NewTcpClient(ip string, port string) *TcpClient {
	options := TcpOptions{
		"\r\n",
		1024 * 1024 * 2,
	}
	pool := NewPool(ip, port, PoolOptions{5, 5})
	return &TcpClient{
		ip,
		port,
		nil,
		options,
		pool,
	}
}

func (c *TcpClient) BatchAppend(method string, params any, result any, isNotify bool) *error {
	singleRequest := &common.SingleRequest{
		method,
		params,
		result,
		new(error),
		isNotify,
	}
	c.RequestList = append(c.RequestList, singleRequest)
	return singleRequest.Error
}

func (c *TcpClient) BatchCall() error {
	var (
		err error
		br  []any
	)
	for _, v := range c.RequestList {
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
	bReq = append(bReq, []byte(c.Options.PackageEof)...)
	err = c.handleFunc(bReq, c.RequestList)
	c.RequestList = make([]*common.SingleRequest, 0)
	return err
}

func (c *TcpClient) SetOptions(tcpOptions any) {
	c.Options = tcpOptions.(TcpOptions)
}

func (c *TcpClient) SetPoolOptions(poolOption any) {
	c.Pool.SetOptions(poolOption.(PoolOptions))
}

func (c *TcpClient) Call(method string, params any, result any, isNotify bool) error {
	var (
		err error
		req []byte
	)
	if isNotify {
		req = common.JsonRs(nil, method, params)
	} else {
		req = common.JsonRs(strconv.FormatInt(time.Now().Unix(), 10), method, params)
	}
	req = append(req, []byte(c.Options.PackageEof)...)
	err = c.handleFunc(req, result)
	return err
}

func (c *TcpClient) handleFunc(b []byte, result any) error {
	var (
		err  error
		conn net.Conn
	)

	conn = c.Pool.Borrow()
	_, err = conn.Write(b)
	if err != nil {
		conn = c.Pool.BorrowAfterRemove(conn)
		_, err = conn.Write(b)
		if conn == nil {
			return errors.New("Unable to connect to the server.")
		}
	}

	defer c.Pool.Release(conn)

	eofb := []byte(c.Options.PackageEof)
	eofl := len(eofb)
	var (
		data []byte
	)
	l := 0
	for {
		var buf = make([]byte, c.Options.PackageMaxLength)
		n, err := conn.Read(buf)
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
