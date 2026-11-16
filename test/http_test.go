package test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/sunquakes/jsonrpc4go"
	"github.com/sunquakes/jsonrpc4go/client"
	"github.com/sunquakes/jsonrpc4go/common"
	"github.com/sunquakes/jsonrpc4go/discovery/consul"
	"github.com/sunquakes/jsonrpc4go/discovery/nacos"
	"github.com/sunquakes/jsonrpc4go/server"
)

const (
	EQUAL_MESSAGE_TEMPLETE     = "%d + %d expected be %d, but %d got"
	ERROR_MESSAGE_TEMPLETE     = "Error expected be %s, but %s got"
	ERROR_CALLING_ADD_TEMPLETE = "Error calling Add: %v"
)

type IntRpc struct{}

type Params struct {
	A int `json:"a"`
	B int `json:"b"`
}

func (i *IntRpc) Add(params *Params, result *int) error {
	*result = params.A + params.B
	return nil
}

func (i *IntRpc) Sub(params *Params, result *int) error {
	*result = params.A - params.B
	return nil
}

func MakeRpcCallWithErrorCheck(t *testing.T, c client.Client, method string, params, result any) {
	if err := c.Call(method, params, result, false); err != nil {
		t.Errorf(ERROR_CALLING_ADD_TEMPLETE, err)
	}
}

func TestHttpCall(t *testing.T) {
	s, _ := jsonrpc4go.NewServer("http", 0)
	s.Register(new(IntRpc))
	go s.Start()
	<-s.GetEvent()
	c, _ := jsonrpc4go.NewClient("IntRpc", "http", serverAddr(s))
	params := Params{1, 2}
	result := new(int)
	MakeRpcCallWithErrorCheck(t, c, "Add", &params, result)
	if *result != 3 {
		t.Errorf(EQUAL_MESSAGE_TEMPLETE, params.A, params.B, 3, *result)
	}
}

func TestHttpCallMethod(t *testing.T) {
	s, _ := jsonrpc4go.NewServer("http", 0)
	s.Register(new(IntRpc))
	go s.Start()
	<-s.GetEvent()
	c, _ := jsonrpc4go.NewClient("IntRpc", "http", serverAddr(s))
	params := Params{1, 2}
	result := new(int)
	MakeRpcCallWithErrorCheck(t, c, "Add", &params, result)
	if *result != 3 {
		t.Errorf(EQUAL_MESSAGE_TEMPLETE, params.A, params.B, 3, *result)
	}
}

func TestHttpNotifyCall(t *testing.T) {
	s, _ := jsonrpc4go.NewServer("http", 0)
	s.Register(new(IntRpc))
	go s.Start()
	<-s.GetEvent()
	c, _ := jsonrpc4go.NewClient("IntRpc", "http", serverAddr(s))
	params := Params{2, 3}
	result := new(int)
	MakeRpcCallWithErrorCheck(t, c, "Add", &params, result)
	if *result != 5 {
		t.Errorf(EQUAL_MESSAGE_TEMPLETE, params.A, params.B, 5, *result)
	}
}

func TestHttpBatchCall(t *testing.T) {
	s, _ := jsonrpc4go.NewServer("http", 0)
	s.Register(new(IntRpc))
	go s.Start()
	<-s.GetEvent()
	c, _ := jsonrpc4go.NewClient("IntRpc", "http", serverAddr(s))

	result1 := new(int)
	err1 := c.BatchAppend("Add1", Params{1, 6}, result1, false)
	result2 := new(int)
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
	s, _ := jsonrpc4go.NewServer("http", 0)
	s.Register(new(IntRpc))
	s.SetRateLimit(0.5, 1)
	go s.Start()
	<-s.GetEvent()
	c, _ := jsonrpc4go.NewClient("IntRpc", "http", serverAddr(s))
	result := new(int)
	MakeRpcCallWithErrorCheck(t, c, "Add", &params, result)
	err := c.Call("Add", &params, result, false)
	if err.Error() != "Too many requests" {
		t.Errorf(ERROR_MESSAGE_TEMPLETE, "Too many requests", err.Error())
	}
	time.Sleep(time.Duration(2) * time.Second)
	MakeRpcCallWithErrorCheck(t, c, "Add", &params, result)
}

