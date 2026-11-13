package test

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/sunquakes/jsonrpc4go"
	"github.com/sunquakes/jsonrpc4go/client"
	"github.com/sunquakes/jsonrpc4go/common"
	"github.com/sunquakes/jsonrpc4go/discovery/consul"
	"github.com/sunquakes/jsonrpc4go/discovery/nacos"
	"github.com/sunquakes/jsonrpc4go/server"
)

const CUSTOM_ERROR = "Custom Error"

func TestTcpCall(t *testing.T) {
	s, _ := jsonrpc4go.NewServer("tcp", 0)
	s.Register(new(IntRpc))
	go s.Start()
	<-s.GetEvent()
	c, _ := jsonrpc4go.NewClient("IntRpc", "tcp", serverAddr(s))
	params := Params{1, 2}
	result := new(int)
	MakeRpcCallWithErrorCheck(t, c, "Add", &params, result)
	if *result != 3 {
		t.Errorf(EQUAL_MESSAGE_TEMPLETE, params.A, params.B, 3, *result)
	}
}

func TestTcpCallMethod(t *testing.T) {
	s, _ := jsonrpc4go.NewServer("tcp", 0)
	s.Register(new(IntRpc))
	go s.Start()
	<-s.GetEvent()
	c, _ := jsonrpc4go.NewClient("IntRpc", "tcp", serverAddr(s))
	params := Params{1, 2}
	result := new(int)
	MakeRpcCallWithErrorCheck(t, c, "Add", &params, result)
	if *result != 3 {
		t.Errorf(EQUAL_MESSAGE_TEMPLETE, params.A, params.B, 3, *result)
	}
}

func TestTcpNotifyCall(t *testing.T) {
	s, _ := jsonrpc4go.NewServer("tcp", 0)
	s.Register(new(IntRpc))
	go s.Start()
	<-s.GetEvent()
	c, _ := jsonrpc4go.NewClient("IntRpc", "tcp", serverAddr(s))
	params := Params{2, 3}
	result := new(int)
	MakeRpcCallWithErrorCheck(t, c, "Add", &params, result)
	if *result != 5 {
		t.Errorf(EQUAL_MESSAGE_TEMPLETE, params.A, params.B, 5, *result)
	}
}

