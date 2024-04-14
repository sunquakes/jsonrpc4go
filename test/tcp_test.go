package test

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/sunquakes/jsonrpc4go"
	"github.com/sunquakes/jsonrpc4go/client"
	"github.com/sunquakes/jsonrpc4go/common"
	"github.com/sunquakes/jsonrpc4go/discovery/consul"
	"github.com/sunquakes/jsonrpc4go/discovery/nacos"
	"github.com/sunquakes/jsonrpc4go/server"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func TestTcpCall(t *testing.T) {
	s, _ := jsonrpc4go.NewServer("tcp", 3601)
	s.Register(new(IntRpc))
	go func() {
		s.Start()
	}()
	<-s.GetEvent()
	c, _ := jsonrpc4go.NewClient("IntRpc", "tcp", "127.0.0.1:3601")
	params := Params{1, 2}
	result := new(Result)
	c.Call("Add", &params, result, false)
	if *result != 3 {
		t.Errorf("%d + %d expected be %d, but %d got", params.A, params.B, 3, *result)
	}
}

func TestTcpCallMethod(t *testing.T) {
	go func() {
		s, _ := jsonrpc4go.NewServer("tcp", 3602)
		s.Register(new(IntRpc))
		s.Start()
	}()
	time.Sleep(time.Duration(2) * time.Second)
	c, _ := jsonrpc4go.NewClient("IntRpc", "tcp", "127.0.0.1:3602")
	params := Params{1, 2}
	result := new(Result)
	c.Call("Add", &params, result, false)
	if *result != 3 {
		t.Errorf("%d + %d expected be %d, but %d got", params.A, params.B, 3, *result)
	}
}

func TestTcpNotifyCall(t *testing.T) {
	go func() {
		s, _ := jsonrpc4go.NewServer("tcp", 3603)
		s.Register(new(IntRpc))
		s.Start()
	}()
	time.Sleep(time.Duration(2) * time.Second)
	s, _ := jsonrpc4go.NewClient("IntRpc", "tcp", "127.0.0.1:3603")
	params := Params{2, 3}
	result := new(Result)
	s.Call("Add", &params, result, true)
	if *result != 5 {
		t.Errorf("%d + %d expected be %d, but %d got", params.A, params.B, 5, *result)
	}
}

func TestTcpBatchCall(t *testing.T) {
	go func() {
		s, _ := jsonrpc4go.NewServer("tcp", 3604)
		s.Register(new(IntRpc))
		s.Start()
	}()
	time.Sleep(time.Duration(2) * time.Second)
	c, _ := jsonrpc4go.NewClient("IntRpc", "tcp", "127.0.0.1:3604")

	result1 := new(Result)
	err1 := c.BatchAppend("Add1", Params{1, 6}, result1, false)
	result2 := new(Result)
	err2 := c.BatchAppend("Add", Params{2, 3}, result2, false)
	c.BatchCall()

	if *err2 != nil || *result2 != 5 {
		t.Errorf("%d + %d expected be %d, but %d got", 2, 3, 5, result2)
	}
	if (*err1).Error() != common.CodeMap[common.MethodNotFound] {
		t.Errorf("Error message expected be %s, but %s got", common.CodeMap[common.MethodNotFound], (*err1).Error())
	}
}

func TestSetOption(t *testing.T) {
	go func() {
		s, _ := jsonrpc4go.NewServer("tcp", 3605)
		s.SetOptions(server.TcpOptions{"aaaaaa", 2 * 1024 * 1024})
		s.Register(new(IntRpc))
		s.Start()
	}()
	time.Sleep(time.Duration(2) * time.Second)
	s, _ := jsonrpc4go.NewClient("IntRpc", "tcp", "127.0.0.1:3605")
	s.SetOptions(client.TcpOptions{"aaaaaa", 2 * 1024 * 1024})
	params := Params{1, 2}
	result := new(Result)
	s.Call("Add", &params, result, false)
	if *result != 3 {
		t.Errorf("%d + %d expected be %d, but %d got", params.A, params.B, 3, *result)
	}
}

func TestSetHooks(t *testing.T) {
	params := Params{1, 2}
	go func() {
		s, _ := jsonrpc4go.NewServer("tcp", 3606)
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
				t.Errorf("Result expected be %d, but %d got", 3, r)
			}
			return nil
		})
		s.Register(new(IntRpc))
		s.Start()
	}()
	time.Sleep(time.Duration(2) * time.Second)
	s, _ := jsonrpc4go.NewClient("IntRpc", "tcp", "127.0.0.1:3606")
	result := new(Result)
	s.Call("Add", &params, result, false)
	if *result != 3 {
		t.Errorf("%d + %d expected be %d, but %d got", params.A, params.B, 3, *result)
	}
}

