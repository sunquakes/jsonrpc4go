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
	EQUAL_MESSAGE_TEMPLETE     = "%d + %d expected be %d, but %v got"
	ERROR_MESSAGE_TEMPLETE     = "Error expected be %s, but %s got"
	ERROR_CALLING_ADD_TEMPLETE = "Error calling Add: %v"
)

type IntRpc struct{}

func (i *IntRpc) Add(a, b int) int {
	return a + b
}

func (i *IntRpc) Sub(a, b int) int {
	return a - b
}

func MakeRpcCallWithErrorCheck[T any](t *testing.T, c client.Client, method string, args ...any) T {
	result, err := jsonrpc4go.Call[T](c, method, args...)
	if err != nil {
		t.Errorf(ERROR_CALLING_ADD_TEMPLETE, err)
	}
	return result
}

func TestHttpCall(t *testing.T) {
	s, _ := jsonrpc4go.NewServer("http", 0)
	s.Register(new(IntRpc))
	go s.Start()
	<-s.GetEvent()
	c, _ := jsonrpc4go.NewClient("IntRpc", "http", serverAddr(s))
	result := MakeRpcCallWithErrorCheck[int](t, c, "Add", 1, 2)
	if result != 3 {
		t.Errorf(EQUAL_MESSAGE_TEMPLETE, 1, 2, 3, result)
	}
}

func TestHttpCallMethod(t *testing.T) {
	s, _ := jsonrpc4go.NewServer("http", 0)
	s.Register(new(IntRpc))
	go s.Start()
	<-s.GetEvent()
	c, _ := jsonrpc4go.NewClient("IntRpc", "http", serverAddr(s))
	result := MakeRpcCallWithErrorCheck[int](t, c, "Add", 1, 2)
	if result != 3 {
		t.Errorf(EQUAL_MESSAGE_TEMPLETE, 1, 2, 3, result)
	}
}

func TestHttpNotifyCall(t *testing.T) {
	s, _ := jsonrpc4go.NewServer("http", 0)
	s.Register(new(IntRpc))
	go s.Start()
	<-s.GetEvent()
	c, _ := jsonrpc4go.NewClient("IntRpc", "http", serverAddr(s))
	if err := c.Notify("Add", 2, 3); err != nil {
		t.Errorf(ERROR_CALLING_ADD_TEMPLETE, err)
	}
}

func TestHttpBatchCall(t *testing.T) {
	s, _ := jsonrpc4go.NewServer("http", 0)
	s.Register(new(IntRpc))
	go s.Start()
	<-s.GetEvent()
	c, _ := jsonrpc4go.NewClient("IntRpc", "http", serverAddr(s))

	c.BatchAppend("Add1", 1, 6)
	c.BatchAppend("Add", 2, 3)
	results, err := c.BatchCall()
	if err != nil {
		t.Errorf("Error calling BatchCall: %v", err)
	}
	if results[1].Err != nil {
		t.Errorf("Error expected be nil, but %v got", results[1].Err)
	}
	if value, ok := results[1].Result.(float64); !ok || value != 5 {
		t.Errorf(EQUAL_MESSAGE_TEMPLETE, 2, 3, 5, results[1].Result)
	}
	if results[0].Err == nil || results[0].Err.Error() != common.CodeMap[common.MethodNotFound] {
		t.Errorf(ERROR_MESSAGE_TEMPLETE, common.CodeMap[common.MethodNotFound], results[0].Err)
	}
}

func TestHttpRateLimit(t *testing.T) {
	s, _ := jsonrpc4go.NewServer("http", 0)
	s.Register(new(IntRpc))
	s.SetRateLimit(0.5, 1)
	go s.Start()
	<-s.GetEvent()
	c, _ := jsonrpc4go.NewClient("IntRpc", "http", serverAddr(s))
	MakeRpcCallWithErrorCheck[int](t, c, "Add", 1, 2)
	_, err := c.Call("Add", 1, 2)
	if err.Error() != "Too many requests" {
		t.Errorf(ERROR_MESSAGE_TEMPLETE, "Too many requests", err.Error())
	}
	time.Sleep(time.Duration(2) * time.Second)
	MakeRpcCallWithErrorCheck[int](t, c, "Add", 1, 2)
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
	result := MakeRpcCallWithErrorCheck[int](t, c, "Add", 10, 11)
	if result != 21 {
		t.Errorf(EQUAL_MESSAGE_TEMPLETE, 10, 11, 21, result)
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
	result := MakeRpcCallWithErrorCheck[int](t, c, "Add", 10, 11)
	if result != 21 {
		t.Errorf(EQUAL_MESSAGE_TEMPLETE, 10, 11, 21, result)
	}
}

func TestDataType(t *testing.T) {
	s, _ := jsonrpc4go.NewServer("http", 0)
	s.Register(new(IntRpc))
	go s.Start()
	<-s.GetEvent()
	c, _ := jsonrpc4go.NewClient("IntRpc", "http", serverAddr(s))
	result := MakeRpcCallWithErrorCheck[int](t, c, "Sub", 2, 1)
	if result != 1 {
		t.Errorf(EQUAL_MESSAGE_TEMPLETE, 2, 1, 1, result)
	}
}
