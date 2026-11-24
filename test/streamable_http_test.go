package test

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"iter"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/sunquakes/jsonrpc4go"
	"github.com/sunquakes/jsonrpc4go/client"
	"github.com/sunquakes/jsonrpc4go/common"
	"github.com/sunquakes/jsonrpc4go/server"
)

// StreamRpc exercises the streaming result shapes: slice, channel and iterator.
type StreamRpc struct{}

func (s *StreamRpc) Count(n int) []int {
	out := make([]int, n)
	for i := range out {
		out[i] = i + 1
	}
	return out
}

func (s *StreamRpc) Chan(n int) chan int {
	ch := make(chan int)
	go func() {
		defer close(ch)
		for i := 1; i <= n; i++ {
			ch <- i
		}
	}()
	return ch
}

func (s *StreamRpc) Seq(n int) iter.Seq[int] {
	return func(yield func(int) bool) {
		for i := 1; i <= n; i++ {
			if !yield(i * 10) {
				return
			}
		}
	}
}

func (s *StreamRpc) Fail(n int) (int, error) {
	if n < 0 {
		return 0, errors.New("negative")
	}
	return n, nil
}

// shEvent is one parsed server-sent event.
type shEvent struct {
	id   string
	data map[string]any
}

// shStart launches a streamable HTTP server on a random port.
func shStart(t *testing.T, opts *server.StreamableHttpOptions, services ...any) string {
	t.Helper()
	s, err := jsonrpc4go.NewServer("streamable-http", 0)
	if err != nil {
		t.Fatal(err)
	}
	if opts != nil {
		s.SetOptions(*opts)
	}
	for _, v := range services {
		s.Register(v)
	}
	go s.Start()
	<-s.GetEvent()
	return serverAddr(s)
}

// shPost sends one raw JSON payload to the /mcp endpoint.
func shPost(t *testing.T, addr string, body any, headers map[string]string) (*http.Response, []byte) {
	t.Helper()
	var data []byte
	switch b := body.(type) {
	case string:
		data = []byte(b)
	default:
		var err error
		data, err = json.Marshal(b)
		if err != nil {
			t.Fatal(err)
		}
	}
	req, err := http.NewRequest(http.MethodPost, "http://"+addr+server.McpEndpoint, bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	return resp, b
}

// shInitialize performs the MCP handshake and returns the session id.
func shInitialize(t *testing.T, addr string, protocolVersion string) (string, http.Header, map[string]any) {
	t.Helper()
	body := map[string]any{
		"jsonrpc": "2.0",
		"id":      "1",
		"method":  "initialize",
		"params": map[string]any{
			"protocolVersion": protocolVersion,
			"capabilities":    map[string]any{},
			"clientInfo":      map[string]any{"name": "test-client", "version": "0.0.1"},
		},
	}
	resp, b := shPost(t, addr, body, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("initialize status %d, body %s", resp.StatusCode, b)
	}
	sid := resp.Header.Get(server.McpSessionHeader)
	if sid == "" {
		t.Fatal("initialize did not return a session id")
	}
	result := map[string]any{}
	if err := json.Unmarshal(b, &result); err != nil {
		t.Fatal(err)
	}
	res, _ := result["result"].(map[string]any)
	if res == nil {
		t.Fatalf("initialize result missing: %s", b)
	}
	return sid, resp.Header, res
}

func shSession(sid string) map[string]string {
	return map[string]string{server.McpSessionHeader: sid}
}

// shRequestJSON sends one native JSON-RPC request and decodes the response.
func shRequestJSON(t *testing.T, addr string, sid string, req map[string]any) map[string]any {
	t.Helper()
	resp, b := shPost(t, addr, req, shSession(sid))
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("request status %d, body %s", resp.StatusCode, b)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Fatalf("expected JSON content type, got %q", ct)
	}
	out := map[string]any{}
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}
	return out
}

// shParseSSE decodes a fully buffered SSE body into events.
func shParseSSE(t *testing.T, body []byte) []shEvent {
	t.Helper()
	var events []shEvent
	for _, chunk := range strings.Split(string(body), "\n\n") {
		if strings.TrimSpace(chunk) == "" {
			continue
		}
		ev := shEvent{}
		for _, line := range strings.Split(chunk, "\n") {
			switch {
			case strings.HasPrefix(line, "id:"):
				ev.id = strings.TrimSpace(strings.TrimPrefix(line, "id:"))
			case strings.HasPrefix(line, "data:"):
				payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
				if err := json.Unmarshal([]byte(payload), &ev.data); err != nil {
					t.Fatalf("invalid SSE data %q: %v", payload, err)
				}
			}
		}
		if ev.data != nil {
			events = append(events, ev)
		}
	}
	return events
}

