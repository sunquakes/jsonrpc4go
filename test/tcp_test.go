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
	result := MakeRpcCallWithErrorCheck[int](t, c, "Add", 1, 2)
	if result != 3 {
		t.Errorf(EQUAL_MESSAGE_TEMPLETE, 1, 2, 3, result)
	}
}

func TestTcpCallMethod(t *testing.T) {
	s, _ := jsonrpc4go.NewServer("tcp", 0)
	s.Register(new(IntRpc))
	go s.Start()
	<-s.GetEvent()
	c, _ := jsonrpc4go.NewClient("IntRpc", "tcp", serverAddr(s))
	result := MakeRpcCallWithErrorCheck[int](t, c, "Add", 1, 2)
	if result != 3 {
		t.Errorf(EQUAL_MESSAGE_TEMPLETE, 1, 2, 3, result)
	}
}

func TestTcpNotifyCall(t *testing.T) {
	s, _ := jsonrpc4go.NewServer("tcp", 0)
	s.Register(new(IntRpc))
	go s.Start()
	<-s.GetEvent()
	c, _ := jsonrpc4go.NewClient("IntRpc", "tcp", serverAddr(s))
	if err := c.Notify("Add", 2, 3); err != nil {
		t.Errorf(ERROR_CALLING_ADD_TEMPLETE, err)
	}
}

func TestTcpBatchCall(t *testing.T) {
	s, _ := jsonrpc4go.NewServer("tcp", 0)
	s.Register(new(IntRpc))
	go s.Start()
	<-s.GetEvent()
	c, _ := jsonrpc4go.NewClient("IntRpc", "tcp", serverAddr(s))

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

func TestSetOption(t *testing.T) {
	s, _ := jsonrpc4go.NewServer("tcp", 0)
	s.SetOptions(server.TcpOptions{PackageEof: "aaaaaa", PackageMaxLength: 2 * 1024 * 1024})
	s.Register(new(IntRpc))
	go s.Start()
	<-s.GetEvent()
	c, _ := jsonrpc4go.NewClient("IntRpc", "tcp", serverAddr(s))
	c.SetOptions(client.TcpOptions{PackageEof: "aaaaaa", PackageMaxLength: 2 * 1024 * 1024})
	result := MakeRpcCallWithErrorCheck[int](t, c, "Add", 1, 2)
	if result != 3 {
		t.Errorf(EQUAL_MESSAGE_TEMPLETE, 1, 2, 3, result)
	}
}

func TestSetHooks(t *testing.T) {
	s, _ := jsonrpc4go.NewServer("tcp", 0)
	s.SetBeforeFunc(func(id any, method string, p any) error {
		if method != "Add" {
			t.Errorf("Method expected be %s, but %s got", "Add", method)
		}
		args, ok := p.([]any)
		if !ok || len(args) != 2 || args[0] != 1 || args[1] != 2 {
			jsonP, _ := json.Marshal(p)
			t.Errorf("Params expected be %s, but %s got", `[1,2]`, jsonP)
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
	result := MakeRpcCallWithErrorCheck[int](t, c, "Add", 1, 2)
	if result != 3 {
		t.Errorf(EQUAL_MESSAGE_TEMPLETE, 1, 2, 3, result)
	}
}

func TestSetHooksCustomError(t *testing.T) {
	s, _ := jsonrpc4go.NewServer("tcp", 0)
	s.SetBeforeFunc(func(id any, method string, p any) error {
		return errors.New(CUSTOM_ERROR)
	})
	s.Register(new(IntRpc))
	go s.Start()
	<-s.GetEvent()
	c, _ := jsonrpc4go.NewClient("IntRpc", "tcp", serverAddr(s))
	_, err := jsonrpc4go.Call[int](c, "Add", 1, 2)
	if err.Error() != CUSTOM_ERROR {
		t.Errorf(ERROR_MESSAGE_TEMPLETE, CUSTOM_ERROR, err.Error())
	}
}

func TestRateLimit(t *testing.T) {
	s, _ := jsonrpc4go.NewServer("tcp", 0)
	s.Register(new(IntRpc))
	s.SetRateLimit(0.2, 1)
	go s.Start()
	<-s.GetEvent()
	c, _ := jsonrpc4go.NewClient("IntRpc", "tcp", serverAddr(s))
	_, err := jsonrpc4go.Call[int](c, "Add", 1, 2)
	if err != nil {
		t.Errorf(ERROR_MESSAGE_TEMPLETE, "nil", err.Error())
	}
	_, err = jsonrpc4go.Call[int](c, "Add", 1, 2)
	if err.Error() != "Too many requests" {
		t.Errorf(ERROR_MESSAGE_TEMPLETE, "Too many requests", err.Error())
	}
	time.Sleep(5 * time.Second)
	_, err = jsonrpc4go.Call[int](c, "Add", 1, 2)
	if err != nil {
		t.Errorf(ERROR_MESSAGE_TEMPLETE, "nil", err.Error())
	}
}

type LongRpc struct{}

func (i *LongRpc) Add(a, b string) string {
	return a + b
}

func executeLongStringTests(t *testing.T, c client.Client, iterations int) {
	for range iterations {
		result := MakeRpcCallWithErrorCheck[string](t, c, "Add", LongString1, LongString2)
		ls := LongString1 + LongString2
		if result != ls {
			t.Errorf("The long string result does not match")
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
	for range 11 {
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
	for i := range 40 {
		wg.Add(1)
		go func(group *sync.WaitGroup, index int) {
			defer group.Done()
			c, _ := jsonrpc4go.NewClient("IntRpc", "tcp", addr)
			for j := range 100 {
				result := MakeRpcCallWithErrorCheck[int](t, c, "Add", index, j)
				if result != (index + j) {
					t.Errorf(EQUAL_MESSAGE_TEMPLETE, index, j, (index + j), result)
				}
			}
		}(&wg, i)
	}
	wg.Wait()
}

func TestFailConnect(t *testing.T) {
	// First point the client at a port nothing is listening on, to verify failure.
	c, _ := jsonrpc4go.NewClient("IntRpc", "tcp", "127.0.0.1:1")
	var err error
	for range 20 {
		_, err = jsonrpc4go.Call[int](c, "Add", 1, 2)
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
	for range 20 {
		_, err = jsonrpc4go.Call[int](c2, "Add", 1, 2)
		if err != nil {
			t.Errorf(EQUAL_MESSAGE_TEMPLETE, 1, 2, 3, err)
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
	for i := range 20 {
		wg.Add(1)
		go func(group *sync.WaitGroup, index int) {
			defer group.Done()
			c, _ := jsonrpc4go.NewClient("IntRpc", "tcp", addrList)
			for j := range 20 {
				result := MakeRpcCallWithErrorCheck[int](t, c, "Add", index, j)
				if result != (index + j) {
					t.Errorf(EQUAL_MESSAGE_TEMPLETE, index, j, (index + j), result)
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
	result := MakeRpcCallWithErrorCheck[int](t, c, "Add", 10, 11)
	if result != 21 {
		t.Errorf(EQUAL_MESSAGE_TEMPLETE, 10, 11, 21, result)
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
	result := MakeRpcCallWithErrorCheck[int](t, c, "Add", 10, 11)
	if result != 21 {
		t.Errorf(EQUAL_MESSAGE_TEMPLETE, 10, 11, 21, result)
	}
}
