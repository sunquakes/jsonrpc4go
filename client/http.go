package client

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/sunquakes/jsonrpc4go/common"
	"github.com/sunquakes/jsonrpc4go/discovery"
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
	AddressList []*AddressInfo
	RequestList []*common.SingleRequest
	Options     *HttpOptions
}

type AddressInfo struct {
	Address string
	Load    int
}

type HttpOptions struct {
	CaPath string
}

func (p *Http) NewClient() Client {
	return NewHttpClient(p.Name, p.Protocol, p.Address, p.Discovery)
}

func NewHttpClient(name string, protocol string, address string, dc discovery.Driver) *HttpClient {
	c := &HttpClient{
		name,
		protocol,
		address,
		dc,
		nil,
		nil,
		nil,
	}
	c.SetAddressList()
	return c
}

func (c *HttpClient) SetOptions(httpOptions any) {
	// Set http request options.
	c.Options = httpOptions.(*HttpOptions)
}

func (c *HttpClient) SetPoolOptions(httpOptions any) {
	// Set http pool options.
}

func (c *HttpClient) BatchAppend(method string, params any, result any, isNotify bool) *error {
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
		if v.IsNotify {
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
	address, err := c.GetAddress()
	if err != nil {
		return err
	}
	url := fmt.Sprintf("%s://%s", c.Protocol, address)
	transport := &http.Transport{}

	client := &http.Client{Transport: transport}

	resp, err := client.Post(url, "application/json", bytes.NewReader(b))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	err = common.GetResult(body, result)
	return err
}

func (c *HttpClient) SetAddressList() {
	var (
		err error
	)
	address := c.Address
	if c.Discovery != nil {
		address, err = c.Discovery.Get(c.Name)
		if err != nil {
			common.Debug(err.Error())
		}
	}
	addresses := strings.Split(address, ",")
	addressList := make([]*AddressInfo, 0)
	for _, v := range addresses {
		addressList = append(addressList, &AddressInfo{
			Address: v,
			Load:    0,
		})
	}
	c.AddressList = addressList
}

func (c *HttpClient) GetAddress() (string, error) {
	size := len(c.AddressList)
	if size == 0 {
		c.SetAddressList()
	}
	size = len(c.AddressList)
	if size == 0 {
		return "", errors.New("fail to get service url")
	}
	if size == 1 {
		return c.AddressList[0].Address, nil
	}
	// Randomly select two nodes
	// 初始化随机数生成器，确保每次运行程序时结果不同
	randSource := rand.New(rand.NewSource(time.Now().UnixNano()))
	index1 := randSource.Intn(size)
	index2 := randSource.Intn(size)
	// Make sure the two nodes are different
	for index1 == index2 {
		index2 = rand.Intn(size)
	}
	if c.AddressList[index1].Load < c.AddressList[index2].Load {
		c.AddressList[index1].Load++
		return c.AddressList[index1].Address, nil
	}
	c.AddressList[index2].Load++
	return c.AddressList[index2].Address, nil
}
