package client

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"iter"
	"math/rand/v2"
	"net/http"
	"strings"
	"sync"

	"github.com/sunquakes/jsonrpc4go/common"
	"github.com/sunquakes/jsonrpc4go/discovery"
)

const (
	STREAMABLE_HTTP_PROTOCOL = "streamable-http"
	mcpEndpointPath          = "/mcp"
	mcpSessionHeaderName     = "Mcp-Session-Id"
	mcpProtocolHeaderName    = "Mcp-Protocol-Version"
	mcpLatestProtocolVersion = "2025-06-18"
)

// errStopSSE aborts SSE reading once the final response is seen.
var errStopSSE = errors.New("streamable-http: end of stream")

/*
 * StreamableHttp represents the streamable HTTP protocol implementation.
 * @property Name - The name of the service
 * @property Address - The address of the service
 * @property Discovery - The service discovery driver
 */
type StreamableHttp struct {
	Name      string
	Address   string
	Discovery discovery.Driver
}

/*
 * StreamableHttpClient implements the MCP streamable HTTP client: unary
 * results arrive as JSON, streaming results as SSE, and both are handled by
 * Call and Stream.
 * @property Name - The name of the service
 * @property Address - The address of the service
 * @property Discovery - The service discovery driver
 * @property RequestList - The list of requests for batch calls
 */
type StreamableHttpClient struct {
	Name        string
	Address     string
	Discovery   discovery.Driver
	RequestList []*common.SingleRequest

	lock            sync.Mutex
	client          *http.Client
	addresses       []string
	sessionID       string
	protocolVersion string
}

/*
 * NewClient creates a new streamable HTTP client
 * @return Client - The new streamable HTTP client
 */
func (p *StreamableHttp) NewClient() Client {
	return NewStreamableHttpClient(p.Name, p.Address, p.Discovery)
}

/*
 * NewStreamableHttpClient creates a new streamable HTTP client
 * @param name - The name of the service
 * @param address - The address of the service
 * @param dc - The service discovery driver
 * @return *StreamableHttpClient - The new streamable HTTP client
 */
func NewStreamableHttpClient(name string, address string, dc discovery.Driver) *StreamableHttpClient {
	c := &StreamableHttpClient{
		Name:            name,
		Address:         address,
		Discovery:       dc,
		protocolVersion: mcpLatestProtocolVersion,
	}
	c.refreshAddresses()
	return c
}

/*
 * SetOptions configures the underlying HTTP client options.
 * @param options - A *http.Client to use, otherwise ignored
 */
func (c *StreamableHttpClient) SetOptions(options any) {
	c.lock.Lock()
	defer c.lock.Unlock()
	if hc, ok := options.(*http.Client); ok {
		c.client = hc
	}
}

/*
 * SetPoolOptions is a no-op for the streamable HTTP client.
 */
func (c *StreamableHttpClient) SetPoolOptions(any) {}

/*
 * refreshAddresses resolves the address list from discovery or the static
 * comma separated address.
 */
func (c *StreamableHttpClient) refreshAddresses() {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.refreshAddressesLocked()
}

func (c *StreamableHttpClient) refreshAddressesLocked() {
	address := c.Address
	if c.Discovery != nil {
		resolved, err := c.Discovery.Get(c.Name)
		if err != nil {
			common.Debug(err.Error())
		} else {
			address = resolved
		}
	}
	list := make([]string, 0, 4)
	for _, v := range strings.Split(address, ",") {
		if v = strings.TrimSpace(v); v != "" {
			list = append(list, v)
		}
	}
	c.addresses = list
}

// urlLocked builds the /mcp endpoint URL for the next request. c.lock must
// be held by the caller.
func (c *StreamableHttpClient) urlLocked() (string, error) {
	if len(c.addresses) == 0 {
		c.refreshAddressesLocked()
	}
	if len(c.addresses) == 0 {
		return "", errors.New("fail to get service url")
	}
	address := c.addresses[0]
	if len(c.addresses) > 1 {
		address = c.addresses[rand.IntN(len(c.addresses))]
	}
	return fmt.Sprintf("http://%s%s", address, mcpEndpointPath), nil
}

/*
 * httpClient returns a shared HTTP client so connections can be reused.
 */
func (c *StreamableHttpClient) httpClient() *http.Client {
	c.lock.Lock()
	defer c.lock.Unlock()
	return c.httpClientLocked()
}

// httpClientLocked returns the shared HTTP client. c.lock must be held by
// the caller.
func (c *StreamableHttpClient) httpClientLocked() *http.Client {
	if c.client == nil {
		transport := http.DefaultTransport.(*http.Transport).Clone()
		c.client = &http.Client{Transport: transport}
	}
	return c.client
}