// shGet opens the GET SSE channel; the caller must cancel and close the body.
func shGet(t *testing.T, addr string, headers map[string]string) (context.CancelFunc, *http.Response) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+addr+server.McpEndpoint, nil)
	if err != nil {
		t.Fatal(err)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return cancel, resp
}

// shReadEvents reads exactly n events from a live SSE body, failing on timeout.
func shReadEvents(t *testing.T, body io.Reader, n int) []shEvent {
	t.Helper()
	type res struct{ events []shEvent }
	ch := make(chan res, 1)
	go func() {
		scanner := bufio.NewScanner(body)
		var events []shEvent
		var cur shEvent
		hasData := false
		for scanner.Scan() {
			line := scanner.Text()
			switch {
			case line == "":
				if !hasData {
					continue
				}
				events = append(events, cur)
				cur = shEvent{}
				hasData = false
				if len(events) == n {
					ch <- res{events}
					return
				}
			case strings.HasPrefix(line, "id:"):
				cur.id = strings.TrimSpace(strings.TrimPrefix(line, "id:"))
			case strings.HasPrefix(line, "data:"):
				payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
				if err := json.Unmarshal([]byte(payload), &cur.data); err != nil {
					ch <- res{nil}
					return
				}
				hasData = true
			}
		}
		ch <- res{events}
	}()
	select {
	case r := <-ch:
		if len(r.events) != n {
			t.Fatalf("expected %d SSE events, got %d", n, len(r.events))
		}
		return r.events
	case <-time.After(5 * time.Second):
		t.Fatalf("timeout waiting for %d SSE events", n)
		return nil
	}
}

// shStreamValues extracts the values carried by notifications/stream events.
func shStreamValues(t *testing.T, events []shEvent) []float64 {
	t.Helper()
	var out []float64
	for _, ev := range events {
		if ev.data["method"] != "notifications/stream" {
			continue
		}
		params, _ := ev.data["params"].(map[string]any)
		if params == nil {
			continue
		}
		v, _ := params["value"].(float64)
		out = append(out, v)
	}
	return out
}

func TestStreamableHandshake(t *testing.T) {
	addr := shStart(t, nil, new(IntRpc))
	sid, headers, result := shInitialize(t, addr, "2025-03-26")
	if sid == "" {
		t.Fatal("empty session id")
	}
	if got := headers.Get(server.McpProtocolHeader); got != "2025-03-26" {
		t.Errorf("negotiated version %q, want 2025-03-26", got)
	}
	if result["protocolVersion"] != "2025-03-26" {
		t.Errorf("result protocolVersion %v", result["protocolVersion"])
	}
	if info, _ := result["serverInfo"].(map[string]any); info["name"] != "jsonrpc4go" {
		t.Errorf("serverInfo %v", result["serverInfo"])
	}
	if caps, _ := result["capabilities"].(map[string]any); caps["tools"] == nil {
		t.Errorf("capabilities missing tools: %v", result["capabilities"])
	}
	// An unsupported version falls back to the latest supported one.
	_, headers, result = shInitialize(t, addr, "1999-01-01")
	if headers.Get(server.McpProtocolHeader) != "2025-06-18" {
		t.Errorf("fallback version %q", headers.Get(server.McpProtocolHeader))
	}
	if result["protocolVersion"] != "2025-06-18" {
		t.Errorf("fallback result version %v", result["protocolVersion"])
	}
}

func TestStreamableUnaryJSON(t *testing.T) {
	addr := shStart(t, nil, new(IntRpc))
	sid, _, _ := shInitialize(t, addr, "2025-06-18")
	out := shRequestJSON(t, addr, sid, map[string]any{
		"jsonrpc": "2.0", "id": "2", "method": "IntRpc/Add", "params": []any{1, 2},
	})
	if out["result"] != float64(3) {
		t.Errorf("Add 1+2 expected 3, got %v", out["result"])
	}
	// Unknown method keeps JSON-RPC error semantics.
	out = shRequestJSON(t, addr, sid, map[string]any{
		"jsonrpc": "2.0", "id": "3", "method": "IntRpc/Missing", "params": []any{},
	})
	errObj, _ := out["error"].(map[string]any)
	if errObj == nil || errObj["code"] != float64(common.MethodNotFound) {
		t.Errorf("expected MethodNotFound error, got %v", out)
	}
}

