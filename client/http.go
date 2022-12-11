package client

import (
	"bytes"
	"fmt"
	"github.com/sunquakes/jsonrpc4go/common"
	"github.com/sunquakes/jsonrpc4go/discovery"
	"io/ioutil"
	"net/http"
	"strconv"
	"time"
)

type Http struct {
	Name      string
	Protocol  string
	Address   string
	Discovery discovery.Driver
}

type HttpClient struct {
	Name        string
	Protocol    string
	Address     string
	Discovery   discovery.Driver
	RequestList []*common.SingleRequest
}

func (p *Http) NewClient() Client {
	return NewHttpClient(p.Name, p.Protocol, p.Address, p.Discovery)
}

func NewHttpClient(name string, protocol string, address string, dc discovery.Driver) *HttpClient {
	return &HttpClient{
		name,
		protocol,
		address,
		dc,
		nil,
	}
}

func (c *HttpClient) SetOptions(httpOptions any) {
}

func (c *HttpClient) SetPoolOptions(httpOptions any) {
}

func (c *HttpClient) BatchAppend(method string, params any, result any, isNotify bool) *error {
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

func (c *HttpClient) BatchCall() error {
	var (
		err error
		br  []any
	)
	for _, v := range c.RequestList {
		var (
			req any
		)
		method := fmt.Sprintf("%s/%s", c.Name, v.Method)
		if v.IsNotify == true {
			req = common.Rs(nil, method, v.Params)
		} else {
			req = common.Rs(strconv.FormatInt(time.Now().Unix(), 10), method, v.Params)
		}
		br = append(br, req)
	}
	bReq := common.JsonBatchRs(br)
	err = c.handleFunc(bReq, c.RequestList)
	c.RequestList = make([]*common.SingleRequest, 0)
	return err
}

func (c *HttpClient) Call(method string, params any, result any, isNotify bool) error {
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
	err = c.handleFunc(req, result)
	return err
}

func (c *HttpClient) handleFunc(b []byte, result any) error {
	address := c.Address
	//address := GetAddress(c.AddressList)
	url := fmt.Sprintf("%s://%s", c.Protocol, address)
	resp, err := http.Post(url, "application/json", bytes.NewReader(b))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	err = common.GetResult(body, result)
	return err
}
