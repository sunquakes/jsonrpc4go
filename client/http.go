package client

import (
	"bytes"
	"fmt"
	"github.com/sunquakes/jsonrpc4go/common"
	"io/ioutil"
	"net/http"
	"strconv"
	"time"
)

type Http struct {
	Ip   string
	Port string
}

type HttpClient struct {
	Ip          string
	Port        string
	RequestList []*common.SingleRequest
}

func (c *Http) NewClient() (Client, error) {
	return &HttpClient{
		c.Ip,
		c.Port,
		nil,
	}, nil
}

func NewHttpClient(ip string, port string) *HttpClient {
	return &HttpClient{
		ip,
		port,
		nil,
	}
}

func (p *HttpClient) SetOptions(httpOptions any) {
}

func (p *HttpClient) BatchAppend(method string, params any, result any, isNotify bool) *error {
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

func (p *HttpClient) BatchCall() error {
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
	err = p.handleFunc(bReq, p.RequestList)
	p.RequestList = make([]*common.SingleRequest, 0)
	return err
}

func (p *HttpClient) Call(method string, params any, result any, isNotify bool) error {
	var (
		err error
		req []byte
	)
	if isNotify {
		req = common.JsonRs(nil, method, params)
	} else {
		req = common.JsonRs(strconv.FormatInt(time.Now().Unix(), 10), method, params)
	}
	err = p.handleFunc(req, result)
	return err
}

func (p *HttpClient) handleFunc(b []byte, result any) error {
	var url = fmt.Sprintf("http://%s:%s", p.Ip, p.Port)
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