/*
 * ensureSession performs the MCP initialize handshake once per client.
 */
func (c *StreamableHttpClient) ensureSession() error {
	c.lock.Lock()
	defer c.lock.Unlock()
	if c.sessionID != "" {
		return nil
	}
	url, err := c.urlLocked()
	if err != nil {
		return err
	}
	initReq := map[string]any{
		"jsonrpc": common.JsonRpc,
		"id":      nextRequestID(),
		"method":  "initialize",
		"params": map[string]any{
			"protocolVersion": c.protocolVersion,
			"capabilities":    map[string]any{},
			"clientInfo":      map[string]any{"name": "jsonrpc4go", "version": "1.0.0"},
		},
	}
	data, err := json.Marshal(initReq)
	if err != nil {
		return err
	}
	resp, err := c.httpClientLocked().Post(url, "application/json", bytes.NewReader(data))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	sid := resp.Header.Get(mcpSessionHeaderName)
	if sid == "" {
		return errors.New("streamable-http: server did not create a session")
	}
	if pv := resp.Header.Get(mcpProtocolHeaderName); pv != "" {
		c.protocolVersion = pv
	}
	c.sessionID = sid
	return nil
}

/*
 * post sends one JSON-RPC payload to /mcp with the session headers.
 */
func (c *StreamableHttpClient) post(body []byte) (*http.Response, error) {
	if err := c.ensureSession(); err != nil {
		return nil, err
	}
	c.lock.Lock()
	url, err := c.urlLocked()
	sid, pv := c.sessionID, c.protocolVersion
	c.lock.Unlock()
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set(mcpSessionHeaderName, sid)
	req.Header.Set(mcpProtocolHeaderName, pv)
	return c.httpClient().Do(req)
}

// isSSE reports whether the response carries an SSE body.
func isSSE(resp *http.Response) bool {
	return strings.HasPrefix(resp.Header.Get("Content-Type"), "text/event-stream")
}

/*
 * sseMessage is one decoded server-sent event.
 */
type sseMessage struct {
	ID    string
	Event string
	Data  []byte
}

/*
 * readSSE parses an event stream and dispatches complete events to onMessage.
 * Returning errStopSSE from the callback stops the scan without an error.
 */
