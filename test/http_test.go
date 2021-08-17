package test

import (
	"github.com/sunquakes/jsonrpc4go"
	"github.com/sunquakes/jsonrpc4go/common"
	"testing"
	"time"
)

type IntRpc struct{}

type Params struct {
	A int `json:"a"`
	B int `json:"b"`
}

type Result = int

func (i *IntRpc) Add(params *Params, result *Result) error {
	a := params.A + params.B
	*result = interface{}(a).(Result)
	return nil
}

func TestHttpCall(t *testing.T) {
	go func() {
		s, _ := jsonrpc4go.NewServer("http", "127.0.0.1", "3201")
		s.Register(new(IntRpc))
		s.Start()
	}()
	time.Sleep(time.Duration(2) * time.Second)
	c, _ := jsonrpc4go.NewClient("http", "127.0.0.1", "3201")
	params := Params{1, 2}
	result := new(Result)
	c.Call("IntRpc.Add", &params, result, false)
	if *result != 3 {
		t.Errorf("%d + %d expected be %d, but %d got", params.A, params.B, 3, *result)
	}
}

func TestHttpCallMethod(t *testing.T) {
	go func() {
		s, _ := jsonrpc4go.NewServer("http", "127.0.0.1", "3202")
		s.Register(new(IntRpc))
		s.Start()
	}()
	time.Sleep(time.Duration(2) * time.Second)
	c, _ := jsonrpc4go.NewClient("http", "127.0.0.1", "3202")
	params := Params{1, 2}
	result := new(Result)
	c.Call("int_rpc/Add", &params, result, false)
	if *result != 3 {
		t.Errorf("%d + %d expected be %d, but %d got", params.A, params.B, 3, *result)
	}
}

func TestHttpNotifyCall(t *testing.T) {
	go func() {
		s, _ := jsonrpc4go.NewServer("http", "127.0.0.1", "3203")
		s.Register(new(IntRpc))
		s.Start()
	}()
	time.Sleep(time.Duration(2) * time.Second)
	c, _ := jsonrpc4go.NewClient("http", "127.0.0.1", "3203")
	params := Params{2, 3}
	result := new(Result)
	c.Call("IntRpc.Add", &params, result, true)
	if *result != 5 {
		t.Errorf("%d + %d expected be %d, but %d got", params.A, params.B, 5, *result)
	}
}

func TestHttpBatchCall(t *testing.T) {
	go func() {
		s, _ := jsonrpc4go.NewServer("http", "127.0.0.1", "3204")
		s.Register(new(IntRpc))
		s.Start()
	}()
	time.Sleep(time.Duration(2) * time.Second)
	c, _ := jsonrpc4go.NewClient("http", "127.0.0.1", "3204")

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

func TestHttpRateLimit(t *testing.T) {
	params := Params{1, 2}
	go func() {
		s, _ := jsonrpc4go.NewServer("http", "127.0.0.1", "3205")
		s.Register(new(IntRpc))
		s.SetRateLimit(0.5, 1)
		s.Start()
	}()
	time.Sleep(time.Duration(2) * time.Second)
	s, _ := jsonrpc4go.NewClient("http", "127.0.0.1", "3205")
	result := new(Result)
	err := s.Call("IntRpc.Add", &params, result, false)
	if err != nil {
		t.Errorf("Error expected be %s, but %s got", "nil", err.Error())
	}
	err = s.Call("IntRpc.Add", &params, result, false)
	if err.Error() != "Too many requests" {
		t.Errorf("Error expected be %s, but %s got", "Too many requests", err.Error())
	}
	time.Sleep(time.Duration(2) * time.Second)
	err = s.Call("IntRpc.Add", &params, result, false)
	if err != nil {
		t.Errorf("Error expected be %s, but %s got", "nil", err.Error())
	}
}