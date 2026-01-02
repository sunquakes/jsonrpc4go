package client

import (
	"bytes"
	"fmt"
	"net"
	"strconv"
	"time"

	"github.com/sunquakes/jsonrpc4go/common"
	"github.com/sunquakes/jsonrpc4go/discovery"
)

/**
 * @Description: Basic configuration structure for TCP client
 * @Field Name: Service name
 * @Field Protocol: Protocol type
 * @Field Address: Service address
 * @Field Discovery: Service discovery driver
 */
type Tcp struct {
	Name      string
	Protocol  string
	Address   string
	Discovery discovery.Driver
}

/**
 * @Description: Main structure for TCP client
 * @Field Name: Service name
 * @Field Protocol: Protocol type
 * @Field Address: Service address
 * @Field Discovery: Service discovery driver
 * @Field RequestList: Request list for batch calls
 * @Field Options: TCP client options
 * @Field Pool: Connection pool
 */
type TcpClient struct {
	Name        string
	Protocol    string
	Address     string
	Discovery   discovery.Driver
	RequestList []*common.SingleRequest
	Options     TcpOptions
	Pool        *Pool
}

/**
 * @Description: Options structure for TCP client
 * @Field PackageEof: Packet end delimiter
 * @Field PackageMaxLength: Maximum packet length
 */
type TcpOptions struct {
	PackageEof       string
	PackageMaxLength int64
}

/**
 * @Description: Create a new TCP client
 * @Receiver p: Tcp structure pointer
 * @Return Client: Client interface
 */
func (p *Tcp) NewClient() Client {
	return NewTcpClient(p.Name, p.Protocol, p.Address, p.Discovery)
}

/**
 * @Description: Create a new TcpClient instance
 * @Param name: Service name
 * @Param protocol: Protocol type
 * @Param address: Service address
 * @Param dc: Service discovery driver
 * @Return *TcpClient: TcpClient instance pointer
 */
func NewTcpClient(name string, protocol string, address string, dc discovery.Driver) *TcpClient {
	options := &TcpOptions{
		"\r\n",
		1024 * 1024 * 2,
	}
	pool := NewPool(name, address, dc, PoolOptions{5, 5})
	return &TcpClient{
		Name:        name,
		Protocol:    protocol,
		Address:     address,
		Discovery:   dc,
		RequestList: nil,
		Options:     *options,
		Pool:        pool,
	}
}

/**
 * @Description: Batch add requests
 * @Receiver c: TcpClient structure pointer
 * @Param method: Method name
 * @Param params: Parameters
 * @Param result: Result
 * @Param isNotify: Whether it's a notification
 * @Return *error: Error pointer
 */
func (c *TcpClient) BatchAppend(method string, params any, result any, isNotify bool) *error {
	singleRequest := &common.SingleRequest{
		Method:   method,
		Params:   params,
		Result:   result,
		Error:    new(error),
		IsNotify: isNotify,
	}
	c.RequestList = append(c.RequestList, singleRequest)
	return singleRequest.Error
}

/**
 * @Description: Execute batch requests
 * @Receiver c: TcpClient structure pointer
 * @Return error: Error message
 */
func (c *TcpClient) BatchCall() error {
	var (
		err error
		br  []any
	)
	for _, v := range c.RequestList {
		var (
			req any
		)
		method := fmt.Sprintf("%s/%s", c.Name, v.Method)
		if v.IsNotify {
			req = common.Rs(nil, method, v.Params)
		} else {
			req = common.Rs(strconv.FormatInt(time.Now().Unix(), 10), method, v.Params)
		}
		br = append(br, req)
	}
	bReq := common.JsonBatchRs(br)
	bReq = append(bReq, []byte(c.Options.PackageEof)...)
	err = c.handleFunc(bReq, c.RequestList)
	c.RequestList = make([]*common.SingleRequest, 0)
	return err
}

/**
 * @Description: Set TCP options
 * @Receiver c: TcpClient structure pointer
 * @Param tcpOptions: TCP options
 */
func (c *TcpClient) SetOptions(tcpOptions any) {
	c.Options = tcpOptions.(TcpOptions)
}

/**
 * @Description: Set connection pool options
 * @Receiver c: TcpClient structure pointer
 * @Param poolOption: Connection pool options
 */
func (c *TcpClient) SetPoolOptions(poolOption any) {
	c.Pool.SetOptions(poolOption.(PoolOptions))
}

/**
 * @Description: Execute a single request
 * @Receiver c: TcpClient structure pointer
 * @Param method: Method name
 * @Param params: Parameters
 * @Param result: Result
 * @Param isNotify: Whether it's a notification
 * @Return error: Error message
 */
func (c *TcpClient) Call(method string, params any, result any, isNotify bool) error {
	var (
		err error
		req []byte
	)
	method = fmt.Sprintf("%s/%s", c.Name, method)
	if isNotify {
		req = common.JsonRs(nil, method, params)
	} else {
		req = common.JsonRs(strconv.FormatInt(time.Now().Unix(), 10), method, params)
	}
	req = append(req, []byte(c.Options.PackageEof)...)
	err = c.handleFunc(req, result)
	return err
}

/**
 * @Description: Handle request and response
 * @Receiver c: TcpClient structure pointer
 * @Param b: Request data
 * @Param result: Result
 * @Return error: Error message
 */
func (c *TcpClient) handleFunc(b []byte, result any) error {
	var (
		err  error
		conn net.Conn
	)

	conn, err = c.Pool.Borrow()
	if err == nil {
		_, err = conn.Write(b)
	}
	if err != nil {
		conn, err = c.Pool.BorrowAfterRemove(conn)
		if err != nil {
			c.Pool.Remove(conn)
			return err
		}
		_, err = conn.Write(b)
		if err != nil {
			c.Pool.Remove(conn)
			return err
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
		n, readErr := conn.Read(buf)
		if readErr != nil {
			if n == 0 {
				return readErr
			}
			common.Debug(readErr.Error())
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
