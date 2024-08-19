package test

import (
	"fmt"
	"github.com/sunquakes/jsonrpc4go"
	"github.com/sunquakes/jsonrpc4go/common"
	"github.com/sunquakes/jsonrpc4go/discovery/consul"
	"github.com/sunquakes/jsonrpc4go/discovery/nacos"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

const EQUAL_MESSAGE_TEMPLETE = "%d + %d expected be %d, but %d got"
const ERROR_MESSAGE_TEMPLETE = "Error expected be %s, but %s got"

type IntRpc struct{}

type Params struct {
	A int `json:"a"`
	B int `json:"b"`
}

type Result = int

func (i *IntRpc) Add(params *Params, result *Result) error {
	a := params.A + params.B
	*result = any(a).(Result)
	return nil
}

func (i *IntRpc) Sub(params *Params, result *int) error {
	a := params.A - params.B
	*result = any(a).(int)
	return nil
}

func TestHttpCall(t *testing.T) {
	s, _ := jsonrpc4go.NewServer("http", 3201)
	s.Register(new(IntRpc))
	go func() {
		s.Start()
	}()
	<-s.GetEvent()
	time.Sleep(time.Duration(2) * time.Second)
	c, _ := jsonrpc4go.NewClient("IntRpc", "http", "127.0.0.1:3201")
	params := Params{1, 2}
	result := new(Result)
	_ = c.Call("Add", &params, result, false)
	if *result != 3 {
		t.Errorf(EQUAL_MESSAGE_TEMPLETE, params.A, params.B, 3, *result)
	}
}

func TestHttpCallMethod(t *testing.T) {
	s, _ := jsonrpc4go.NewServer("http", 3202)
	s.Register(new(IntRpc))
	go func() {
		s.Start()
	}()
	<-s.GetEvent()
	c, _ := jsonrpc4go.NewClient("IntRpc", "http", "127.0.0.1:3202")
	params := Params{1, 2}
	result := new(Result)
	_ = c.Call("Add", &params, result, false)
	if *result != 3 {
		t.Errorf(EQUAL_MESSAGE_TEMPLETE, params.A, params.B, 3, *result)
	}
}

func TestHttpNotifyCall(t *testing.T) {
	s, _ := jsonrpc4go.NewServer("http", 3203)
	s.Register(new(IntRpc))
	go func() {
		s.Start()
	}()
	<-s.GetEvent()
	c, _ := jsonrpc4go.NewClient("IntRpc", "http", "127.0.0.1:3203")
	params := Params{2, 3}
	result := new(Result)
	_ = c.Call("Add", &params, result, true)
	if *result != 5 {
		t.Errorf(EQUAL_MESSAGE_TEMPLETE, params.A, params.B, 5, *result)
	}
}

func TestHttpBatchCall(t *testing.T) {
	s, _ := jsonrpc4go.NewServer("http", 3204)
	s.Register(new(IntRpc))
	go func() {
		s.Start()
	}()
	<-s.GetEvent()
	c, _ := jsonrpc4go.NewClient("IntRpc", "http", "127.0.0.1:3204")

	result1 := new(Result)
	err1 := c.BatchAppend("Add1", Params{1, 6}, result1, false)
	result2 := new(Result)
	err2 := c.BatchAppend("Add", Params{2, 3}, result2, false)
	_ = c.BatchCall()
	if *err2 != nil || *result2 != 5 {
		t.Errorf(EQUAL_MESSAGE_TEMPLETE, 2, 3, 5, result2)
	}
	if (*err1).Error() != common.CodeMap[common.MethodNotFound] {
		t.Errorf(ERROR_MESSAGE_TEMPLETE, common.CodeMap[common.MethodNotFound], (*err1).Error())
	}
}

func TestHttpRateLimit(t *testing.T) {
	params := Params{1, 2}
	s, _ := jsonrpc4go.NewServer("http", 3205)
	s.Register(new(IntRpc))
	s.SetRateLimit(0.5, 1)
	go func() {
		s.Start()
	}()
	<-s.GetEvent()
	c, _ := jsonrpc4go.NewClient("IntRpc", "http", "127.0.0.1:3205")
	result := new(Result)
	err := c.Call("Add", &params, result, false)
	if err != nil {
		t.Errorf(ERROR_MESSAGE_TEMPLETE, "nil", err.Error())
	}
	err = c.Call("Add", &params, result, false)
	if err.Error() != "Too many requests" {
		t.Errorf(ERROR_MESSAGE_TEMPLETE, "Too many requests", err.Error())
	}
	time.Sleep(time.Duration(2) * time.Second)
	err = c.Call("Add", &params, result, false)
	if err != nil {
		t.Errorf(ERROR_MESSAGE_TEMPLETE, "nil", err.Error())
	}
}

func TestHttpConsul(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, `[{"AggregatedStatus":"passing","Service":{"ID":"IntRpc:3615","Service":"IntRpc","Tags":[],"Meta":{},"Port":3615,"Address":"127.0.0.1","TaggedAddresses":{"lan_ipv4":{"Address":"127.0.0.1","Port":3615},"wan_ipv4":{"Address":"127.0.0.1","Port":3615}},"Weights":{"Passing":1,"Warning":1},"EnableTagOverride":false,"Datacenter":"dc1"},"Checks":[{"Node":"1ae846e40d15","CheckID":"service:IntRpc:3615","Name":"Service 'IntRpc' check","Status":"passing","Notes":"","Output":"HTTP GET http://127.0.0.1:3615: 200 OK Output: ","ServiceID":"IntRpc:3615","ServiceName":"IntRpc","ServiceTags":null,"Type":"","ExposedPort":0,"Definition":{"Interval":"0s","Timeout":"0s","DeregisterCriticalServiceAfter":"0s","HTTP":"","Header":null,"Method":"","Body":"","TLSServerName":"","TLSSkipVerify":false,"TCP":"","UDP":"","GRPC":"","GRPCUseTLS":false},"CreateIndex":0,"ModifyIndex":0}]}]`)
	}))
	dc, err := consul.NewConsul(ts.URL)
	// dc, err := consul.NewConsul("http://localhost:8500?check=false&instanceId=1&interval=10s")
	if err != nil {
		t.Errorf(err.Error())
	}
	go func() {
		s, _ := jsonrpc4go.NewServer("http", 3615)
		s.SetDiscovery(dc, "")
		s.Register(new(IntRpc))
		s.Start()
	}()
	time.Sleep(time.Duration(2) * time.Second)

	c, _ := jsonrpc4go.NewClient("IntRpc", "http", dc)
	params := Params{10, 11}
	result := new(Result)
	c.Call("Add", &params, result, false)
	if *result != 21 {
		t.Errorf(EQUAL_MESSAGE_TEMPLETE, params.A, params.B, 21, *result)
	}
}