func TestTcpBatchCall(t *testing.T) {
	s, _ := jsonrpc4go.NewServer("tcp", 0)
	s.Register(new(IntRpc))
	go s.Start()
	<-s.GetEvent()
	c, _ := jsonrpc4go.NewClient("IntRpc", "tcp", serverAddr(s))

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

func TestSetOption(t *testing.T) {
	s, _ := jsonrpc4go.NewServer("tcp", 0)
	s.SetOptions(server.TcpOptions{PackageEof: "aaaaaa", PackageMaxLength: 2 * 1024 * 1024})
	s.Register(new(IntRpc))
	go s.Start()
	<-s.GetEvent()
	c, _ := jsonrpc4go.NewClient("IntRpc", "tcp", serverAddr(s))
	c.SetOptions(client.TcpOptions{PackageEof: "aaaaaa", PackageMaxLength: 2 * 1024 * 1024})
	params := Params{1, 2}
	result := new(int)
	MakeRpcCallWithErrorCheck(t, c, "Add", &params, result)
	if *result != 3 {
		t.Errorf(EQUAL_MESSAGE_TEMPLETE, params.A, params.B, 3, *result)
	}
}

func TestSetHooks(t *testing.T) {
	params := Params{1, 2}
	s, _ := jsonrpc4go.NewServer("tcp", 0)
	s.SetBeforeFunc(func(id any, method string, p any) error {
		if method != "Add" {
			t.Errorf("Method expected be %s, but %s got", "Add", method)
		}
		if p.(Params) != params {
			jsonParams, _ := json.Marshal(params)
			jsonP, _ := json.Marshal(p)
			t.Errorf("Params expected be %s, but %s got", jsonParams, jsonP)
		}
		return nil
	})
	s.SetAfterFunc(func(id any, method string, r any) error {
		if method != "Add" {
			t.Errorf("Method expected be %s, but %s got", "Add", method)
		}
		if r != 3 {
			t.Errorf("int expected be %d, but %d got", 3, r)
		}
		return nil
	})
	s.Register(new(IntRpc))
	go s.Start()
	<-s.GetEvent()
	c, _ := jsonrpc4go.NewClient("IntRpc", "tcp", serverAddr(s))
	result := new(int)
	MakeRpcCallWithErrorCheck(t, c, "Add", &params, result)
	if *result != 3 {
		t.Errorf(EQUAL_MESSAGE_TEMPLETE, params.A, params.B, 3, *result)
	}
}

func TestSetHooksCustomError(t *testing.T) {
	params := Params{1, 2}
	s, _ := jsonrpc4go.NewServer("tcp", 0)
	s.SetBeforeFunc(func(id any, method string, p any) error {
		return errors.New(CUSTOM_ERROR)
	})
	s.Register(new(IntRpc))
	go s.Start()
	<-s.GetEvent()
	c, _ := jsonrpc4go.NewClient("IntRpc", "tcp", serverAddr(s))
	result := new(int)
	err := c.Call("Add", &params, result, false)
	if err.Error() != CUSTOM_ERROR {
		t.Errorf(ERROR_MESSAGE_TEMPLETE, CUSTOM_ERROR, err.Error())
	}
}

func TestRateLimit(t *testing.T) {
	params := Params{1, 2}
	s, _ := jsonrpc4go.NewServer("tcp", 0)
	s.Register(new(IntRpc))
	s.SetRateLimit(0.2, 1)
	go s.Start()
	<-s.GetEvent()
	c, _ := jsonrpc4go.NewClient("IntRpc", "tcp", serverAddr(s))
	result := new(int)
	err := c.Call("Add", &params, result, false)
	if err != nil {
		t.Errorf(ERROR_MESSAGE_TEMPLETE, "nil", err.Error())
	}
	err = c.Call("Add", &params, result, false)
	if err.Error() != "Too many requests" {
		t.Errorf(ERROR_MESSAGE_TEMPLETE, "Too many requests", err.Error())
	}
	time.Sleep(5 * time.Second)
	err = c.Call("Add", &params, result, false)
	if err != nil {
		t.Errorf(ERROR_MESSAGE_TEMPLETE, "nil", err.Error())
	}
}

type LongRpc struct{}

type LongParams struct {
	A string `json:"a"`
	B string `json:"b"`
}

func (i *LongRpc) Add(params *LongParams, result *string) error {
	*result = params.A + params.B
	return nil
}

func executeLongStringTests(t *testing.T, c client.Client, iterations int) {
	params := LongParams{LongString1, LongString2}
	result := new(string)

	for j := 0; j < iterations; j++ {
		MakeRpcCallWithErrorCheck(t, c, "Add", &params, result)
		ls := LongString1 + LongString2
		if *result != ls {
			t.Errorf("%s + %s expected be %s, but %s got", params.A, params.B, ls, *result)
		}
	}
}

func TestLongPackageTcpCall(t *testing.T) {
	s, _ := jsonrpc4go.NewServer("tcp", 0)
	s.SetOptions(server.TcpOptions{PackageEof: "\r\n", PackageMaxLength: 2 * 1024 * 1024})
	s.Register(new(LongRpc))
	go s.Start()
	<-s.GetEvent()
	addr := serverAddr(s)
	var wg sync.WaitGroup
	for i := 0; i < 11; i++ {
		wg.Add(1)
		go func(group *sync.WaitGroup) {
			defer group.Done()
			c, _ := jsonrpc4go.NewClient("LongRpc", "tcp", addr)
			c.SetOptions(client.TcpOptions{PackageEof: "\r\n", PackageMaxLength: 2 * 1024 * 1024})

			executeLongStringTests(t, c, 100)
			executeLongStringTests(t, c, 100)
		}(&wg)
	}
	wg.Wait()
}

func TestCoTcpCall(t *testing.T) {
	s, _ := jsonrpc4go.NewServer("tcp", 0)
	s.Register(new(IntRpc))
	go s.Start()
	<-s.GetEvent()
	addr := serverAddr(s)

	var wg sync.WaitGroup
	for i := 0; i < 40; i++ {
		wg.Add(1)
		go func(group *sync.WaitGroup, index int) {
			defer group.Done()
			c, _ := jsonrpc4go.NewClient("IntRpc", "tcp", addr)
			for j := 0; j < 100; j++ {
				params := Params{index, j}
				result := new(int)
				MakeRpcCallWithErrorCheck(t, c, "Add", &params, result)
				if *result != (index + j) {
					t.Errorf(EQUAL_MESSAGE_TEMPLETE, params.A, params.B, (index + j), *result)
				}
			}
		}(&wg, i)
	}
	wg.Wait()
}

func TestFailConnect(t *testing.T) {
	params := Params{1, 2}
	// First point the client at a port nothing is listening on, to verify failure.
	c, _ := jsonrpc4go.NewClient("IntRpc", "tcp", "127.0.0.1:1")
	result := new(int)
	var err error
	for i := 0; i < 20; i++ {
		err = c.Call("Add", &params, result, false)
		if err == nil {
			t.Errorf("Error expected be not nil, but nil got")
		}
	}
	// Start the server on an OS-assigned port, then re-point the same client.
	s, _ := jsonrpc4go.NewServer("tcp", 0)
	s.Register(new(IntRpc))
	go s.Start()
	<-s.GetEvent()
	c2, _ := jsonrpc4go.NewClient("IntRpc", "tcp", serverAddr(s))
	for i := 0; i < 20; i++ {
		err = c2.Call("Add", &params, result, false)
		if err != nil {
			t.Errorf(EQUAL_MESSAGE_TEMPLETE, params.A, params.B, 3, *result)
		}
	}
}

func TestRibbonTcpCall(t *testing.T) {
	// Start the first server.
	s1, _ := jsonrpc4go.NewServer("tcp", 0)
	s1.Register(new(IntRpc))
	go s1.Start()
	<-s1.GetEvent()
	// Start the second server.
	s2, _ := jsonrpc4go.NewServer("tcp", 0)
	s2.Register(new(IntRpc))
	go s2.Start()
	<-s2.GetEvent()
	addrList := serverAddr(s1) + "," + serverAddr(s2)

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(group *sync.WaitGroup, index int) {
			defer group.Done()
			c, _ := jsonrpc4go.NewClient("IntRpc", "tcp", addrList)
			for j := 0; j < 20; j++ {
				params := Params{index, j}
				result := new(int)
				MakeRpcCallWithErrorCheck(t, c, "Add", &params, result)
				if *result != (index + j) {
					t.Errorf(EQUAL_MESSAGE_TEMPLETE, params.A, params.B, (index + j), *result)
				}
			}
		}(&wg, i)
	}
	wg.Wait()
}

