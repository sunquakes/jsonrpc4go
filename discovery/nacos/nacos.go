package nacos

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/sunquakes/jsonrpc4go/discovery"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const IS_EPHEMERAL = "true"
const HEARTBEAT_INTERVAL = 5

type Nacos struct {
	URL           *url.URL
	Token         string
	Ephemeral     string
	HeartbeatList []Service
}

type GetResp struct {
	Hosts []Service `json:"hosts"`
}

type Service struct {
	Ip         string `json:"ip"`
	Port       int    `json:"port"`
	Healthy    bool   `json:"healthy"`
	InstanceId string `json:"instanceId"`
}

func NewNacos(rawURL string) (discovery.Driver, error) {
	URL, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}
	ephemeral := IS_EPHEMERAL
	if URL.Query().Has("ephemeral") {
		ephemeral = URL.Query().Get("ephemeral")

	}
	nacos := &Nacos{URL, URL.Query().Get("token"), ephemeral, make([]Service, 0)}
	return nacos, err
}

func (d *Nacos) Register(name string, protocol string, hostname string, port int) error {
	query := make(map[string]string)
	// Get the instanceId from url
	query["serviceName"] = name
	query["ip"] = hostname
	query["port"] = strconv.Itoa(port)
	queries := d.URL.Query()
	if queries != nil {
		for k, v := range queries {
			if len(v) > 0 {
				query[k] = v[0]
			}
		}
	}
	query["ephemeral"] = d.Ephemeral
	URL, err := GetURL(d.URL.Redacted(), "/nacos/v1/ns/instance", query)
	if err != nil {
		return err
	}
	req, err := http.NewRequest("POST", URL, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != STATUS_CODE_PASSING {
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		return errors.New(string(body))
	}
	if d.Ephemeral == "false" {
		if len(d.HeartbeatList) == 0 {
			d.Heartbeat()
		}
		d.HeartbeatList = append(d.HeartbeatList, Service{hostname, port, true, name})
	}
	return nil
}

func (d *Nacos) Get(name string) (string, error) {
	query := make(map[string]string)
	query["serviceName"] = name
	URL, err := GetURL(d.URL.Redacted(), "/nacos/v1/ns/instance/list", query)
	if err != nil {
		return "", err
	}
	resp, err := http.Get(URL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if resp.StatusCode != STATUS_CODE_PASSING {
		if err != nil {
			return "", err
		}
		return "", errors.New(string(body))
	}
	var gr GetResp
	json.Unmarshal(body, &gr)
	ua := make([]string, 0)
	for _, v := range gr.Hosts {
		if !v.Healthy {
			continue
		}
		ua = append(ua, fmt.Sprintf("%s:%d", v.Ip, v.Port))
	}
	if len(ua) == 0 {
		return "", errors.New("Unable to get service url.")
	}
	return strings.Join(ua, ",")[1:], err
}

func (d *Nacos) Beat(name string, hostname string, port int) error {
	query := make(map[string]string)
	// Get the instanceId from url
	query["serviceName"] = name
	query["ip"] = hostname
	query["port"] = strconv.Itoa(port)
	queries := d.URL.Query()
	if queries != nil {
		for k, v := range queries {
			if len(v) > 0 {
				query[k] = v[0]
			}
		}
	}
	query["ephemeral"] = d.Ephemeral
	URL, err := GetURL(d.URL.Redacted(), "/nacos/v1/ns/instance/beat", query)
	if err != nil {
		return err
	}
	req, err := http.NewRequest("PUT", URL, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != STATUS_CODE_PASSING {
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		return errors.New(string(body))
	}
	return nil
}

func (d *Nacos) Heartbeat() error {
	go func() {
		for {
			for _, service := range d.HeartbeatList {
				err := d.Beat(service.InstanceId, service.Ip, service.Port)
				if err != nil {
					log.Panic(err)
				}
			}
			time.Sleep(time.Second * HEARTBEAT_INTERVAL)
		}
	}()
	return nil
}
