package client

import (
	"bytes"
	"errors"
	"fmt"
	"net"

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
 * @Param args: Positional method arguments
 */
func (c *TcpClient) BatchAppend(method string, args ...any) {
	singleRequest := &common.SingleRequest{
		Method: method,
		Args:   args,
	}
	c.RequestList = append(c.RequestList, singleRequest)
}

/**
 * @Description: Execute batch requests
 * @Receiver c: TcpClient structure pointer
 * @Return []BatchResult: Per-request results
 * @Return error: Error message
 */
func (c *TcpClient) BatchCall() ([]BatchResult, error) {
	var (
		err error
		br  []any
	)
	if len(c.RequestList) == 0 {
		return nil, nil
	}
	for _, v := range c.RequestList {
		method := fmt.Sprintf("%s/%s", c.Name, v.Method)
		br = append(br, common.Rs(nextRequestID(), method, normalizeArgs(v.Args)))
	}
	bReq := common.JsonBatchRs(br)
	bReq = append(bReq, []byte(c.Options.PackageEof)...)
	data, err := c.handleFunc(bReq)
	c.RequestList = make([]*common.SingleRequest, 0)
	if err != nil {
		return nil, err
	}
	items, err := common.GetBatchResult(data)
	if err != nil {
		return nil, err
	}
	results := make([]BatchResult, len(items))
	for k, v := range items {
		results[k] = BatchResult{Result: v.Result, Err: v.Err}
	}
	return results, nil
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
 * @Param args: Positional method arguments
 * @Return any: Result value
 * @Return error: Error message
 */
func (c *TcpClient) Call(method string, args ...any) (any, error) {
	fullMethod := fmt.Sprintf("%s/%s", c.Name, method)
	req := common.JsonRs(nextRequestID(), fullMethod, normalizeArgs(args))
	req = append(req, []byte(c.Options.PackageEof)...)
	data, err := c.handleFunc(req)
	if err != nil {
		return nil, err
	}
	return common.GetResult(data)
}

/**
 * @Description: Execute a notification request
 * @Receiver c: TcpClient structure pointer
 * @Param method: Method name
 * @Param args: Positional method arguments
 * @Return error: Error message
 */
func (c *TcpClient) Notify(method string, args ...any) error {
	fullMethod := fmt.Sprintf("%s/%s", c.Name, method)
	req := common.JsonRs(nil, fullMethod, normalizeArgs(args))
	req = append(req, []byte(c.Options.PackageEof)...)
	data, err := c.handleFunc(req)
	if err != nil {
		return err
	}
	// Drain the response so the pooled connection stays in sync.
	_, err = common.GetResult(data)
	return err
}

/**
 * @Description: Handle request and response
 * @Receiver c: TcpClient structure pointer
 * @Param b: Request data
 * @Return []byte: Response data
 * @Return error: Error message
 */
func (c *TcpClient) handleFunc(b []byte) (data []byte, err error) {
	conn, err := c.Pool.Borrow()
	if err != nil {
		return nil, err
	}
	// Return broken connections to the pool by removing them instead of reusing.
	defer func() {
		if err != nil {
			c.Pool.Remove(conn)
			return
		}
		c.Pool.Release(conn)
	}()

	if _, err = conn.Write(b); err != nil {
		// The connection may be stale; replace it and retry once.
		conn, err = c.Pool.BorrowAfterRemove(conn)
		if err != nil {
			return nil, err
		}
		if _, err = conn.Write(b); err != nil {
			return nil, err
		}
	}
	return readPackage(conn, c.Options)
}

/*
 * readPackage reads a single delimited response packet from conn.
 * @param conn - The TCP connection
 * @param options - The TCP client options
 * @return []byte - The response packet without the delimiter
 * @return error - An error if the read failed
 */
func readPackage(conn net.Conn, options TcpOptions) ([]byte, error) {
	eofb := []byte(options.PackageEof)
	if len(eofb) == 0 {
		return nil, errors.New("package eof is empty")
	}
	maxLength := options.PackageMaxLength
	if maxLength <= 0 {
		maxLength = 1024 * 1024 * 2
	}
	// Reuse a single read buffer instead of allocating the max package size per read.
	buf := make([]byte, 32*1024)
	var data []byte
	for {
		n, readErr := conn.Read(buf)
		if n > 0 {
			data = append(data, buf[:n]...)
			if len(data) >= len(eofb) && bytes.HasSuffix(data, eofb) {
				return data[:len(data)-len(eofb)], nil
			}
			if int64(len(data)) > maxLength {
				return nil, errors.New("package exceeds max length")
			}
		}
		if readErr != nil {
			return nil, readErr
		}
	}
}
