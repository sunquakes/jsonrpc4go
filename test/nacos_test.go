package test

import (
	"github.com/sunquakes/jsonrpc4go/discovery/nacos"
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

// func TestNacosGet(t *testing.T) {
// 	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
// 		fmt.Fprintln(w, `[{"AggregatedStatus":"passing","Service":{"ID":"java_http-2:3202","Service":"java_http","Tags":[],"Meta":{},"Port":3202,"Address":"10.222.1.164","TaggedAddresses":{"lan_ipv4":{"Address":"10.222.1.164","Port":3202},"wan_ipv4":{"Address":"10.222.1.164","Port":3202}},"Weights":{"Passing":1,"Warning":1},"EnableTagOverride":false,"Datacenter":"dc1"},"Checks":[{"Node":"1ae846e40d15","CheckID":"service:java_http-2:3202","Name":"Service 'java_http' check","Status":"passing","Notes":"","Output":"HTTP GET http://10.222.1.164:3202: 200 OK Output: ","ServiceID":"java_http-2:3202","ServiceName":"java_http","ServiceTags":null,"Type":"","ExposedPort":0,"Definition":{"Interval":"0s","Timeout":"0s","DeregisterCriticalServiceAfter":"0s","HTTP":"","Header":null,"Method":"","Body":"","TLSServerName":"","TLSSkipVerify":false,"TCP":"","UDP":"","GRPC":"","GRPCUseTLS":false},"CreateIndex":0,"ModifyIndex":0}]}]`)
// 	}))
// 	defer ts.Close()
// 	r, err := nacos.NewConsul(ts.URL)
// 	if err != nil {
// 		t.Error(err)
// 	}
// 	URL, err := r.Get("java_http")
// 	if err != nil {
// 		t.Error(err)
// 	}
// 	if URL != "10.222.1.164:3202" {
// 		t.Errorf("URL expected be %s, but %s got", "", "")
// 	}
// }
//
func TestNacosRegister(t *testing.T) {
	// ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	// 	fmt.Fprintln(w, ``)
	// }))
	// defer ts.Close()
	// r, err := nacos.NewNacos(ts.URL)
	r, err := nacos.NewNacos("http://localhost:8849")
	if err != nil {
		t.Error(err)
	}
	err = r.Register("java_tcp", "tcp", "192.168.1.15", 3232)
	if err != nil {
		t.Error(err)
	}
}

// func TestNacosCheck(t *testing.T) {
// 	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
// 		fmt.Fprintln(w, ``)
// 	}))
// 	defer ts.Close()
// 	URL, err := url.Parse(ts.URL)
// 	if err != nil {
// 		t.Error(err)
// 	}
// 	r := &nacos.Consul{URL, URL.Query().Get("token")}
// 	if err != nil {
// 		t.Error(err)
// 	}
// 	err = r.Check(&nacos.Check{
// 		"java_tcp:9999",
// 		"java_tcp",
// 		"passing",
// 		"java_tcp:9999",
// 		"",
// 		"",
// 		"localhost:9999",
// 		"10s",
// 		"10s",
// 	})
// 	if err != nil {
// 		t.Error(err)
// 	}
// }
//