func TestStreamableUnknownSession(t *testing.T) {
	addr := shStart(t, nil, new(IntRpc))
	resp, b := shPost(t, addr, map[string]any{
		"jsonrpc": "2.0", "id": "1", "method": "IntRpc/Add", "params": []any{1, 2},
	}, map[string]string{server.McpSessionHeader: "bogus"})
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status %d, want 404, body %s", resp.StatusCode, b)
	}
}

func TestStreamableStateless(t *testing.T) {
	addr := shStart(t, nil, new(IntRpc))
	// Requests without a session header are served statelessly.
	out := shRequestJSON(t, addr, "", map[string]any{
		"jsonrpc": "2.0", "id": "1", "method": "IntRpc/Add", "params": []any{4, 5},
	})
	if out["result"] != float64(9) {
		t.Errorf("Add 4+5 expected 9, got %v", out["result"])
	}
}

func TestStreamableSliceSSE(t *testing.T) {
	addr := shStart(t, nil, new(StreamRpc))
	sid, _, _ := shInitialize(t, addr, "2025-06-18")
	resp, b := shPost(t, addr, map[string]any{
		"jsonrpc": "2.0", "id": "7", "method": "StreamRpc/Count", "params": []any{3},
	}, shSession(sid))
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/event-stream") {
		t.Fatalf("expected SSE content type, got %q", ct)
	}
	events := shParseSSE(t, b)
	if len(events) != 4 {
		t.Fatalf("expected 4 events, got %d: %s", len(events), b)
	}
	values := shStreamValues(t, events)
	if len(values) != 3 || values[0] != 1 || values[2] != 3 {
		t.Errorf("stream values %v", values)
	}
	final := events[3].data
	if final["method"] != nil {
		t.Errorf("final event should be the response, got %v", final)
	}
	if final["id"] != "7" {
		t.Errorf("final event id %v", final["id"])
	}
	// Events are numbered for replay.
	if events[0].id == "" || events[3].id == "" {
		t.Errorf("events missing ids: %s", b)
	}
}

func TestStreamableChanAndSeqSSE(t *testing.T) {
	addr := shStart(t, nil, new(StreamRpc))
	sid, _, _ := shInitialize(t, addr, "2025-06-18")
	for _, tc := range []struct {
		method string
		want   []float64
	}{
		{"Chan", []float64{1, 2}},
		{"Seq", []float64{10, 20}},
	} {
		resp, b := shPost(t, addr, map[string]any{
			"jsonrpc": "2.0", "id": "x", "method": "StreamRpc/" + tc.method, "params": []any{2},
		}, shSession(sid))
		if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/event-stream") {
			t.Fatalf("%s expected SSE, got %q", tc.method, ct)
		}
		values := shStreamValues(t, shParseSSE(t, b))
		if len(values) != 2 || values[0] != tc.want[0] || values[1] != tc.want[1] {
			t.Errorf("%s values %v, want %v", tc.method, values, tc.want)
		}
	}
}

func TestStreamableNotification(t *testing.T) {
	addr := shStart(t, nil, new(IntRpc))
	sid, _, _ := shInitialize(t, addr, "2025-06-18")
	// Native notification (no id) must be accepted with 202.
	resp, b := shPost(t, addr, map[string]any{
		"jsonrpc": "2.0", "method": "IntRpc/Add", "params": []any{1, 2},
	}, shSession(sid))
	if resp.StatusCode != http.StatusAccepted {
		t.Errorf("notification status %d, body %s", resp.StatusCode, b)
	}
	// MCP notifications are accepted too.
	resp, b = shPost(t, addr, map[string]any{
		"jsonrpc": "2.0", "method": "notifications/initialized",
	}, shSession(sid))
	if resp.StatusCode != http.StatusAccepted {
		t.Errorf("mcp notification status %d, body %s", resp.StatusCode, b)
	}
}

func TestStreamableOrigin(t *testing.T) {
	addr := shStart(t, &server.StreamableHttpOptions{AllowedOrigins: []string{"http://good.example"}}, new(IntRpc))
	body := map[string]any{"jsonrpc": "2.0", "id": "1", "method": "IntRpc/Add", "params": []any{1, 2}}
	resp, _ := shPost(t, addr, body, map[string]string{"Origin": "http://evil.example"})
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("evil origin status %d, want 403", resp.StatusCode)
	}
	resp, _ = shPost(t, addr, body, map[string]string{"Origin": "http://good.example"})
	if resp.StatusCode != http.StatusOK {
		t.Errorf("allowed origin status %d, want 200", resp.StatusCode)
	}
	resp, _ = shPost(t, addr, body, nil)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("missing origin status %d, want 200", resp.StatusCode)
	}
}