func TestSetHooksCustomError(t *testing.T) {
	params := Params{1, 2}
	go func() {
		s, _ := jsonrpc4go.NewServer("tcp", 3607)
		s.SetBeforeFunc(func(id any, method string, p any) error {
			return errors.New("Custom Error")
		})
		s.Register(new(IntRpc))
		s.Start()
	}()
	time.Sleep(time.Duration(2) * time.Second)
	s, _ := jsonrpc4go.NewClient("IntRpc", "tcp", "127.0.0.1:3607")
	result := new(Result)
	err := s.Call("Add", &params, result, false)
	if err.Error() != "Custom Error" {
		t.Errorf("Error expected be %s, but %s got", "Custom Error", err.Error())
	}
}

func TestRateLimit(t *testing.T) {
	params := Params{1, 2}
	go func() {
		s, _ := jsonrpc4go.NewServer("tcp", 3608)
		s.Register(new(IntRpc))
		s.SetRateLimit(0.2, 1)
		s.Start()
	}()
	time.Sleep(time.Duration(5) * time.Second)
	s, _ := jsonrpc4go.NewClient("IntRpc", "tcp", "127.0.0.1:3608")
	result := new(Result)
	err := s.Call("Add", &params, result, false)
	if err != nil {
		t.Errorf("Error expected be %s, but %s got", "nil", err.Error())
	}
	err = s.Call("Add", &params, result, false)
	if err.Error() != "Too many requests" {
		t.Errorf("Error expected be %s, but %s got", "Too many requests", err.Error())
	}
	time.Sleep(time.Duration(5) * time.Second)
	err = s.Call("Add", &params, result, false)
	if err != nil {
		t.Errorf("Error expected be %s, but %s got", "nil", err.Error())
	}
}

type LongRpc struct{}

type LongParams struct {
	A string `json:"a"`
	B string `json:"b"`
}

type LongResult = string

func (i *LongRpc) Add(params *LongParams, result *LongResult) error {
	a := params.A + params.B
	*result = any(a).(LongResult)
	return nil
}

func TestLongPackageTcpCall(t *testing.T) {
	go func() {
		s, _ := jsonrpc4go.NewServer("tcp", 3609)
		s.SetOptions(server.TcpOptions{"\r\n", 2 * 1024 * 1024})
		s.Register(new(LongRpc))
		s.Start()
	}()
	time.Sleep(time.Duration(2) * time.Second)
	var wg sync.WaitGroup
	for i := 0; i < 11; i++ {
		wg.Add(1)
		go func(group *sync.WaitGroup) {
			defer group.Done()
			c, _ := jsonrpc4go.NewClient("LongRpc", "tcp", "127.0.0.1:3609")
			c.SetOptions(client.TcpOptions{"\r\n", 2 * 1024 * 1024})
			params := LongParams{LongString1, LongString2}
			result := new(LongResult)
			for j := 0; j < 100; j++ {
				c.Call("Add", &params, result, false)
				ls := LongString1 + LongString2
				if *result != ls {
					t.Errorf("%s + %s expected be %s, but %s got", params.A, params.B, ls, *result)
				}
			}
			time.Sleep(time.Duration(2) * time.Second)
			for j := 0; j < 100; j++ {
				c.Call("Add", &params, result, false)
				ls := LongString1 + LongString2
				if *result != ls {
					t.Errorf("%s + %s expected be %s, but %s got", params.A, params.B, ls, *result)
				}
			}

		}(&wg)
	}
	wg.Wait()
}

func TestCoTcpCall(t *testing.T) {
	go func() {
		s, _ := jsonrpc4go.NewServer("tcp", 3610)
		s.Register(new(IntRpc))
		s.Start()
	}()
	time.Sleep(time.Duration(2) * time.Second)

	var wg sync.WaitGroup
	for i := 0; i < 40; i++ {
		wg.Add(1)
		go func(group *sync.WaitGroup) {
			defer group.Done()
			c, _ := jsonrpc4go.NewClient("IntRpc", "tcp", "127.0.0.1:3610")
			for j := 0; j < 100; j++ {
				params := Params{i, j}
				result := new(Result)
				c.Call("Add", &params, result, false)
				if *result != (i + j) {
					t.Errorf("%d + %d expected be %d, but %d got", params.A, params.B, (i + j), *result)
				}
			}
		}(&wg)
	}
	wg.Wait()
}

func TestFailConnect(t *testing.T) {
	var err error
	params := Params{1, 2}
	s, _ := jsonrpc4go.NewClient("IntRpc", "tcp", "127.0.0.1:3611")
	result := new(Result)
	for i := 0; i < 20; i++ {
		err = s.Call("Add", &params, result, false)
		if err == nil {
			t.Errorf("Error expected be not nil, but nil got")
		}
	}
	go func() {
		s, _ := jsonrpc4go.NewServer("tcp", 3611)
		s.Register(new(IntRpc))
		s.Start()
	}()
	time.Sleep(time.Duration(2) * time.Second)
	for i := 0; i < 20; i++ {
		err = s.Call("Add", &params, result, false)
		if err != nil {
			t.Errorf("%d + %d expected be %d, but %d got", params.A, params.B, 3, *result)
		}
	}
}

