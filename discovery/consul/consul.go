package consul

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/sunquakes/jsonrpc4go/discovery"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
)

type Consul struct {
	URL   *url.URL
	Token string
}

type HealthService struct {
	AggregatedStatus string  `json:"AggregatedStatus"`
	Service          Service `json:"Service"`
}

type Service struct {
	ID      string `json:"ID"`
	Service string `json:"Service"`
	Port    int    `json:"Port"`
	Address string `json:"Address"`
}

func NewConsul(rawURL string) (discovery.Driver, error) {
	URL, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}
	consul := &Consul{URL, URL.Query().Get("token")}
	return consul, err
}

func (d *Consul) Register(name string, protocol string, hostname string, port int) {
}

func (d *Consul) Get(name string) (string, error) {
	URL, err := GetURL(d.URL.Redacted(), "/v1/agent/health/service/name/"+name, d.Token)
	resp, err := http.Get(URL)
	defer resp.Body.Close()
	if resp.StatusCode != STATUS_CODE_PASSING {
		return "", errors.New(StatusCodeMap[resp.StatusCode])
	}
	body, err := ioutil.ReadAll(resp.Body)
	var hss []HealthService
	json.Unmarshal(body, &hss)
	ua := make([]string, len(hss))
	for _, v := range hss {
		ua = append(ua, fmt.Sprintf("%s:%d", v.Service.Address, v.Service.Port))
	}
	return strings.Join(ua, ",")[1:], err
}
