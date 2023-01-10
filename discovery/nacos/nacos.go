package nacos

import (
	"fmt"
	"github.com/sunquakes/jsonrpc4go/discovery"
	"net/http"
	"net/url"
	"strconv"
)

type Nacos struct {
	URL   *url.URL
	Token string
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
	instanceId := d.URL.Query().Get("instanceId")
	var ID string
	if instanceId == "" {
		ID = fmt.Sprintf("%s:%d", name, port)
	} else {
		ID = fmt.Sprintf("%s-%s:%d", name, instanceId, port)
	}
	query := make(map[string]string)
	query["serviceName"] = ID
	query["ip"] = hostname
	query["port"] = strconv.Itoa(port)
	URL, err := GetURL(d.URL.Redacted(), "/nacos/v1/ns/instance", query)
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
	fmt.Println(resp.StatusCode)
	return nil
}

func (d *Nacos) Get(name string) (string, error) {
	return "", nil
}
