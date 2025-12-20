package consul

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/sunquakes/jsonrpc4go/discovery"
)

/**
 * @Description: Consul client structure, implements discovery.Driver interface
 * @Field URL: Consul server URL address
 * @Field Token: Authentication token
 */
type Consul struct {
	URL   *url.URL
	Token string
}

/**
 * @Description: Health service structure
 * @Field AggregatedStatus: Aggregated status
 * @Field Service: Service information
 */
type HealthService struct {
	AggregatedStatus string  `json:"AggregatedStatus"`
	Service          Service `json:"Service"`
}

/**
 * @Description: Service structure
 * @Field ID: Service ID
 * @Field Service: Service name
 * @Field Port: Port number
 * @Field Address: Service address
 */
type Service struct {
	ID      string `json:"ID"`
	Service string `json:"Service"`
	Port    int    `json:"Port"`
	Address string `json:"Address"`
}

/**
 * @Description: Register service structure
 * @Field ID: Service ID
 * @Field Name: Service name
 * @Field Port: Port number
 * @Field Address: Service address
 */
type RegisterService struct {
	ID      string `json:"ID"`
	Name    string `json:"Name"`
	Port    int    `json:"Port"`
	Address string `json:"Address"`
}

/**
 * @Description: Health check structure
 * @Field ID: Check ID
 * @Field Name: Check name
 * @Field Status: Check status
 * @Field ServiceID: Service ID
 * @Field HTTP: HTTP check address
 * @Field Method: HTTP check method
 * @Field TCP: TCP check address
 * @Field Interval: Check interval
 * @Field Timeout: Check timeout
 */
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

/**
 * @Description: Create Consul client instance
 * @Param rawURL: Consul server URL address
 * @Return discovery.Driver: Service discovery driver instance
 * @Return error: Error message
 */
func NewConsul(rawURL string) (discovery.Driver, error) {
	URL, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}
	consul := &Consul{URL, URL.Query().Get("token")}
	return consul, err
}

/**
 * @Description: Register service
 * @Receiver d: Consul structure pointer
 * @Param name: Service name
 * @Param protocol: Protocol type
 * @Param hostname: Hostname
 * @Param port: Port number
 * @Return error: Error message
 */
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
	d.Check(ID, name, protocol, hostname, port)
	return nil
}

/**
 * @Description: Check enable flag
 */
const (
	CHECK_TRUE = "true"
	/**
	 * @Description: Default check interval
	 */
	DEFAULT_INTERVAL = "30s"
	/**
	 * @Description: Default check timeout
	 */
	DEFAULT_TIMEOUT = "10s"
	/**
	 * @Description: HTTP protocol
	 */
	PROTOCOL_HTTP = "http"
	/**
	 * @Description: HTTPS protocol
	 */
	PROTOCOL_HTTPS = "https"
	/**
	 * @Description: TCP protocol
	 */
	PROTOCOL_TCP = "tcp"
	/**
	 * @Description: Check status - passing
	 */
	CHECK_STATUS_PASSING = "passing"
)

/**
 * @Description: Set service health check
 * @Receiver d: Consul structure pointer
 * @Param ID: Service ID
 * @Param name: Service name
 * @Param protocol: Protocol type
 * @Param hostname: Hostname
 * @Param port: Port number
 * @Return error: Error message
 */
func (d *Consul) Check(ID string, name string, protocol string, hostname string, port int) error {
	check := d.URL.Query().Get("check")
	if check == CHECK_TRUE {
		interval := d.URL.Query().Get("interval")
		if interval == "" {
			interval = DEFAULT_INTERVAL
		}
		timeout := d.URL.Query().Get("timeout")
		if timeout == "" {
			timeout = DEFAULT_TIMEOUT
		}
		var http, method, tcp string
		switch protocol {
		case PROTOCOL_HTTP, PROTOCOL_HTTPS:
			http = fmt.Sprintf("%s://%s:%d", protocol, hostname, port)
		case PROTOCOL_TCP:
			tcp = fmt.Sprintf("%s:%d", hostname, port)
		}
		check := &Check{
			ID,
			name,
			CHECK_STATUS_PASSING,
			ID,
			http,
			method,
			tcp,
			interval,
			timeout,
		}
		err := d.DoCheck(check)
		if err != nil {
			return err
		}
	}
	return nil
}

/**
 * @Description: Get service address list
 * @Receiver d: Consul structure pointer
 * @Param name: Service name
 * @Return string: Service address list (comma separated)
 * @Return error: Error message
 */
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
	body, err := io.ReadAll(resp.Body)
	var hss []HealthService
	json.Unmarshal(body, &hss)
	ua := make([]string, 0)
	for _, v := range hss {
		ua = append(ua, fmt.Sprintf("%s:%d", v.Service.Address, v.Service.Port))
	}
	return strings.Join(ua, ","), err
}

/**
 * @Description: Register health check
 * @Receiver d: Consul structure pointer
 * @Param check: Health check configuration
 * @Return error: Error message
 */
func (d *Consul) DoCheck(check *Check) error {
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