func TestHttpNacos(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, `{"name":"DEFAULT_GROUP@@java_tcp","groupName":"DEFAULT_GROUP","clusters":"","cacheMillis":10000,"hosts":[{"instanceId":"127.0.0.1#3617#DEFAULT#DEFAULT_GROUP@@java_tcp","ip":"127.0.0.1","port":3617,"weight":1.0,"healthy":true,"enabled":true,"ephemeral":true,"clusterName":"DEFAULT","serviceName":"DEFAULT_GROUP@@java_tcp","metadata":{},"instanceHeartBeatInterval":5000,"instanceHeartBeatTimeOut":15000,"ipDeleteTimeout":30000,"instanceIdGenerator":"simple"}],"lastRefTime":1673444367069,"checksum":"","allIPs":false,"reachProtectionThreshold":false,"valid":true}`)
	}))
	dc, err := nacos.NewNacos(ts.URL)
	// dc, err := nacos.NewNacos("http://localhost:8849")
	if err != nil {
		t.Errorf(err.Error())
	}
	go func() {
		s, _ := jsonrpc4go.NewServer("http", 3617)
		s.SetDiscovery(dc, "")
		s.Register(new(IntRpc))
		s.Start()
	}()
	time.Sleep(time.Duration(2) * time.Second)

	c, _ := jsonrpc4go.NewClient("IntRpc", "http", dc)
	params := Params{10, 11}
	result := new(Result)
	c.Call("Add", &params, result, false)
	if *result != 21 {
		t.Errorf(EQUAL_MESSAGE_TEMPLETE, params.A, params.B, 21, *result)
	}
}

func TestDataType(t *testing.T) {
	s, _ := jsonrpc4go.NewServer("http", 3618)
	s.Register(new(IntRpc))
	go func() {
		s.Start()
	}()
	<-s.GetEvent()
	c, _ := jsonrpc4go.NewClient("IntRpc", "http", "127.0.0.1:3618")
	params := Params{2, 1}
	result := new(int)
	_ = c.Call("Sub", &params, result, false)
	if *result != 1 {
		t.Errorf(EQUAL_MESSAGE_TEMPLETE, params.A, params.B, 1, *result)
	}
}
