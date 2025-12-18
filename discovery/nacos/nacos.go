package nacos

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/sunquakes/jsonrpc4go/discovery"
)

/**
 * @Description: Whether it is an ephemeral instance
 */
const IS_EPHEMERAL = "true"

/**
 * @Description: Heartbeat interval (seconds)
 */
const HEARTBEAT_INTERVAL = 5

/**
 * @Description: Maximum heartbeat retry times
 */
const HEARTBEAT_RETRY_MAX = 3

/**
 * @Description: Read HTTP response body
 * @Param body: HTTP response body
 * @Return []byte: Response body content
 * @Return error: Error message
 */
func ReadAll(body io.ReadCloser) ([]byte, error) {
	return io.ReadAll(body)
}

/**
 * @Description: Nacos client structure, implements discovery.Driver interface
 * @Field URL: Nacos server URL address
 * @Field Token: Authentication token
 * @Field Ephemeral: Whether it is an ephemeral instance
 * @Field HeartbeatList: Heartbeat service list
 * @Field HeartbeatRetry: Heartbeat retry count
 */
type Nacos struct {
	URL            *url.URL
	Token          string
	Ephemeral      string
	HeartbeatList  []Service
	HeartbeatRetry map[string]int
}

/**
 * @Description: Get service list response structure
 * @Field Hosts: Service list
 */
type GetResp struct {
	Hosts []Service `json:"hosts"`
}

/**
 * @Description: Service instance structure
 * @Field Ip: Service IP address
 * @Field Port: Service port number
 * @Field Healthy: Whether healthy
 * @Field InstanceId: Instance ID
 */
type Service struct {
	Ip         string `json:"ip"`
	Port       int    `json:"port"`
	Healthy    bool   `json:"healthy"`
	InstanceId string `json:"instanceId"`
}

/**
 * @Description: Create Nacos client instance
 * @Param rawURL: Nacos server URL address
 * @Return discovery.Driver: Service discovery driver instance
 * @Return error: Error message
 */
func NewNacos(rawURL string) (discovery.Driver, error) {
	URL, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}
	ephemeral := IS_EPHEMERAL
	if URL.Query().Has("ephemeral") {
		ephemeral = URL.Query().Get("ephemeral")

	}
	nacos := &Nacos{URL, URL.Query().Get("token"), ephemeral, make([]Service, 0), make(map[string]int)}
	return nacos, err
}

/**
 * @Description: Register service
 * @Receiver d: Nacos structure pointer
 * @Param name: Service name
 * @Param protocol: Protocol type
 * @Param hostname: Hostname
 * @Param port: Port number
 * @Return error: Error message
 */
func (d *Nacos) Register(name string, protocol string, hostname string, port int) error {
	query := make(map[string]string)
	// Get the instanceId from url
	query["serviceName"] = name
	query["ip"] = hostname
	query["port"] = strconv.Itoa(port)
	queries := d.URL.Query()
	for k, v := range queries {
		if len(v) > 0 {
			query[k] = v[0]
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
		body, err := ReadAll(resp.Body)
		if err != nil {
			return err
		}
		return errors.New(string(body))
	}
	if d.Ephemeral == IS_EPHEMERAL {
		if len(d.HeartbeatList) == 0 {
			d.Heartbeat()
		}
		d.HeartbeatList = append(d.HeartbeatList, Service{hostname, port, true, name})
	}
	return nil
}

/**
 * @Description: Get service address list
 * @Receiver d: Nacos structure pointer
 * @Param name: Service name
 * @Return string: Service address list (comma separated)
 * @Return error: Error message
 */
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
	body, err := ReadAll(resp.Body)
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
		return "", errors.New("unable to get service url")
	}
	return strings.Join(ua, ","), err
}

/**
 * @Description: Send heartbeat
 * @Receiver d: Nacos structure pointer
 * @Param name: Service name
 * @Param hostname: Hostname
 * @Param port: Port number
 * @Return error: Error message
 */
func (d *Nacos) Beat(name string, hostname string, port int) error {
	query := make(map[string]string)
	// Get the instanceId from url
	query["serviceName"] = name
	query["ip"] = hostname
	query["port"] = strconv.Itoa(port)
	queries := d.URL.Query()
	for k, v := range queries {
		if len(v) > 0 {
			query[k] = v[0]
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
		body, err := ReadAll(resp.Body)
		if err != nil {
			return err
		}
		return errors.New(string(body))
	}
	return nil
}

/**
 * @Description: Start heartbeat mechanism
 * @Receiver d: Nacos structure pointer
 * @Return error: Error message
 */
func (d *Nacos) Heartbeat() error {
	go func() {
		ticker := time.NewTicker(time.Second * HEARTBEAT_INTERVAL)
		defer ticker.Stop()
		for range ticker.C {
			d.DoHeartbeat()
		}
	}()
	return nil
}

/**
 * @Description: Execute heartbeat
 * @Receiver d: Nacos structure pointer
 */
func (d *Nacos) DoHeartbeat() {
	for _, service := range d.HeartbeatList {
		err := d.Beat(service.InstanceId, service.Ip, service.Port)
		if err != nil {
			key := fmt.Sprintf("%s-%d", service.Ip, service.Port)
			d.RetryHeartbeat(key)
		}
	}
}

/**
 * @Description: Retry heartbeat
 * @Receiver d: Nacos structure pointer
 * @Param key: Service instance identifier (ip-port)
 */
func (d *Nacos) RetryHeartbeat(key string) {
	if times, ok := d.HeartbeatRetry[key]; ok {
		if times >= HEARTBEAT_RETRY_MAX {
			d.RemoveHeartbeat(key)
		} else {
			d.HeartbeatRetry[key]++
		}
	} else {
		d.HeartbeatRetry[key] = 1
	}
}

/**
 * @Description: Remove heartbeat service
 * @Receiver d: Nacos structure pointer
 * @Param key: Service instance identifier (ip-port)
 */
func (d *Nacos) RemoveHeartbeat(key string) {
	for i, service := range d.HeartbeatList {
		if fmt.Sprintf("%s-%d", service.Ip, service.Port) == key {
			d.HeartbeatList = append(d.HeartbeatList[:i], d.HeartbeatList[i+1:]...)
			break
		}
	}
}