func TestTcpConsul(t *testing.T) {
	srv, _ := jsonrpc4go.NewServer("tcp", 0)
	srv.Register(new(IntRpc))
	go srv.Start()
	<-srv.GetEvent()
	port := srv.(*server.TcpServer).Port
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w,
			`[{"AggregatedStatus":"passing","Service":{"ID":"IntRpc:%d","Service":"IntRpc","Tags":[],"Meta":{},"Port":%d,"Address":"127.0.0.1","TaggedAddresses":{"lan_ipv4":{"Address":"127.0.0.1","Port":%d},"wan_ipv4":{"Address":"127.0.0.1","Port":%d}},"Weights":{"Passing":1,"Warning":1},"EnableTagOverride":false,"Datacenter":"dc1"},"Checks":[]}]`,
			port, port, port, port)
	}))
	defer ts.Close()
	dc, err := consul.NewConsul(ts.URL)
	if err != nil {
		t.Error(err.Error())
	}
	c, _ := jsonrpc4go.NewClient("IntRpc", "tcp", dc)
	params := Params{10, 11}
	result := new(int)
	MakeRpcCallWithErrorCheck(t, c, "Add", &params, result)
	if *result != 21 {
		t.Errorf(EQUAL_MESSAGE_TEMPLETE, params.A, params.B, 21, *result)
	}
}

func TestTcpNacos(t *testing.T) {
	srv, _ := jsonrpc4go.NewServer("tcp", 0)
	srv.Register(new(IntRpc))
	go srv.Start()
	<-srv.GetEvent()
	port := srv.(*server.TcpServer).Port
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
	c, _ := jsonrpc4go.NewClient("IntRpc", "tcp", dc)
	params := Params{10, 11}
	result := new(int)
	MakeRpcCallWithErrorCheck(t, c, "Add", &params, result)
	if *result != 21 {
		t.Errorf(EQUAL_MESSAGE_TEMPLETE, params.A, params.B, 21, *result)
	}
}
