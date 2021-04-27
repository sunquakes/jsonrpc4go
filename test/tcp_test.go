package test

import (
	"encoding/json"
	"errors"
	"github.com/sunquakes/jsonrpc4go"
	"github.com/sunquakes/jsonrpc4go/client"
	"github.com/sunquakes/jsonrpc4go/common"
	"github.com/sunquakes/jsonrpc4go/server"
	"testing"
	"time"
)

func TestTcpCall(t *testing.T) {
	go func() {
		s, _ := jsonrpc4go.NewServer("tcp", "127.0.0.1", "3234")
		s.Register(new(IntRpc))
		s.Start()
	}()
	time.Sleep(time.Duration(2) * time.Second)
	s, _ := jsonrpc4go.NewClient("tcp", "127.0.0.1", "3234")
	params := Params{1, 2}
	result := new(Result)
	s.Call("IntRpc.Add", &params, result, false)
	if *result != 3 {
		t.Errorf("%d + %d expected be %d, but %d got", params.A, params.B, 3, *result)
	}
}

func TestTcpCallMethod(t *testing.T) {
	go func() {
		s, _ := jsonrpc4go.NewServer("tcp", "127.0.0.1", "3239")
		s.Register(new(IntRpc))
		s.Start()
	}()
	time.Sleep(time.Duration(2) * time.Second)
	c, _ := jsonrpc4go.NewClient("tcp", "127.0.0.1", "3239")
	params := Params{1, 2}
	result := new(Result)
	c.Call("int_rpc/Add", &params, result, false)
	if *result != 3 {
		t.Errorf("%d + %d expected be %d, but %d got", params.A, params.B, 3, *result)
	}
}

func TestTcpNotifyCall(t *testing.T) {
	go func() {
		s, _ := jsonrpc4go.NewServer("tcp", "127.0.0.1", "3235")
		s.Register(new(IntRpc))
		s.Start()
	}()
	time.Sleep(time.Duration(2) * time.Second)
	s, _ := jsonrpc4go.NewClient("tcp", "127.0.0.1", "3235")
	params := Params{2, 3}
	result := new(Result)
	s.Call("IntRpc.Add", &params, result, true)
	if *result != 5 {
		t.Errorf("%d + %d expected be %d, but %d got", params.A, params.B, 5, *result)
	}
}

func TestTcpBatchCall(t *testing.T) {
	go func() {
		s, _ := jsonrpc4go.NewServer("tcp", "127.0.0.1", "3237")
		s.Register(new(IntRpc))
		s.Start()
	}()
	time.Sleep(time.Duration(2) * time.Second)
	c, _ := jsonrpc4go.NewClient("tcp", "127.0.0.1", "3237")

	result1 := new(Result)
	err1 := c.BatchAppend("IntRpc/Add1", Params{1, 6}, result1, false)
	result2 := new(Result)
	err2 := c.BatchAppend("IntRpc/Add", Params{2, 3}, result2, false)
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
		s, _ := jsonrpc4go.NewServer("tcp", "127.0.0.1", "3220")
		s.SetOptions(server.TcpOptions{"aaaaaa", 2 * 1024 * 1024})
		s.Register(new(IntRpc))
		s.Start()
	}()
	time.Sleep(time.Duration(2) * time.Second)
	s, _ := jsonrpc4go.NewClient("tcp", "127.0.0.1", "3220")
	s.SetOptions(client.TcpOptions{"aaaaaa", 2 * 1024 * 1024})
	params := Params{1, 2}
	result := new(Result)
	s.Call("IntRpc.Add", &params, result, false)
	if *result != 3 {
		t.Errorf("%d + %d expected be %d, but %d got", params.A, params.B, 3, *result)
	}
}

func TestSetHooks(t *testing.T) {
	params := Params{1, 2}
	go func() {
		s, _ := jsonrpc4go.NewServer("tcp", "127.0.0.1", "3221")
		s.SetBeforeFunc(func(id interface{}, method string, p interface{}) error {
			if method != "IntRpc.Add" {
				t.Errorf("Method expected be %s, but %s got", "IntRpc.Add", method)
			}
			if p.(Params) != params {
				jsonParams, _ := json.Marshal(params)
				jsonP, _ := json.Marshal(p)
				t.Errorf("Params expected be %s, but %s got", jsonParams, jsonP)
			}
			return nil
		})
		s.SetAfterFunc(func(id interface{}, method string, r interface{}) error {
			if method != "IntRpc.Add" {
				t.Errorf("Method expected be %s, but %s got", "IntRpc.Add", method)
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
	s, _ := jsonrpc4go.NewClient("tcp", "127.0.0.1", "3221")
	result := new(Result)
	s.Call("IntRpc.Add", &params, result, false)
	if *result != 3 {
		t.Errorf("%d + %d expected be %d, but %d got", params.A, params.B, 3, *result)
	}
}

func TestSetHooksCustomError(t *testing.T) {
	params := Params{1, 2}
	go func() {
		s, _ := jsonrpc4go.NewServer("tcp", "127.0.0.1", "3222")
		s.SetBeforeFunc(func(id interface{}, method string, p interface{}) error {
			return errors.New("Custom Error")
		})
		s.Register(new(IntRpc))
		s.Start()
	}()
	time.Sleep(time.Duration(2) * time.Second)
	s, _ := jsonrpc4go.NewClient("tcp", "127.0.0.1", "3222")
	result := new(Result)
	err := s.Call("IntRpc.Add", &params, result, false)
	if err.Error() != "Custom Error" {
		t.Errorf("Error expected be %s, but %s got", "Custom Error", err.Error())
	}
}
