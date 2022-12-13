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

type RegisterService struct {
	ID      string `json:"ID"`
	Name    string `json:"Name"`
	Port    int    `json:"Port"`
	Address string `json:"Address"`
}

type Check struct {
	ID        string `json:"ID"`
	Name      string `json:"Name"`
	Status    string `json:"Status"`
	ServiceID string `json:"ServiceID"`
	HTTP      string `json:"HTTP"`
	Method    string `json:"Method"`
	TCP       string `json:"TCP"`
	Interval  string `json:"Interval"`
	Timeout   string `json:"Timeout"`
}

func NewConsul(rawURL string) (discovery.Driver, error) {
	URL, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}
	consul := &Consul{URL, URL.Query().Get("token")}
	return consul, err
}

func (d *Consul) Register(name string, protocol string, hostname string, port int) error {
	// Get the instanceId from url
	instanceId := d.URL.Query().Get("instanceId")
	var ID string
	if instanceId == "" {
		ID = fmt.Sprintf("%s:%d", name, port)
	} else {
		ID = fmt.Sprintf("%s-%s:%d", name, instanceId, port)
	}
	service := &RegisterService{
		ID,
		name,
		port,
		hostname,
	}
	URL, err := GetURL(d.URL.Redacted(), "/v1/agent/service/register", d.Token)
	if err != nil {
		return err
	}
	b, err := json.Marshal(service)
	if err != nil {
		return err
	}
	req, err := http.NewRequest("PUT", URL, strings.NewReader(string(b)))
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != STATUS_CODE_PASSING {
		return errors.New(StatusCodeMap[resp.StatusCode])
	}
	check := d.URL.Query().Get("check")
	if check == "true" {
		interval := d.URL.Query().Get("interval")
		if interval == "" {
			interval = "30s"
		}
		timeout := d.URL.Query().Get("timeout")
		if timeout == "" {
			timeout = "10s"
		}
		var http, method, tcp string
		if protocol == "http" || protocol == "https" {
			http = fmt.Sprintf("%s://%s:%d", protocol, hostname, port)
		} else if protocol == "tcp" {
			tcp = fmt.Sprintf("%s:%d", hostname, port)
		}
		check := &Check{
			ID,
			name,
			"passing",
			ID,
			http,
			method,
			tcp,
			interval,
			timeout,
		}
		err := d.Check(check)
		if err != nil {
			return err
		}
	}
	return nil
}

func (d *Consul) Get(name string) (string, error) {
	URL, err := GetURL(d.URL.Redacted(), "/v1/agent/health/service/name/"+name, d.Token)
	if err != nil {
		return "", err
	}
	resp, err := http.Get(URL)
	if err != nil {
		return "", err
	}
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

func (d *Consul) Check(check *Check) error {
	URL, err := GetURL(d.URL.Redacted(), "/v1/agent/check/register", d.Token)
	if err != nil {
		return err
	}
	b, err := json.Marshal(check)
	if err != nil {
		return err
	}
	req, err := http.NewRequest("PUT", URL, strings.NewReader(string(b)))
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != STATUS_CODE_PASSING {
		return errors.New(StatusCodeMap[resp.StatusCode])
	}
	return nil
}