func readSSE(r io.Reader, onMessage func(sseMessage) error) error {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	var (
		msg     sseMessage
		dataBuf bytes.Buffer
		hasData bool
	)
	dispatch := func() error {
		if !hasData {
			return nil
		}
		m := msg
		m.Data = append([]byte(nil), dataBuf.Bytes()...)
		dataBuf.Reset()
		hasData = false
		if len(m.Data) == 0 {
			// Priming/keep-alive events carry no payload; skip them.
			msg = sseMessage{}
			return nil
		}
		return onMessage(m)
	}
	for scanner.Scan() {
		line := scanner.Text()
		switch {
		case line == "":
			if err := dispatch(); err != nil {
				if err == errStopSSE {
					return nil
				}
				return err
			}
			msg = sseMessage{}
		case strings.HasPrefix(line, "id:"):
			msg.ID = strings.TrimSpace(strings.TrimPrefix(line, "id:"))
		case strings.HasPrefix(line, "event:"):
			msg.Event = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
		case strings.HasPrefix(line, "data:"):
			if hasData {
				dataBuf.WriteByte('\n')
			}
			dataBuf.WriteString(strings.TrimPrefix(strings.TrimPrefix(line, "data:"), " "))
			hasData = true
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	if err := dispatch(); err != nil && err != errStopSSE {
		return err
	}
	return nil
}

// decodeSSEMessage parses the JSON payload of one SSE message.
func decodeSSEMessage(data []byte) (map[string]any, error) {
	m := map[string]any{}
	err := json.Unmarshal(data, &m)
	return m, err
}

// streamValue extracts the value of a notifications/stream message sent for
// the given request id.
func streamValue(m map[string]any, id string) (any, bool) {
	method, _ := m["method"].(string)
	if method != "notifications/stream" {
		return nil, false
	}
	params, _ := m["params"].(map[string]any)
	if params == nil || params["id"] != id {
		return nil, false
	}
	return params["value"], true
}

// responseError converts a JSON-RPC response message for the given id into
// (result, error, matched).
func responseError(m map[string]any, id string) (any, error, bool) {
	if m["id"] != id {
		return nil, nil, false
	}
	if e, ok := m["error"].(map[string]any); ok {
		message, _ := e["message"].(string)
		return nil, errors.New(message), true
	}
	return m["result"], nil, true
}

/*
 * Call executes a single request. Unary results arrive as JSON; streaming
 * results arrive as SSE and are returned as a slice of values.
 * @param method - The method to call
 * @param args - The positional arguments for the method
 * @return any - The decoded result value
 * @return error - An error if the call failed
 */
func (c *StreamableHttpClient) Call(method string, args ...any) (any, error) {
	fullMethod := fmt.Sprintf("%s/%s", c.Name, method)
	id := nextRequestID()
	req := common.JsonRs(id, fullMethod, normalizeArgs(args))
	resp, err := c.post(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if !isSSE(resp) {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}
		return common.GetResult(body)
	}
	var (
		values   []any
		finalErr error
	)
	scanErr := readSSE(resp.Body, func(msg sseMessage) error {
		m, err := decodeSSEMessage(msg.Data)
		if err != nil {
			return nil
		}
		if v, ok := streamValue(m, id); ok {
			values = append(values, v)
			return nil
		}
		result, err, matched := responseError(m, id)
		if !matched {
			return nil
		}
		if err != nil {
			finalErr = err
		} else if result != nil {
			values = append(values, result)
		}
		return errStopSSE
	})
	if scanErr != nil {
		return nil, scanErr
	}
	if finalErr != nil {
		return nil, finalErr
	}
	return values, nil
}

/*
 * Stream executes a request and yields streaming values as they arrive.
 * Unary responses yield their single result.
 * @param method - The method to call
 * @param args - The positional arguments for the method
 * @return iter.Seq2[any, error] - The value iterator
 */
func (c *StreamableHttpClient) Stream(method string, args ...any) iter.Seq2[any, error] {
	return func(yield func(any, error) bool) {
		fullMethod := fmt.Sprintf("%s/%s", c.Name, method)
		id := nextRequestID()
		req := common.JsonRs(id, fullMethod, normalizeArgs(args))
		resp, err := c.post(req)
		if err != nil {
			yield(nil, err)
			return
		}
		defer resp.Body.Close()
		if !isSSE(resp) {
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				yield(nil, err)
				return
			}
			result, err := common.GetResult(body)
			if err != nil {
				yield(nil, err)
				return
			}
			if result != nil {
				yield(result, nil)
			}
			return
		}
		scanErr := readSSE(resp.Body, func(msg sseMessage) error {
			m, err := decodeSSEMessage(msg.Data)
			if err != nil {
				return nil
			}
			if v, ok := streamValue(m, id); ok {
				if !yield(v, nil) {
					return errStopSSE
				}
				return nil
			}
			result, err, matched := responseError(m, id)
			if !matched {
				return nil
			}
			if err != nil {
				yield(nil, err)
			} else if result != nil {
				yield(result, nil)
			}
			return errStopSSE
		})
		if scanErr != nil {
			yield(nil, scanErr)
		}
	}
}

/*
 * Notify executes a notification request, expecting a 202 response.
 * @param method - The method to call
 * @param args - The positional arguments for the method
 * @return error - An error if the notification failed
 */
func (c *StreamableHttpClient) Notify(method string, args ...any) error {
	fullMethod := fmt.Sprintf("%s/%s", c.Name, method)
	req := common.JsonRs(nil, fullMethod, normalizeArgs(args))
	resp, err := c.post(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusAccepted {
		return nil
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if _, err := common.GetResult(body); err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code %d", resp.StatusCode)
	}
	return nil
}

/*
 * BatchAppend appends a request to the batch
 * @param method - The method to call
 * @param args - The positional arguments for the method
 */
func (c *StreamableHttpClient) BatchAppend(method string, args ...any) {
	c.RequestList = append(c.RequestList, &common.SingleRequest{Method: method, Args: args})
}

/*
 * BatchCall executes all requests in the batch as one JSON array request.
 * @return []BatchResult - Per-request results in the order they were appended
 * @return error - An error if the batch call failed
 */
func (c *StreamableHttpClient) BatchCall() ([]BatchResult, error) {
	if len(c.RequestList) == 0 {
		return nil, nil
	}
	var br []any
	for _, v := range c.RequestList {
		method := fmt.Sprintf("%s/%s", c.Name, v.Method)
		br = append(br, common.Rs(nextRequestID(), method, normalizeArgs(v.Args)))
	}
	bReq := common.JsonBatchRs(br)
	resp, err := c.post(bReq)
	c.RequestList = make([]*common.SingleRequest, 0)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	items, err := common.GetBatchResult(body)
	if err != nil {
		return nil, err
	}
	results := make([]BatchResult, len(items))
	for k, v := range items {
		results[k] = BatchResult{Result: v.Result, Err: v.Err}
	}
	return results, nil
}

var (
	_ Client   = (*StreamableHttpClient)(nil)
	_ Streamer = (*StreamableHttpClient)(nil)
)