func TestStreamableLastEventIDReplay(t *testing.T) {
	addr := shStart(t, nil, new(StreamRpc))
	sid, _, _ := shInitialize(t, addr, "2025-06-18")
	shPost(t, addr, map[string]any{
		"jsonrpc": "2.0", "id": "9", "method": "StreamRpc/Count", "params": []any{3},
	}, shSession(sid))
	// Replay everything after event 2: the third value plus the final response.
	cancel, resp := shGet(t, addr, map[string]string{
		server.McpSessionHeader:   sid,
		server.McpLastEventHeader: "2",
	})
	defer cancel()
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET status %d", resp.StatusCode)
	}
	events := shReadEvents(t, resp.Body, 2)
	if events[0].id != "3" || events[1].id != "4" {
		t.Errorf("replayed ids %s %s, want 3 4", events[0].id, events[1].id)
	}
	if events[0].data["method"] != "notifications/stream" {
		t.Errorf("replayed first event %v", events[0].data)
	}
}

func TestStreamableGetChannel(t *testing.T) {
	addr := shStart(t, nil, new(StreamRpc))
	sid, _, _ := shInitialize(t, addr, "2025-06-18")
	cancel, resp := shGet(t, addr, shSession(sid))
	defer cancel()
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET status %d", resp.StatusCode)
	}
	// Give the GET handler a moment to subscribe before publishing.
	time.Sleep(100 * time.Millisecond)
	shPost(t, addr, map[string]any{
		"jsonrpc": "2.0", "id": "10", "method": "StreamRpc/Count", "params": []any{2},
	}, shSession(sid))
	events := shReadEvents(t, resp.Body, 3)
	for _, ev := range events {
		if ev.id == "" {
			t.Errorf("GET channel event missing id: %v", ev.data)
		}
	}
	if events[2].data["method"] != nil && events[2].data["id"] != "10" {
		t.Errorf("unexpected final event %v", events[2].data)
	}
}

func TestStreamableDeleteSession(t *testing.T) {
	addr := shStart(t, nil, new(IntRpc))
	sid, _, _ := shInitialize(t, addr, "2025-06-18")
	req, err := http.NewRequest(http.MethodDelete, "http://"+addr+server.McpEndpoint, nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set(server.McpSessionHeader, sid)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("delete status %d, want 204", resp.StatusCode)
	}
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("second delete status %d, want 404", resp.StatusCode)
	}
	// Requests on the deleted session are rejected.
	shPostResp := func() int {
		r, _ := shPost(t, addr, map[string]any{
			"jsonrpc": "2.0", "id": "1", "method": "IntRpc/Add", "params": []any{1, 2},
		}, shSession(sid))
		return r.StatusCode
	}
	if shPostResp() != http.StatusNotFound {
		t.Errorf("request after delete should be 404")
	}
}

func TestStreamableBatch(t *testing.T) {
	addr := shStart(t, nil, new(IntRpc))
	sid, _, _ := shInitialize(t, addr, "2025-06-18")
	out := shRequestJSON(t, addr, sid, map[string]any{"jsonrpc": "2.0", "id": "1", "method": "ping"})
	_ = out
	resp, b := shPost(t, addr, []any{
		map[string]any{"jsonrpc": "2.0", "id": "1", "method": "IntRpc/Add", "params": []any{2, 3}},
		map[string]any{"jsonrpc": "2.0", "id": "2", "method": "IntRpc/Missing", "params": []any{}},
	}, shSession(sid))
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("batch status %d, body %s", resp.StatusCode, b)
	}
	items, err := common.GetBatchResult(b)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 || items[0].Err != nil || items[0].Result != float64(5) {
		t.Errorf("batch item 0: %+v", items[0])
	}
	if items[1].Err == nil {
		t.Errorf("batch item 1 expected error")
	}
}

func TestStreamableBadRequests(t *testing.T) {
	addr := shStart(t, nil, new(IntRpc))
	// Malformed JSON body.
	resp, b := shPost(t, addr, "{not json", nil)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("parse error status %d, body %s", resp.StatusCode, b)
	}
	if !bytes.Contains(b, []byte("Parse error")) {
		t.Errorf("parse error body %s", b)
	}
	// Unsupported method.
	req, err := http.NewRequest(http.MethodPut, "http://"+addr+server.McpEndpoint, nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("PUT status %d, want 405", resp.StatusCode)
	}
}

func TestStreamableClientUnary(t *testing.T) {
	addr := shStart(t, nil, new(IntRpc), new(StreamRpc))
	c, err := jsonrpc4go.NewClient("IntRpc", "streamable-http", addr)
	if err != nil {
		t.Fatal(err)
	}
	result, err := jsonrpc4go.Call[int](c, "Add", 1, 6)
	if err != nil {
		t.Fatal(err)
	}
	if result != 7 {
		t.Errorf("Add 1+6 expected 7, got %d", result)
	}
}

