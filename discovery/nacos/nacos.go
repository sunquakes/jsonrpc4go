package nacos

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/sunquakes/jsonrpc4go/discovery"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

type Nacos struct {
	URL   *url.URL
	Token string
}

type GetResp struct {
	Hosts []Service `json:"hosts"`
}

type Service struct {
	InstanceId string `json:"instanceId"`
	Healthy    bool   `json:"healthy"`
	Port       int    `json:"port"`
	Ip         string `json:"ip"`
}

func NewNacos(rawURL string) (discovery.Driver, error) {
	URL, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}
	consul := &Nacos{URL, URL.Query().Get("token")}
	return consul, err
}

func (d *Nacos) Register(name string, protocol string, hostname string, port int) error {
	// Get the instanceId from url
	query := make(map[string]string)
	query["serviceName"] = name
	query["ip"] = hostname
	query["port"] = strconv.Itoa(port)
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
	ua := make([]string, len(gr.Hosts))
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
