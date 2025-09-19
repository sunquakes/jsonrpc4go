package test

import (
	"testing"
	"time"

	"github.com/sunquakes/jsonrpc4go"
	"github.com/sunquakes/jsonrpc4go/client"
	"github.com/sunquakes/jsonrpc4go/server"
)

func TestHttpsCall(t *testing.T) {
	s, _ := jsonrpc4go.NewServer("https", 3621)
	s.Register(new(IntRpc))
	go func() {
		s.SetOptions(server.HttpOptions{KeyPath: "E:\\ruixinglong\\jsonrpc4go\\test\\secure\\server.key", CertPath: "E:\\ruixinglong\\jsonrpc4go\\test\\secure\\server.crt"})
		s.Start()
	}()
	<-s.GetEvent()
	time.Sleep(time.Duration(3600) * time.Second)
	c, _ := jsonrpc4go.NewClient("IntRpc", "https", "127.0.0.1:3621")
	c.SetOptions(&client.HttpOptions{CaPath: "E:\\ruixinglong\\jsonrpc4go\\test\\secure\\ca.cer"})
	params := Params{1, 2}
	result := new(Result)
	_ = c.Call("Add", &params, result, false)
	if *result != 3 {
		t.Errorf(EQUAL_MESSAGE_TEMPLETE, params.A, params.B, 3, *result)
	}
}
