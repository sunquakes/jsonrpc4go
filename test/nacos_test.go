package test

import (
	"fmt"
	"github.com/sunquakes/jsonrpc4go/discovery/nacos"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNacosRequestURL(t *testing.T) {
	query := make(map[string]string)
	query["serviceName"] = "test"
	query["ip"] = "127.0.0.1"
	query["port"] = "3200"
	URL, err := nacos.GetURL("http://localhost:8849", "/nacos/v1/ns/instance", query)
	if err != nil {
		t.Error(err)
	}
	expected := "http://localhost:8849/nacos/v1/ns/instance?ip=127.0.0.1&port=3200&serviceName=test"
	if URL != expected {
		t.Errorf("URL expected be %s, but %s got", expected, URL)
	}
}

func TestNacosRegister(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, `ok`)
	}))
	defer ts.Close()
	r, err := nacos.NewNacos(ts.URL)
	// r, err := nacos.NewNacos("http://localhost:8849")
	if err != nil {
		t.Error(err)
	}
	err = r.Register("java_tcp", "tcp", "192.168.1.15", 3232)
	if err != nil {
		t.Error(err)
	}
}

func TestNacosGet(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, `{"name":"DEFAULT_GROUP@@java_tcp","groupName":"DEFAULT_GROUP","clusters":"","cacheMillis":10000,"hosts":[{"instanceId":"192.168.1.15#3232#DEFAULT#DEFAULT_GROUP@@java_tcp","ip":"192.168.1.15","port":3232,"weight":1.0,"healthy":true,"enabled":true,"ephemeral":true,"clusterName":"DEFAULT","serviceName":"DEFAULT_GROUP@@java_tcp","metadata":{},"instanceHeartBeatInterval":5000,"instanceHeartBeatTimeOut":15000,"ipDeleteTimeout":30000,"instanceIdGenerator":"simple"}],"lastRefTime":1673444367069,"checksum":"","allIPs":false,"reachProtectionThreshold":false,"valid":true}`)
	}))
	defer ts.Close()
	r, err := nacos.NewNacos(ts.URL)
	// r, err := nacos.NewNacos("http://localhost:8849")
	if err != nil {
		t.Error(err)
	}
	URL, err := r.Get("java_tcp")
	if err != nil {
		t.Error(err)
	}
	if URL != "192.168.1.15:3232" {
		t.Errorf("URL expected be %s, but %s got", "", "")
	}
}

func TestNacosBeat(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, `ok`)
	}))
	defer ts.Close()
	r, err := nacos.NewNacos(ts.URL)
	//r, err := nacos.NewNacos("http://localhost:8849")
	if err != nil {
		t.Error(err)
	}
	err = r.Register("java_tcp", "tcp", "192.168.1.15", 3232)
	if err != nil {
		t.Error(err)
	}
	select {}
}