func TestHttpConsul(t *testing.T) {
	srv, _ := jsonrpc4go.NewServer("http", 0)
	srv.Register(new(IntRpc))
	go srv.Start()
	<-srv.GetEvent()
	port := srv.(*server.HttpServer).Port
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w,
			`[{"AggregatedStatus":"passing","Service":{"ID":"IntRpc:%d","Service":"IntRpc","Tags":[],"Meta":{},"Port":%d,"Address":"127.0.0.1","TaggedAddresses":{"lan_ipv4":{"Address":"127.0.0.1","Port":%d},"wan_ipv4":{"Address":"127.0.0.1","Port":%d}},"Weights":{"Passing":1,"Warning":1},"EnableTagOverride":false,"Datacenter":"dc1"},"Checks":[]}]`,
			port, port, port, port)
	}))
	defer ts.Close()
	dc, err := consul.NewConsul(ts.URL)
	if err != nil {
		t.Error(err)
	}
	c, _ := jsonrpc4go.NewClient("IntRpc", "http", dc)
	params := Params{10, 11}
	result := new(int)
	MakeRpcCallWithErrorCheck(t, c, "Add", &params, result)
	if *result != 21 {
		t.Errorf(EQUAL_MESSAGE_TEMPLETE, params.A, params.B, 21, *result)
	}
}

func TestHttpNacos(t *testing.T) {
	srv, _ := jsonrpc4go.NewServer("http", 0)
	srv.Register(new(IntRpc))
	go srv.Start()
	<-srv.GetEvent()
	port := srv.(*server.HttpServer).Port
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w,
			`{"name":"DEFAULT_GROUP@@java_tcp","groupName":"DEFAULT_GROUP","clusters":"","cacheMillis":10000,"hosts":[{"instanceId":"127.0.0.1#%d#DEFAULT#DEFAULT_GROUP@@java_tcp","ip":"127.0.0.1","port":%d,"weight":1.0,"healthy":true,"enabled":true,"ephemeral":true,"clusterName":"DEFAULT","serviceName":"DEFAULT_GROUP@@java_tcp","metadata":{},"instanceHeartBeatInterval":5000,"instanceHeartBeatTimeOut":15000,"ipDeleteTimeout":30000,"instanceIdGenerator":"simple"}],"lastRefTime":1673444367069,"checksum":"","allIPs":false,"reachProtectionThreshold":false,"valid":true}`,
			port, port)
	}))
	defer ts.Close()
	dc, err := nacos.NewNacos(ts.URL)
	if err != nil {
		t.Error(err.Error())
	}
	c, _ := jsonrpc4go.NewClient("IntRpc", "http", dc)
	params := Params{10, 11}
	result := new(int)
	MakeRpcCallWithErrorCheck(t, c, "Add", &params, result)
	if *result != 21 {
		t.Errorf(EQUAL_MESSAGE_TEMPLETE, params.A, params.B, 21, *result)
	}
}

func TestDataType(t *testing.T) {
	s, _ := jsonrpc4go.NewServer("http", 0)
	s.Register(new(IntRpc))
	go s.Start()
	<-s.GetEvent()
	c, _ := jsonrpc4go.NewClient("IntRpc", "http", serverAddr(s))
	params := Params{2, 1}
	result := new(int)
	MakeRpcCallWithErrorCheck(t, c, "Sub", &params, result)
	if *result != 1 {
		t.Errorf(EQUAL_MESSAGE_TEMPLETE, params.A, params.B, 1, *result)
	}
}
