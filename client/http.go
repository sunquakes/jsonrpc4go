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
	Ip          string
	Port        string
	RequestList []*common.SingleRequest
}

func NewHttpClient(ip string, port string) *Http {
	return &Http{
		ip,
		port,
		nil,
	}
}

func (p *Http) SetOptions(httpOptions interface{}) {
}

func (p *Http) BatchAppend(method string, params interface{}, result interface{}, isNotify bool) *error {
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

func (p *Http) BatchCall() error {
	var (
		err error
		br  []interface{}
	)
	for _, v := range p.RequestList {
		var (
			req interface{}
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

func (p *Http) Call(method string, params interface{}, result interface{}, isNotify bool) error {
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

func (p *Http) handleFunc(b []byte, result interface{}) error {
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