func TestStreamableClientStreamingCall(t *testing.T) {
	addr := shStart(t, nil, new(StreamRpc))
	c, err := jsonrpc4go.NewClient("StreamRpc", "streamable-http", addr)
	if err != nil {
		t.Fatal(err)
	}
	res, err := c.Call("Count", 3)
	if err != nil {
		t.Fatal(err)
	}
	values, ok := res.([]any)
	if !ok || len(values) != 3 {
		t.Fatalf("Call streaming expected 3 values, got %#v", res)
	}
	if values[0] != float64(1) || values[2] != float64(3) {
		t.Errorf("stream values %v", values)
	}
	// Typed generic helper decodes the collected stream.
	typed, err := jsonrpc4go.Call[[]int](c, "Count", 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(typed) != 2 || typed[0] != 1 || typed[1] != 2 {
		t.Errorf("typed stream %v", typed)
	}
}

func TestStreamableClientStream(t *testing.T) {
	addr := shStart(t, nil, new(StreamRpc))
	c, err := jsonrpc4go.NewClient("StreamRpc", "streamable-http", addr)
	if err != nil {
		t.Fatal(err)
	}
	var got []int
	for v, err := range jsonrpc4go.Stream[int](c, "Seq", 4) {
		if err != nil {
			t.Fatal(err)
		}
		got = append(got, v)
	}
	if len(got) != 4 || got[3] != 40 {
		t.Errorf("Stream values %v", got)
	}
	// Unary methods yield a single value.
	var unary []any
	streamer := c.(client.Streamer)
	for v, err := range streamer.Stream("Fail", 5) {
		if err != nil {
			t.Fatal(err)
		}
		unary = append(unary, v)
	}
	if len(unary) != 1 || unary[0] != float64(5) {
		t.Errorf("unary stream %v", unary)
	}
	// Breaking out of the loop stops the stream.
	count := 0
	for range streamer.Stream("Count", 5) {
		count++
		if count == 2 {
			break
		}
	}
	if count != 2 {
		t.Errorf("early break count %d", count)
	}
}

func TestStreamableClientNotifyBatch(t *testing.T) {
	addr := shStart(t, nil, new(IntRpc))
	c, err := jsonrpc4go.NewClient("IntRpc", "streamable-http", addr)
	if err != nil {
		t.Fatal(err)
	}
	if err := c.Notify("Add", 2, 3); err != nil {
		t.Errorf("notify failed: %v", err)
	}
	c.BatchAppend("Missing", 1)
	c.BatchAppend("Add", 2, 3)
	results, err := c.BatchCall()
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 || results[0].Err == nil {
		t.Errorf("batch results %+v", results)
	}
	if results[1].Err != nil || results[1].Result != float64(5) {
		t.Errorf("batch item 1: %+v", results[1])
	}
}

func TestStreamableClientError(t *testing.T) {
	addr := shStart(t, nil, new(StreamRpc))
	c, err := jsonrpc4go.NewClient("StreamRpc", "streamable-http", addr)
	if err != nil {
		t.Fatal(err)
	}
	_, err = c.Call("Fail", -1)
	if err == nil {
		t.Fatal("expected error from Fail(-1)")
	}
	if err.Error() != common.CodeMap[common.InternalError] {
		t.Errorf("error %q", err.Error())
	}
}

// TestStreamableLegacyHttpRegression checks that the old HTTP transport still
// serves Service/method dispatch and collects stream results into arrays.
func TestStreamableLegacyHttpRegression(t *testing.T) {
	s, err := jsonrpc4go.NewServer("http", 0)
	if err != nil {
		t.Fatal(err)
	}
	s.Register(new(IntRpc))
	s.Register(new(StreamRpc))
	go s.Start()
	<-s.GetEvent()
	c, err := jsonrpc4go.NewClient("StreamRpc", "http", serverAddr(s))
	if err != nil {
		t.Fatal(err)
	}
	res, err := c.Call("Count", 3)
	if err != nil {
		t.Fatal(err)
	}
	values, ok := res.([]any)
	if !ok || len(values) != 3 || values[2] != float64(3) {
		t.Errorf("http collected stream %#v", res)
	}
	c2, _ := jsonrpc4go.NewClient("IntRpc", "http", serverAddr(s))
	res, err = c2.Call("Add", 1, 2)
	if err != nil || res != float64(3) {
		t.Errorf("http Add %v %v", res, err)
	}
}
