package client

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/sunquakes/jsonrpc4go/common"
	"github.com/sunquakes/jsonrpc4go/discovery"
)

const (
	HTTP_PROTOCOL  = "http"
	HTTPS_PROTOCOL = "https"
)

/*
 * Http represents the HTTP protocol implementation
 * @property Name - The name of the service
 * @property Protocol - The protocol to use (http or https)
 * @property Address - The address of the service
 * @property Discovery - The service discovery driver
 */
type Http struct {
	Name      string
	Protocol  string
	Address   string
	Discovery discovery.Driver
}

/*
 * HttpClient represents the HTTP client implementation
 * @property Name - The name of the service
 * @property Protocol - The protocol to use (http or https)
 * @property Address - The address of the service
 * @property Discovery - The service discovery driver
 * @property AddressList - The list of addresses for load balancing
 * @property RequestList - The list of requests for batch calls
 * @property Options - The HTTP client options
 */
type HttpClient struct {
	Name        string
	Protocol    string
	Address     string
	Discovery   discovery.Driver
	AddressList []*AddressInfo
	RequestList []*common.SingleRequest
	Options     *HttpOptions
}

/*
 * AddressInfo represents address information for load balancing
 * @property Address - The address of the service
 * @property Load - The load of the service
 */
type AddressInfo struct {
	Address string
	Load    int
}

/*
 * HttpOptions represents the options for the HTTP client
 * @property CaPath - The path to the CA file
 * @property TLSClientConfig - The TLS client configuration
 */
type HttpOptions struct {
	CaPath          string
	TLSClientConfig *tls.Config
}

/*
 * NewClient creates a new HTTP client
 * @return Client - The new HTTP client
 */
func (p *Http) NewClient() Client {
	return NewHttpClient(p.Name, p.Protocol, p.Address, p.Discovery)
}

/*
 * NewHttpClient creates a new HTTP client
 * @param name - The name of the service
 * @param protocol - The protocol to use (http or https)
 * @param address - The address of the service
 * @param dc - The service discovery driver
 * @return *HttpClient - The new HTTP client
 */
func NewHttpClient(name string, protocol string, address string, dc discovery.Driver) *HttpClient {
	c := &HttpClient{
		name,
		protocol,
		address,
		dc,
		nil,
		nil,
		nil,
	}
	c.SetAddressList()
	return c
}

/*
 * SetOptions sets the HTTP client options
 * @param httpOptions - The HTTP client options
 */
func (c *HttpClient) SetOptions(httpOptions any) {
	// Set http request options.
	c.Options = httpOptions.(*HttpOptions)
	if c.Protocol == HTTPS_PROTOCOL && c.Options != nil && c.Options.CaPath != "" {
		file, err := os.Open(c.Options.CaPath)
		if err != nil {
			return
		}
		defer file.Close()
		caCert, err := io.ReadAll(file)
		if err != nil {
			return
		}
		caCertPool := x509.NewCertPool()
		caCertPool.AppendCertsFromPEM(caCert)
		c.Options.TLSClientConfig = &tls.Config{
			RootCAs: caCertPool,
		}
	}
}

/*
 * SetPoolOptions sets the HTTP pool options
 * @param httpOptions - The HTTP pool options
 */
func (c *HttpClient) SetPoolOptions(httpOptions any) {
	// Set http pool options.
}

/*
 * BatchAppend appends a request to the batch
 * @param method - The method to call
 * @param params - The parameters for the method
 * @param result - The result of the method
 * @param isNotify - Whether the request is a notification
 * @return *error - A pointer to the error
 */
func (c *HttpClient) BatchAppend(method string, params any, result any, isNotify bool) *error {
	singleRequest := &common.SingleRequest{
		Method:   method,
		Params:   params,
		Result:   result,
		Error:    new(error),
		IsNotify: isNotify,
	}
	c.RequestList = append(c.RequestList, singleRequest)
	return singleRequest.Error
}

/*
 * BatchCall executes all requests in the batch
 * @return error - An error if the batch call failed
 */
func (c *HttpClient) BatchCall() error {
	var (
		err error
		br  []any
	)
	for _, v := range c.RequestList {
		var (
			req any
		)
		method := fmt.Sprintf("%s/%s", c.Name, v.Method)
		if v.IsNotify {
			req = common.Rs(nil, method, v.Params)
		} else {
			req = common.Rs(strconv.FormatInt(time.Now().Unix(), 10), method, v.Params)
		}
		br = append(br, req)
	}
	bReq := common.JsonBatchRs(br)
	err = c.handleFunc(bReq, c.RequestList)
	c.RequestList = make([]*common.SingleRequest, 0)
	return err
}

/*
 * Call executes a single request
 * @param method - The method to call
 * @param params - The parameters for the method
 * @param result - The result of the method
 * @param isNotify - Whether the request is a notification
 * @return error - An error if the call failed
 */
func (c *HttpClient) Call(method string, params any, result any, isNotify bool) error {
	var (
		err error
		req []byte
	)
	method = fmt.Sprintf("%s/%s", c.Name, method)
	if isNotify {
		req = common.JsonRs(nil, method, params)
	} else {
		req = common.JsonRs(strconv.FormatInt(time.Now().Unix(), 10), method, params)
	}
	err = c.handleFunc(req, result)
	return err
}

/*
 * handleFunc handles the HTTP request
 * @param b - The request body
 * @param result - The result of the request
 * @return error - An error if the request failed
 */
func (c *HttpClient) handleFunc(b []byte, result any) error {
	address, err := c.GetAddress()
	if err != nil {
		return err
	}
	url := fmt.Sprintf("%s://%s", c.Protocol, address)
	transport := &http.Transport{}
	if c.Protocol == HTTPS_PROTOCOL && c.Options != nil && c.Options.TLSClientConfig != nil {
		transport.TLSClientConfig = c.Options.TLSClientConfig
	}
	client := &http.Client{Transport: transport}
	resp, err := client.Post(url, "application/json", bytes.NewReader(b))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	err = common.GetResult(body, result)
	return err
}

/*
 * SetAddressList sets the address list from the discovery service
 */
func (c *HttpClient) SetAddressList() {
	var (
		err error
	)
	address := c.Address
	if c.Discovery != nil {
		address, err = c.Discovery.Get(c.Name)
		if err != nil {
			common.Debug(err.Error())
		}
	}
	addresses := strings.Split(address, ",")
	addressList := make([]*AddressInfo, 0)
	for _, v := range addresses {
		addressList = append(addressList, &AddressInfo{
			Address: v,
			Load:    0,
		})
	}
	c.AddressList = addressList
}

/*
 * GetAddress gets an address from the address list using load balancing
 * @return string - The address to use
 * @return error - An error if no address is available
 */
func (c *HttpClient) GetAddress() (string, error) {
	size := len(c.AddressList)
	if size == 0 {
		c.SetAddressList()
	}
	size = len(c.AddressList)
	if size == 0 {
		return "", errors.New("fail to get service url")
	}
	if size == 1 {
		return c.AddressList[0].Address, nil
	}
	// Randomly select two nodes
	randSource := rand.New(rand.NewSource(time.Now().UnixNano()))
	index1 := randSource.Intn(size)
	index2 := randSource.Intn(size)
	// Make sure the two nodes are different
	for index1 == index2 {
		index2 = rand.Intn(size)
	}
	if c.AddressList[index1].Load < c.AddressList[index2].Load {
		c.AddressList[index1].Load++
		return c.AddressList[index1].Address, nil
	}
	c.AddressList[index2].Load++
	return c.AddressList[index2].Address, nil
}