func TestRibbonTcpCall(t *testing.T) {
	go func() {
		s, _ := jsonrpc4go.NewServer("tcp", 3612)
		s.Register(new(IntRpc))
		s.Start()
	}()
	go func() {
		s, _ := jsonrpc4go.NewServer("tcp", 3613)
		s.Register(new(IntRpc))
		s.Start()
	}()
	time.Sleep(time.Duration(2) * time.Second)

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(group *sync.WaitGroup) {
			defer group.Done()
			c, _ := jsonrpc4go.NewClient("IntRpc", "tcp", "127.0.0.1:3612,127.0.0.1:3613")
			for j := 0; j < 20; j++ {
				params := Params{i, j}
				result := new(Result)
				c.Call("Add", &params, result, false)
				if *result != (i + j) {
					t.Errorf("%d + %d expected be %d, but %d got", params.A, params.B, (i + j), *result)
				}
			}
		}(&wg)
	}
	wg.Wait()
}

func TestTcpConsul(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, `[{"AggregatedStatus":"passing","Service":{"ID":"IntRpc:3614","Service":"IntRpc","Tags":[],"Meta":{},"Port":3614,"Address":"127.0.0.1","TaggedAddresses":{"lan_ipv4":{"Address":"127.0.0.1","Port":3614},"wan_ipv4":{"Address":"127.0.0.1","Port":3614}},"Weights":{"Passing":1,"Warning":1},"EnableTagOverride":false,"Datacenter":"dc1"},"Checks":[{"Node":"1ae846e40d15","CheckID":"service:IntRpc:3614","Name":"Service 'IntRpc' check","Status":"passing","Notes":"","Output":"HTTP GET http://127.0.0.1:3614: 200 OK Output: ","ServiceID":"IntRpc:3614","ServiceName":"IntRpc","ServiceTags":null,"Type":"","ExposedPort":0,"Definition":{"Interval":"0s","Timeout":"0s","DeregisterCriticalServiceAfter":"0s","HTTP":"","Header":null,"Method":"","Body":"","TLSServerName":"","TLSSkipVerify":false,"TCP":"","UDP":"","GRPC":"","GRPCUseTLS":false},"CreateIndex":0,"ModifyIndex":0}]}]`)
	}))
	dc, err := consul.NewConsul(ts.URL)
	// dc, err := consul.NewConsul("http://localhost:8500?check=false&instanceId=1&interval=10s")
	if err != nil {
		t.Errorf(err.Error())
	}
	go func() {
		s, _ := jsonrpc4go.NewServer("tcp", 3614)
		s.SetDiscovery(dc, "")
		s.Register(new(IntRpc))
		s.Start()
	}()
	time.Sleep(time.Duration(2) * time.Second)

	c, _ := jsonrpc4go.NewClient("IntRpc", "tcp", dc)
	params := Params{10, 11}
	result := new(Result)
	c.Call("Add", &params, result, false)
	if *result != 21 {
		t.Errorf("%d + %d expected be %d, but %d got", params.A, params.B, 21, *result)
	}
}

func TestTcpNacos(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, `{"name":"DEFAULT_GROUP@@java_tcp","groupName":"DEFAULT_GROUP","clusters":"","cacheMillis":10000,"hosts":[{"instanceId":"127.0.0.1#3616#DEFAULT#DEFAULT_GROUP@@java_tcp","ip":"127.0.0.1","port":3616,"weight":1.0, "healthy":true,"enabled":true,"ephemeral":true,"clusterName":"DEFAULT","serviceName":"DEFAULT_GROUP@@java_tcp","metadata":{},"instanceHeartBeatInterval":5000,"instanceHeartBeatTimeOut":15000,"ipDeleteTimeout":30000, "instanceIdGenerator":"simple"}],"lastRefTime":1673444367069,"checksum":"","allIPs":false,"reachProtectionThreshold":false,"valid":true}`)
	}))
	dc, err := nacos.NewNacos(ts.URL)
	// dc, err := nacos.NewNacos("http://localhost:8849?namespaceId=79f14f4e-f5e8-46b6-90b9-0ad105b8626d&groupName=test1")
	if err != nil {
		t.Errorf(err.Error())
	}
	go func() {
		s, _ := jsonrpc4go.NewServer("tcp", 3616)
		s.SetDiscovery(dc, "")
		s.Register(new(IntRpc))
		s.Start()
	}()
	time.Sleep(time.Duration(2) * time.Second)

	c, _ := jsonrpc4go.NewClient("IntRpc", "tcp", dc)
	params := Params{10, 11}
	result := new(Result)
	c.Call("Add", &params, result, false)
	if *result != 21 {
		t.Errorf("%d + %d expected be %d, but %d got", params.A, params.B, 21, *result)
	}
}
