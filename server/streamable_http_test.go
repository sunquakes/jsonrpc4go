package server

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/sunquakes/jsonrpc4go/common"
)

// unitPerson exercises struct parameter schema expansion.
type unitPerson struct {
	Name   string `json:"name"`
	Age    int    `json:"age,omitempty"`
	Skip   string `json:"-"`
	secret string
}

// UnitRpc is a test service covering unary, error and streaming shapes.
type UnitRpc struct{}

func (u *UnitRpc) Add(a, b int) int { return a + b }

func (u *UnitRpc) Hello(p unitPerson) string { return "hi " + p.Name }

func (u *UnitRpc) Count(n int) []int {
	out := make([]int, n)
	for i := range out {
		out[i] = i + 1
	}
	return out
}

func (u *UnitRpc) Fail(n int) (int, error) {
	if n < 0 {
		return 0, errors.New("negative")
	}
	return n, nil
}

func (u *UnitRpc) NoSplit() int { return 1 }

// newUnitServer builds a streamable HTTP server with the test services.
func newUnitServer(t *testing.T) *StreamableHttpServer {
	t.Helper()
	s, ok := (&StreamableHttp{Port: 0}).NewServer().(*StreamableHttpServer)
	if !ok {
		t.Fatal("NewServer returned unexpected type")
	}
	s.Register(new(UnitRpc))
	return s
}

// unitPost sends one raw request through handleMcp and captures the reply.
func unitPost(t *testing.T, s *StreamableHttpServer, body string, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, McpEndpoint, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	s.handleMcp(rec, req)
	return rec
}

// unitRequest sends one JSON-RPC request and decodes the JSON response.
func unitRequest(t *testing.T, s *StreamableHttpServer, sid string, req map[string]any) map[string]any {
	t.Helper()
	data, err := json.Marshal(req)
	if err != nil {
		t.Fatal(err)
	}
	headers := map[string]string{}
	if sid != "" {
		headers[McpSessionHeader] = sid
	}
	rec := unitPost(t, s, string(data), headers)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d body %s", rec.Code, rec.Body)
	}
	out := map[string]any{}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode %s: %v", rec.Body, err)
	}
	return out
}

// unitInitialize performs the handshake and returns the session id.
func unitInitialize(t *testing.T, s *StreamableHttpServer, version string) (string, map[string]any) {
	t.Helper()
	data, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0", "id": "1", "method": "initialize",
		"params": map[string]any{"protocolVersion": version},
	})
	rec := unitPost(t, s, string(data), nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("initialize status %d body %s", rec.Code, rec.Body)
	}
	sid := rec.Header().Get(McpSessionHeader)
	if sid == "" {
		t.Fatalf("no session id in initialize response: %s", rec.Body)
	}
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	res, _ := out["result"].(map[string]any)
	if res == nil {
		t.Fatalf("initialize result missing: %v", out)
	}
	return sid, res
}

func TestMcpSessionRing(t *testing.T) {
	sess := newMcpSession("2025-06-18")
	if len(sess.id) != 32 {
		t.Errorf("session id %q length %d, want 32", sess.id, len(sess.id))
	}
	if sess.protocolVersion != "2025-06-18" {
		t.Errorf("protocol version %q", sess.protocolVersion)
	}
	other := newMcpSession("2025-06-18")
	if other.id == sess.id {
		t.Error("session ids collide")
	}
	if got := sess.nextID(); got != 1 {
		t.Errorf("first event id %d", got)
	}
	if got := sess.nextID(); got != 2 {
		t.Errorf("second event id %d", got)
	}
	// Fill beyond the ring size and check eviction.
	total := mcpEventRingSize + 10
	for i := 1; i <= total; i++ {
		sess.record(sseEvent{ID: i, Data: []byte("x")})
	}
	if len(sess.ring) != mcpEventRingSize {
		t.Fatalf("ring length %d, want %d", len(sess.ring), mcpEventRingSize)
	}
	// Only events after the marker and still buffered are replayed.
	events := sess.replayAfter(total - 3)
	if len(events) != 3 {
		t.Fatalf("replayed %d events, want 3", len(events))
	}
	for i, ev := range events {
		if ev.ID != total-2+i {
			t.Errorf("replayed id %d, want %d", ev.ID, total-2+i)
		}
	}
	// Evicted ids replay everything still buffered.
	if len(sess.replayAfter(1)) != mcpEventRingSize {
		t.Errorf("evicted replay length wrong")
	}
	if len(sess.replayAfter(total)) != 0 {
		t.Errorf("replay after last should be empty")
	}
}

func TestMcpSubscribeFanout(t *testing.T) {
	sess := newMcpSession("2025-06-18")
	ch, unsub := sess.subscribe()
	sess.record(sseEvent{ID: 1, Data: []byte("a")})
	select {
	case ev := <-ch:
		if ev.ID != 1 || string(ev.Data) != "a" {
			t.Errorf("received %+v", ev)
		}
	default:
		t.Fatal("subscriber did not receive the event")
	}
	unsub()
	// After unsubscribing, events no longer reach the old channel.
	sess.record(sseEvent{ID: 2, Data: []byte("b")})
	select {
	case ev := <-ch:
		t.Errorf("event after unsub: %+v", ev)
	default:
	}
}

func TestCheckHost(t *testing.T) {
	tests := []struct {
		name    string
		allowed []string
		host    string
		wantOK  bool
	}{
		{"empty allowlist", nil, "evil.com", true},
		{"plain match", []string{"localhost"}, "localhost", true},
		{"match with port", []string{"localhost"}, "localhost:3001", true},
		{"allowlist entry with port", []string{"localhost:3001"}, "localhost", true},
		{"ipv6 loopback", []string{"[::1]"}, "[::1]:3001", true},
		{"rebinding host", []string{"localhost", "127.0.0.1"}, "evil.com", false},
		{"case insensitive", []string{"LocalHost"}, "localhost", true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := &StreamableHttpServer{Options: StreamableHttpOptions{AllowedHosts: tc.allowed}}
			req := httptest.NewRequest(http.MethodPost, McpEndpoint, nil)
			req.Host = tc.host
			if ok := s.checkHost(req); ok != tc.wantOK {
				t.Errorf("checkHost(%q)=%v, want %v", tc.host, ok, tc.wantOK)
			}
		})
	}
}

func TestCheckOrigin(t *testing.T) {
	tests := []struct {
		name    string
		allowed []string
		origin  string
		wantOK  bool
	}{
		{"empty allowlist", nil, "http://evil", true},
		{"matching", []string{"http://good"}, "http://good", true},
		{"mismatching", []string{"http://good"}, "http://evil", false},
		{"missing header", []string{"http://good"}, "", true},
		{"second entry", []string{"http://a", "http://b"}, "http://b", true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := &StreamableHttpServer{Options: StreamableHttpOptions{AllowedOrigins: tc.allowed}}
			req := httptest.NewRequest(http.MethodPost, McpEndpoint, nil)
			if tc.origin != "" {
				req.Header.Set("Origin", tc.origin)
			}
			if ok := s.checkOrigin(req); ok != tc.wantOK {
				t.Errorf("checkOrigin(%q)=%v, want %v", tc.origin, ok, tc.wantOK)
			}
		})
	}
}

func TestHandleInitializeNegotiation(t *testing.T) {
	s := newUnitServer(t)
	for _, tc := range []struct {
		version string
		want    string
	}{
		{"2024-11-05", "2024-11-05"},
		{"2025-03-26", "2025-03-26"},
		{"2025-06-18", "2025-06-18"},
		{"2025-11-25", "2025-11-25"},
		{"1999-01-01", mcpLatestVersion},
		{"", mcpLatestVersion},
	} {
		_, res := unitInitialize(t, s, tc.version)
		if res["protocolVersion"] != tc.want {
			t.Errorf("version %q negotiated %v, want %q", tc.version, res["protocolVersion"], tc.want)
		}
		if res["serverInfo"] == nil || res["capabilities"] == nil {
			t.Errorf("result missing fields: %v", res)
		}
	}
}

func TestHandleUnknownSession(t *testing.T) {
	s := newUnitServer(t)
	rec := unitPost(t, s, `{"jsonrpc":"2.0","id":"1","method":"ping"}`, map[string]string{McpSessionHeader: "bogus"})
	if rec.Code != http.StatusNotFound {
		t.Errorf("status %d, want 404", rec.Code)
	}
}

func TestHandlePingAndNotifications(t *testing.T) {
	s := newUnitServer(t)
	sid, _ := unitInitialize(t, s, "2025-06-18")
	resp := unitRequest(t, s, sid, map[string]any{"jsonrpc": "2.0", "id": "2", "method": "ping"})
	res, ok := resp["result"].(map[string]any)
	if !ok || len(res) != 0 {
		t.Errorf("ping result %v", resp["result"])
	}
	rec := unitPost(t, s, `{"jsonrpc":"2.0","method":"notifications/initialized"}`, map[string]string{McpSessionHeader: sid})
	if rec.Code != http.StatusAccepted {
		t.Errorf("notification status %d, want 202", rec.Code)
	}
	// Native notification without id is executed and accepted.
	rec = unitPost(t, s, `{"jsonrpc":"2.0","method":"UnitRpc/Add","params":[1,2]}`, map[string]string{McpSessionHeader: sid})
	if rec.Code != http.StatusAccepted {
		t.Errorf("native notification status %d, want 202", rec.Code)
	}
}

func TestHandleDelete(t *testing.T) {
	s := newUnitServer(t)
	sid, _ := unitInitialize(t, s, "2025-06-18")
	do := func() int {
		req := httptest.NewRequest(http.MethodDelete, McpEndpoint, nil)
		req.Header.Set(McpSessionHeader, sid)
		rec := httptest.NewRecorder()
		s.handleMcp(rec, req)
		return rec.Code
	}
	if got := do(); got != http.StatusNoContent {
		t.Errorf("first delete %d, want 204", got)
	}
	if got := do(); got != http.StatusNotFound {
		t.Errorf("second delete %d, want 404", got)
	}
	// DELETE without a session id is rejected.
	req := httptest.NewRequest(http.MethodDelete, McpEndpoint, nil)
	rec := httptest.NewRecorder()
	s.handleMcp(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("delete without session %d, want 400", rec.Code)
	}
}

func TestHandleGetValidation(t *testing.T) {
	s := newUnitServer(t)
	// GET without a session id.
	req := httptest.NewRequest(http.MethodGet, McpEndpoint, nil)
	rec := httptest.NewRecorder()
	s.handleMcp(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("GET no session %d, want 400", rec.Code)
	}
	// GET with an unknown session id.
	req = httptest.NewRequest(http.MethodGet, McpEndpoint, nil)
	req.Header.Set(McpSessionHeader, "bogus")
	rec = httptest.NewRecorder()
	s.handleMcp(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Errorf("GET bad session %d, want 404", rec.Code)
	}
}

func TestHandleMethodNotAllowed(t *testing.T) {
	s := newUnitServer(t)
	req := httptest.NewRequest(http.MethodPut, McpEndpoint, nil)
	rec := httptest.NewRecorder()
	s.handleMcp(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("PUT status %d, want 405", rec.Code)
	}
}

func TestHandleParseAndBatchErrors(t *testing.T) {
	s := newUnitServer(t)
	rec := unitPost(t, s, "{broken", nil)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("parse status %d, want 400", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Parse error") {
		t.Errorf("parse body %s", rec.Body)
	}
	// Empty batch.
	rec = unitPost(t, s, `[]`, nil)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("empty batch status %d, want 400", rec.Code)
	}
	// Batch with items returns one response per item.
	rec = unitPost(t, s, `[{"jsonrpc":"2.0","id":"1","method":"UnitRpc/Add","params":[2,3]},{"jsonrpc":"2.0","id":"2","method":"UnitRpc/Missing","params":[]}]`, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("batch status %d body %s", rec.Code, rec.Body)
	}
	items, err := common.GetBatchResult(rec.Body.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 || items[0].Err != nil || items[0].Result != float64(5) || items[1].Err == nil {
		t.Errorf("batch items %+v", items)
	}
}

func TestHandleOriginForbidden(t *testing.T) {
	s := newUnitServer(t)
	s.Options.AllowedOrigins = []string{"http://good"}
	rec := unitPost(t, s, `{"jsonrpc":"2.0","id":"1","method":"ping"}`, map[string]string{"Origin": "http://evil"})
	if rec.Code != http.StatusForbidden {
		t.Errorf("status %d, want 403", rec.Code)
	}
	rec = unitPost(t, s, `{"jsonrpc":"2.0","id":"1","method":"ping"}`, map[string]string{"Origin": "http://good"})
	if rec.Code != http.StatusOK {
		t.Errorf("allowed origin status %d", rec.Code)
	}
}

func TestJsonTypeName(t *testing.T) {
	tests := []struct {
		t    reflect.Type
		want string
	}{
		{reflect.TypeFor[int](), "integer"},
		{reflect.TypeFor[uint8](), "integer"},
		{reflect.TypeFor[float64](), "number"},
		{reflect.TypeFor[bool](), "boolean"},
		{reflect.TypeFor[string](), "string"},
		{reflect.TypeFor[[]int](), "array"},
		{reflect.TypeFor[map[string]any](), "object"},
		{reflect.TypeFor[unitPerson](), "object"},
	}
	for _, tc := range tests {
		if got := jsonTypeName(tc.t); got != tc.want {
			t.Errorf("jsonTypeName(%v)=%q, want %q", tc.t, got, tc.want)
		}
	}
}

func TestJsonFieldName(t *testing.T) {
	if got := jsonFieldName(reflect.StructField{Name: "Value"}); got != "value" {
		t.Errorf("no tag: %q", got)
	}
	if got := jsonFieldName(reflect.StructField{Name: "Value", Tag: `json:"custom"`}); got != "custom" {
		t.Errorf("tag name: %q", got)
	}
	if got := jsonFieldName(reflect.StructField{Name: "Value", Tag: `json:"custom,omitempty"`}); got != "custom" {
		t.Errorf("tag options: %q", got)
	}
	if got := jsonFieldName(reflect.StructField{Name: "Value", Tag: `json:"-"`}); got != "value" {
		t.Errorf("skipped field name: %q", got)
	}
}

func TestInputSchema(t *testing.T) {
	// Value parameters are named arg0..argN.
	schema := inputSchema([]reflect.Type{reflect.TypeFor[int](), reflect.TypeFor[string]()})
	props, _ := schema["properties"].(map[string]any)
	if props["arg0"] == nil || props["arg1"] == nil {
		t.Fatalf("properties %v", props)
	}
	if arg0, _ := props["arg0"].(map[string]any); arg0["type"] != "integer" {
		t.Errorf("arg0 schema %v", arg0)
	}
	required, _ := schema["required"].([]string)
	if len(required) != 2 {
		t.Errorf("required %v", required)
	}
	if schema["type"] != "object" {
		t.Errorf("schema type %v", schema["type"])
	}
	// Struct parameters expand to JSON-tagged exported fields.
	person := unitPerson{Name: "amy", Age: 3, Skip: "gone", secret: "hidden"}
	schema = inputSchema([]reflect.Type{reflect.TypeOf(person)})
	props, _ = schema["properties"].(map[string]any)
	if props["name"] == nil || props["age"] == nil {
		t.Errorf("struct properties %v", props)
	}
	if props["secret"] != nil {
		t.Errorf("unexported field leaked into schema: %v", props)
	}
	if props["skip"] != nil {
		t.Errorf("json:\"-\" field leaked into schema: %v", props)
	}
	// No parameters yields an empty object schema.
	schema = inputSchema(nil)
	props, _ = schema["properties"].(map[string]any)
	if len(props) != 0 {
		t.Errorf("empty properties %v", props)
	}
}

func TestBindParamsAndArguments(t *testing.T) {
	types := []reflect.Type{reflect.TypeFor[int](), reflect.TypeFor[unitPerson]()}
	params := bindParams(types)
	if len(params) != 2 {
		t.Fatalf("bindParams %+v", params)
	}
	if params[0].name != "arg0" {
		t.Errorf("first param %q", params[0].name)
	}
	if params[1].name != "" || len(params[1].fields) != 2 ||
		params[1].fields[0] != "name" || params[1].fields[1] != "age" {
		t.Errorf("struct binding %+v", params[1])
	}
	// json:"-" fields are excluded because they never decode.
	// bindArguments maps the tool arguments back to positional values.
	arguments := map[string]any{"arg0": 7, "name": "bob", "age": 3, "extra": 1}
	args := bindArguments(params, types, arguments)
	if len(args) != 2 || args[0] != 7 {
		t.Fatalf("args %+v", args)
	}
	obj, ok := args[1].(map[string]any)
	if !ok || obj["name"] != "bob" || obj["age"] != 3 {
		t.Errorf("struct arg %+v", args[1])
	}
	if obj["extra"] != nil {
		t.Errorf("unknown argument leaked: %+v", obj)
	}
	// Missing keys become nil values.
	args = bindArguments(params, types, map[string]any{})
	if args[0] != nil {
		t.Errorf("missing value argument %+v", args[0])
	}
	obj, _ = args[1].(map[string]any)
	if obj["name"] != nil || obj["age"] != nil {
		t.Errorf("missing struct fields %+v", obj)
	}
}

func TestCallToolResponse(t *testing.T) {
	// Success wraps the JSON-encoded result as text content.
	succ, ok := callToolResponse("1", common.S("1", common.JsonRpc, 3)).(common.SuccessResponse)
	if !ok {
		t.Fatal("callToolResponse did not return a success response")
	}
	if succ.Id != "1" {
		t.Errorf("id %q", succ.Id)
	}
	env, _ := succ.Result.(map[string]any)
	if env == nil || env["isError"] != false {
		t.Fatalf("envelope %v", succ.Result)
	}
	content, _ := env["content"].([]map[string]any)
	if len(content) != 1 || content[0]["type"] != "text" || content[0]["text"] != "3" {
		t.Errorf("content %v", env["content"])
	}
	// Method errors become isError tool results.
	for _, code := range []int{common.InternalError, common.CustomError} {
		succ, ok := callToolResponse("1", common.E("1", common.JsonRpc, code)).(common.SuccessResponse)
		if !ok {
			t.Fatalf("code %d not converted to tool result", code)
		}
		env, _ := succ.Result.(map[string]any)
		if env["isError"] != true {
			t.Errorf("code %d isError %v", code, env["isError"])
		}
		content, _ := env["content"].([]map[string]any)
		if content[0]["text"] != common.CodeMap[code] {
			t.Errorf("code %d text %v", code, content[0]["text"])
		}
	}
	// Protocol errors stay JSON-RPC errors.
	errResp, ok := callToolResponse("1", common.E("1", common.JsonRpc, common.MethodNotFound)).(common.ErrorResponse)
	if !ok || errResp.Error.Code != common.MethodNotFound {
		t.Errorf("passthrough %+v", errResp)
	}
	// Anything else is passed through untouched.
	notify := common.S(nil, common.JsonRpc, 5)
	if got := callToolResponse("1", notify); !reflect.DeepEqual(got, notify) {
		t.Errorf("passthrough of %T changed", notify)
	}
}

func TestListTools(t *testing.T) {
	s := newUnitServer(t)
	tools := s.listTools()
	names := make([]string, len(tools))
	for i, tool := range tools {
		names[i] = tool["name"].(string)
		if tool["description"] == "" {
			t.Errorf("tool %s missing description", names[i])
		}
		if tool["inputSchema"] == nil {
			t.Errorf("tool %s missing inputSchema", names[i])
		}
	}
	want := []string{"UnitRpc_Add", "UnitRpc_Count", "UnitRpc_Fail", "UnitRpc_Hello", "UnitRpc_NoSplit"}
	if !reflect.DeepEqual(names, want) {
		t.Errorf("tool names %v, want %v", names, want)
	}
}

func TestLookupTool(t *testing.T) {
	s := newUnitServer(t)
	if _, m := s.lookupTool("UnitRpc", "Add"); m == nil {
		t.Error("exact lookup failed")
	}
	// The LineToHump fallback maps snake_case service names to registered ones.
	if _, m := s.lookupTool("unit_rpc", "Add"); m == nil {
		t.Error("LineToHump fallback failed")
	}
	if _, m := s.lookupTool("UnitRpc", "Missing"); m != nil {
		t.Error("missing method found")
	}
	if _, m := s.lookupTool("nope", "Add"); m != nil {
		t.Error("missing service found")
	}
}

func TestToolsCallUnary(t *testing.T) {
	s := newUnitServer(t)
	resp := unitRequest(t, s, "", map[string]any{
		"jsonrpc": "2.0", "id": "1", "method": "tools/call",
		"params": map[string]any{"name": "UnitRpc_Add", "arguments": map[string]any{"arg0": 1, "arg1": 2}},
	})
	res, _ := resp["result"].(map[string]any)
	if res == nil || res["isError"] != false {
		t.Fatalf("tools/call result %v", resp)
	}
	content, _ := res["content"].([]any)
	item, _ := content[0].(map[string]any)
	if item["text"] != "3" {
		t.Errorf("content text %v", item["text"])
	}
}

func TestToolsCallStructArgument(t *testing.T) {
	s := newUnitServer(t)
	resp := unitRequest(t, s, "", map[string]any{
		"jsonrpc": "2.0", "id": "1", "method": "tools/call",
		"params": map[string]any{"name": "UnitRpc_Hello", "arguments": map[string]any{"name": "amy"}},
	})
	res, _ := resp["result"].(map[string]any)
	content, _ := res["content"].([]any)
	item, _ := content[0].(map[string]any)
	if item["text"] != `"hi amy"` {
		t.Errorf("content text %v", item["text"])
	}
}

func TestToolsCallErrors(t *testing.T) {
	s := newUnitServer(t)
	// Unknown tool keeps JSON-RPC error semantics.
	resp := unitRequest(t, s, "", map[string]any{
		"jsonrpc": "2.0", "id": "1", "method": "tools/call",
		"params": map[string]any{"name": "missing_tool", "arguments": map[string]any{}},
	})
	errObj, _ := resp["error"].(map[string]any)
	if errObj == nil || errObj["code"] != float64(common.MethodNotFound) {
		t.Errorf("unknown tool %v", resp)
	}
	// A name without the separator is an invalid parameter.
	resp = unitRequest(t, s, "", map[string]any{
		"jsonrpc": "2.0", "id": "2", "method": "tools/call",
		"params": map[string]any{"name": "noseparator"},
	})
	errObj, _ = resp["error"].(map[string]any)
	if errObj == nil || errObj["code"] != float64(common.InvalidParams) {
		t.Errorf("bad name %v", resp)
	}
	// Missing params are invalid too.
	resp = unitRequest(t, s, "", map[string]any{"jsonrpc": "2.0", "id": "3", "method": "tools/call"})
	errObj, _ = resp["error"].(map[string]any)
	if errObj == nil || errObj["code"] != float64(common.InvalidParams) {
		t.Errorf("missing params %v", resp)
	}
	// Method errors surface as isError tool results.
	resp = unitRequest(t, s, "", map[string]any{
		"jsonrpc": "2.0", "id": "4", "method": "tools/call",
		"params": map[string]any{"name": "UnitRpc_Fail", "arguments": map[string]any{"arg0": -1}},
	})
	res, _ := resp["result"].(map[string]any)
	if res == nil || res["isError"] != true {
		t.Errorf("tool error result %v", resp)
	}
}

func TestToolsCallProgress(t *testing.T) {
	s := newUnitServer(t)
	sid, _ := unitInitialize(t, s, "2025-06-18")
	data, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0", "id": "9", "method": "tools/call",
		"params": map[string]any{
			"name":      "UnitRpc_Count",
			"arguments": map[string]any{"arg0": 3},
			"_meta":     map[string]any{"progressToken": "tok"},
		},
	})
	rec := unitPost(t, s, string(data), map[string]string{McpSessionHeader: sid})
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/event-stream") {
		t.Fatalf("content type %q", ct)
	}
	body := rec.Body.String()
	if got := strings.Count(body, "notifications/progress"); got != 3 {
		t.Errorf("progress events %d, want 3 body %s", got, body)
	}
	if !strings.Contains(body, `"progressToken":"tok"`) {
		t.Errorf("progress token missing in %s", body)
	}
	for i := 1; i <= 3; i++ {
		if !strings.Contains(body, `"progress":`+string(rune('0'+i))) {
			t.Errorf("progress %d missing", i)
		}
	}
	// The final event carries the CallToolResult envelope.
	if !strings.Contains(body, `"isError":false`) {
		t.Errorf("final envelope missing in %s", body)
	}
}

func TestToolsCallStreamWithoutToken(t *testing.T) {
	s := newUnitServer(t)
	resp := unitRequest(t, s, "", map[string]any{
		"jsonrpc": "2.0", "id": "1", "method": "tools/call",
		"params": map[string]any{"name": "UnitRpc_Count", "arguments": map[string]any{"arg0": 2}},
	})
	res, _ := resp["result"].(map[string]any)
	content, _ := res["content"].([]any)
	item, _ := content[0].(map[string]any)
	if item["text"] != "[1,2]" {
		t.Errorf("collected stream text %v", item["text"])
	}
}

func TestNativeDispatchAndStream(t *testing.T) {
	s := newUnitServer(t)
	// Legacy Service/method dispatch stays compatible.
	resp := unitRequest(t, s, "", map[string]any{
		"jsonrpc": "2.0", "id": "1", "method": "UnitRpc/Add", "params": []any{1, 2},
	})
	if resp["result"] != float64(3) {
		t.Errorf("legacy result %v", resp)
	}
	// Streaming methods answer with SSE when a session exists.
	sid, _ := unitInitialize(t, s, "2025-06-18")
	data, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0", "id": "2", "method": "UnitRpc/Count", "params": []any{2},
	})
	rec := unitPost(t, s, string(data), map[string]string{McpSessionHeader: sid})
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/event-stream") {
		t.Fatalf("content type %q body %s", ct, rec.Body)
	}
	body := rec.Body.String()
	if got := strings.Count(body, "notifications/stream"); got != 2 {
		t.Errorf("stream events %d, want 2 body %s", got, body)
	}
	if !strings.Contains(body, `"id":"2"`) {
		t.Errorf("final response event missing in %s", body)
	}
}

func TestSetOptions(t *testing.T) {
	s := (&StreamableHttp{Port: 0}).NewServer().(*StreamableHttpServer)
	s.SetOptions(StreamableHttpOptions{AllowedOrigins: []string{"http://a"}})
	if len(s.Options.AllowedOrigins) != 1 || s.Options.AllowedOrigins[0] != "http://a" {
		t.Errorf("options %+v", s.Options)
	}
}

func TestMcpSessionConcurrency(t *testing.T) {
	sess := newMcpSession("2025-06-18")
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				sess.record(sseEvent{ID: sess.nextID(), Data: []byte("v")})
			}
		}()
	}
	wg.Wait()
	if len(sess.ring) != mcpEventRingSize {
		t.Errorf("ring length %d", len(sess.ring))
	}
	ids := map[int]bool{}
	for _, ev := range sess.ring {
		if ids[ev.ID] {
			t.Fatalf("duplicate event id %d", ev.ID)
		}
		ids[ev.ID] = true
	}
}

func TestInitializeCapabilities(t *testing.T) {
	s := newUnitServer(t)
	_, res := unitInitialize(t, s, "2025-11-25")
	caps, _ := res["capabilities"].(map[string]any)
	for _, name := range []string{"tools", "resources", "prompts", "logging", "completions"} {
		if caps[name] == nil {
			t.Errorf("capability %q missing: %v", name, caps)
		}
	}
	resources, _ := caps["resources"].(map[string]any)
	if resources["subscribe"] != true {
		t.Errorf("resources.subscribe %v", resources["subscribe"])
	}
}

func TestSessionRequestRouting(t *testing.T) {
	sess := newMcpSession("2025-11-25")
	if id := sess.nextRequestID(); id != "srv-1" {
		t.Errorf("first request id %q", id)
	}
	if id := sess.nextRequestID(); id != "srv-2" {
		t.Errorf("second request id %q", id)
	}
	ch := sess.beginRequest("srv-3")
	if !sess.deliverResponse("srv-3", "value", nil) {
		t.Fatal("deliverResponse lost the pending request")
	}
	reply := <-ch
	if reply.result != "value" || reply.err != nil {
		t.Errorf("reply %+v", reply)
	}
	if sess.deliverResponse("srv-4", nil, nil) {
		t.Error("unknown id accepted")
	}
	// Error objects convert to reply errors carrying the message.
	ch = sess.beginRequest("srv-5")
	sess.deliverResponse("srv-5", nil, map[string]any{"code": -32602, "message": "declined"})
	reply = <-ch
	if reply.err == nil || reply.err.Error() != "declined" {
		t.Errorf("error reply %+v", reply)
	}
	sess.endRequest("srv-5")
	if sess.deliverResponse("srv-5", nil, nil) {
		t.Error("endRequest did not unregister the pending request")
	}
}

func TestClientResponsePost(t *testing.T) {
	s := newUnitServer(t)
	sid, _ := unitInitialize(t, s, "2025-11-25")
	v, ok := s.Sessions.Load(sid)
	if !ok {
		t.Fatal("session not stored")
	}
	sess := v.(*mcpSession)
	ch := sess.beginRequest("srv-1")
	rec := unitPost(t, s, `{"jsonrpc":"2.0","id":"srv-1","result":{"action":"accept"}}`,
		map[string]string{McpSessionHeader: sid})
	if rec.Code != http.StatusAccepted {
		t.Fatalf("status %d body %s", rec.Code, rec.Body)
	}
	reply := <-ch
	res, _ := reply.result.(map[string]any)
	if res["action"] != "accept" || reply.err != nil {
		t.Errorf("reply %+v", reply)
	}
	// Responses to unknown ids are accepted silently.
	rec = unitPost(t, s, `{"jsonrpc":"2.0","id":"srv-9","result":{}}`,
		map[string]string{McpSessionHeader: sid})
	if rec.Code != http.StatusAccepted {
		t.Errorf("unknown id status %d", rec.Code)
	}
	// Error objects route as errors.
	ch = sess.beginRequest("srv-2")
	rec = unitPost(t, s, `{"jsonrpc":"2.0","id":"srv-2","error":{"code":-32602,"message":"declined"}}`,
		map[string]string{McpSessionHeader: sid})
	if rec.Code != http.StatusAccepted {
		t.Fatalf("error response status %d", rec.Code)
	}
	reply = <-ch
	if reply.err == nil || reply.err.Error() != "declined" {
		t.Errorf("error reply %+v", reply)
	}
}

func TestMcpToolResponse(t *testing.T) {
	// Handler errors become isError tool results.
	resp, ok := mcpToolResponse("1", nil, errors.New("boom")).(common.SuccessResponse)
	if !ok {
		t.Fatal("error result not converted to a tool result")
	}
	env, _ := resp.Result.(map[string]any)
	if env["isError"] != true {
		t.Errorf("isError %v", env["isError"])
	}
	content, _ := env["content"].([]map[string]any)
	if len(content) != 1 || content[0]["text"] != "boom" {
		t.Errorf("content %v", env["content"])
	}
	// Maps carrying "content" pass through as full CallToolResults.
	full := map[string]any{
		"content": []map[string]any{{"type": "image", "data": "AAA=", "mimeType": "image/png"}},
		"isError": false,
	}
	resp, _ = mcpToolResponse("2", full, nil).(common.SuccessResponse)
	env, _ = resp.Result.(map[string]any)
	items, _ := env["content"].([]map[string]any)
	if len(items) != 1 || items[0]["type"] != "image" {
		t.Errorf("passthrough content %v", env["content"])
	}
	// Plain values are JSON-encoded as text content.
	resp, _ = mcpToolResponse("3", map[string]any{"a": 1}, nil).(common.SuccessResponse)
	env, _ = resp.Result.(map[string]any)
	items, _ = env["content"].([]map[string]any)
	if env["isError"] != false || items[0]["text"] != `{"a":1}` {
		t.Errorf("encoded result %v", env)
	}
}

func TestMatchResourceTemplate(t *testing.T) {
	tests := []struct {
		tmpl string
		uri  string
		want bool
	}{
		{"test://template/{id}/data", "test://template/42/data", true},
		{"test://template/{id}/data", "test://template//data", false},
		{"test://template/{id}/data", "test://template/42/other", false},
		{"test://template/{id}/data", "test://template/42", false},
		{"test://static", "test://static", true},
	}
	for _, tc := range tests {
		if got := matchResourceTemplate(tc.tmpl, tc.uri); got != tc.want {
			t.Errorf("matchResourceTemplate(%q, %q)=%v, want %v", tc.tmpl, tc.uri, got, tc.want)
		}
	}
}

func TestRegisterMcpToolListing(t *testing.T) {
	s := newUnitServer(t)
	handler := func(ctx *McpContext, a map[string]any) (any, error) { return nil, nil }
	schema := map[string]any{"type": "object", "properties": map[string]any{"x": map[string]any{"type": "string"}}}
	s.RegisterMcpTool(McpTool{Name: "exact_tool", Description: "first", InputSchema: schema, Handler: handler})
	s.RegisterMcpTool(McpTool{Name: "second_tool", Description: "second", Handler: handler})
	// Re-registering keeps the original position and replaces the definition.
	s.RegisterMcpTool(McpTool{Name: "exact_tool", Description: "updated", InputSchema: schema, Handler: handler})
	tools := s.listTools()
	if len(tools) != 7 {
		t.Fatalf("tool count %d, want 7", len(tools))
	}
	if tools[5]["name"] != "exact_tool" || tools[6]["name"] != "second_tool" {
		t.Errorf("custom tool order %v %v", tools[5]["name"], tools[6]["name"])
	}
	if tools[5]["description"] != "updated" {
		t.Errorf("re-registration kept %v", tools[5]["description"])
	}
	got, _ := tools[5]["inputSchema"].(map[string]any)
	if !reflect.DeepEqual(got, schema) {
		t.Errorf("custom schema %v", got)
	}
	// A nil schema falls back to an empty object schema.
	def, _ := tools[6]["inputSchema"].(map[string]any)
	if def["type"] != "object" {
		t.Errorf("default schema %v", def)
	}
}

func TestResourceMethods(t *testing.T) {
	s := newUnitServer(t)
	s.RegisterMcpResource(McpResource{URI: "test://text", Name: "Text", Title: "T", Description: "d", Text: "hello"})
	s.RegisterMcpResource(McpResource{URI: "test://bin", Name: "Bin", Blob: []byte{1, 2, 3}})
	s.RegisterMcpResourceTemplate(McpResourceTemplate{
		URITemplate: "test://t/{id}", Name: "Tpl", MimeType: "text/markdown",
		Read: func(uri string) (string, []byte, bool) { return "read " + uri, nil, true },
	})
	s.RegisterMcpResourceTemplate(McpResourceTemplate{URITemplate: "test://dead/{id}", Name: "Dead"})
	resp := unitRequest(t, s, "", map[string]any{"jsonrpc": "2.0", "id": "1", "method": "resources/list"})
	res, _ := resp["result"].(map[string]any)
	items, _ := res["resources"].([]any)
	if len(items) != 2 {
		t.Fatalf("resources %v", res["resources"])
	}
	first, _ := items[0].(map[string]any)
	if first["uri"] != "test://text" || first["title"] != "T" || first["description"] != "d" {
		t.Errorf("resource item %v", first)
	}
	if first["mimeType"] != nil {
		t.Errorf("empty mimeType should be omitted: %v", first)
	}
	second, _ := items[1].(map[string]any)
	if second["title"] != nil || second["description"] != nil {
		t.Errorf("empty fields leaked: %v", second)
	}
	resp = unitRequest(t, s, "", map[string]any{"jsonrpc": "2.0", "id": "2", "method": "resources/templates/list"})
	res, _ = resp["result"].(map[string]any)
	tpls, _ := res["resourceTemplates"].([]any)
	if len(tpls) != 2 {
		t.Fatalf("templates %v", res["resourceTemplates"])
	}
	read := func(id, uri string) map[string]any {
		resp := unitRequest(t, s, "", map[string]any{
			"jsonrpc": "2.0", "id": id, "method": "resources/read",
			"params": map[string]any{"uri": uri},
		})
		res, _ := resp["result"].(map[string]any)
		if res != nil {
			return res
		}
		errObj, _ := resp["error"].(map[string]any)
		if errObj == nil {
			t.Fatalf("read %s: %v", uri, resp)
		}
		if errObj["code"] != float64(common.InvalidParams) {
			t.Errorf("read %s error %v", uri, errObj)
		}
		return nil
	}
	// Static text resource.
	res = read("3", "test://text")
	contents, _ := res["contents"].([]any)
	item, _ := contents[0].(map[string]any)
	if item["text"] != "hello" || item["mimeType"] != "text/plain" {
		t.Errorf("text read %v", item)
	}
	// Binary resource is base64 encoded.
	res = read("4", "test://bin")
	contents, _ = res["contents"].([]any)
	item, _ = contents[0].(map[string]any)
	if item["blob"] != base64.StdEncoding.EncodeToString([]byte{1, 2, 3}) ||
		item["mimeType"] != "application/octet-stream" {
		t.Errorf("binary read %v", item)
	}
	// Template read uses the registered mimeType.
	res = read("5", "test://t/9")
	contents, _ = res["contents"].([]any)
	item, _ = contents[0].(map[string]any)
	if item["text"] != "read test://t/9" || item["mimeType"] != "text/markdown" {
		t.Errorf("template read %v", item)
	}
	// A template without a reader and an unknown uri are invalid params.
	if res = read("6", "test://dead/1"); res != nil {
		t.Errorf("dead template returned %v", res)
	}
	if res = read("7", "test://nope"); res != nil {
		t.Errorf("unknown uri returned %v", res)
	}
	// Subscribe and unsubscribe answer with empty results.
	for _, method := range []string{"resources/subscribe", "resources/unsubscribe"} {
		resp = unitRequest(t, s, "", map[string]any{
			"jsonrpc": "2.0", "id": "8", "method": method,
			"params": map[string]any{"uri": "test://text"},
		})
		res, _ = resp["result"].(map[string]any)
		if res == nil || len(res) != 0 {
			t.Errorf("%s result %v", method, resp)
		}
	}
}

func TestPromptMethods(t *testing.T) {
	s := newUnitServer(t)
	s.RegisterMcpPrompt(McpPrompt{
		Name: "greet", Title: "G", Description: "greeting",
		Arguments: []map[string]any{{"name": "who", "required": true}},
		Render: func(a map[string]any) (map[string]any, error) {
			who, _ := a["who"].(string)
			return map[string]any{"description": "greeting", "messages": []map[string]any{
				{"role": "user", "content": map[string]any{"type": "text", "text": "hi " + who}},
			}}, nil
		},
	})
	s.RegisterMcpPrompt(McpPrompt{
		Name: "broken", Description: "d",
		Render: func(a map[string]any) (map[string]any, error) { return nil, errors.New("render failed") },
	})
	s.RegisterMcpPrompt(McpPrompt{Name: "bare", Description: "no renderer"})
	resp := unitRequest(t, s, "", map[string]any{"jsonrpc": "2.0", "id": "1", "method": "prompts/list"})
	res, _ := resp["result"].(map[string]any)
	prompts, _ := res["prompts"].([]any)
	if len(prompts) != 3 {
		t.Fatalf("prompts %v", res["prompts"])
	}
	first, _ := prompts[0].(map[string]any)
	if first["name"] != "greet" || first["title"] != "G" || first["arguments"] == nil {
		t.Errorf("prompt item %v", first)
	}
	bare, _ := prompts[2].(map[string]any)
	if bare["title"] != nil || bare["arguments"] != nil {
		t.Errorf("empty prompt fields leaked: %v", bare)
	}
	resp = unitRequest(t, s, "", map[string]any{
		"jsonrpc": "2.0", "id": "2", "method": "prompts/get",
		"params": map[string]any{"name": "greet", "arguments": map[string]any{"who": "amy"}},
	})
	res, _ = resp["result"].(map[string]any)
	messages, _ := res["messages"].([]any)
	msg, _ := messages[0].(map[string]any)
	content, _ := msg["content"].(map[string]any)
	if content["text"] != "hi amy" {
		t.Errorf("rendered prompt %v", res)
	}
	// A failing renderer surfaces the message as a JSON-RPC error.
	resp = unitRequest(t, s, "", map[string]any{
		"jsonrpc": "2.0", "id": "3", "method": "prompts/get",
		"params": map[string]any{"name": "broken"},
	})
	errObj, _ := resp["error"].(map[string]any)
	if errObj == nil || errObj["message"] != "render failed" {
		t.Errorf("render error %v", resp)
	}
	// Unknown names and prompts without a renderer are invalid params.
	for _, name := range []string{"bare", "missing"} {
		resp = unitRequest(t, s, "", map[string]any{
			"jsonrpc": "2.0", "id": "4", "method": "prompts/get",
			"params": map[string]any{"name": name},
		})
		errObj, _ = resp["error"].(map[string]any)
		if errObj == nil || errObj["code"] != float64(common.InvalidParams) {
			t.Errorf("prompt %s error %v", name, resp)
		}
	}
}

func TestCompletionAndLogging(t *testing.T) {
	s := newUnitServer(t)
	resp := unitRequest(t, s, "", map[string]any{
		"jsonrpc": "2.0", "id": "1", "method": "completion/complete",
		"params": map[string]any{
			"ref":      map[string]any{"type": "ref/prompt", "name": "x"},
			"argument": map[string]any{"name": "a", "value": ""},
		},
	})
	res, _ := resp["result"].(map[string]any)
	comp, _ := res["completion"].(map[string]any)
	if comp == nil || comp["values"] == nil || comp["hasMore"] != false {
		t.Errorf("completion %v", res)
	}
	resp = unitRequest(t, s, "", map[string]any{
		"jsonrpc": "2.0", "id": "2", "method": "logging/setLevel",
		"params": map[string]any{"level": "info"},
	})
	res, _ = resp["result"].(map[string]any)
	if res == nil || len(res) != 0 {
		t.Errorf("setLevel result %v", resp)
	}
}

func TestCustomToolStream(t *testing.T) {
	s := newUnitServer(t)
	sid, _ := unitInitialize(t, s, "2025-11-25")
	s.RegisterMcpTool(McpTool{
		Name:        "stream_tool",
		Description: "streams events",
		Handler: func(ctx *McpContext, a map[string]any) (any, error) {
			if ctx.ProtocolVersion() != "2025-11-25" {
				t.Errorf("protocol version %q", ctx.ProtocolVersion())
			}
			_ = ctx.Log("warning", "unit", "careful")
			_ = ctx.Progress(1, 2, "halfway")
			return "done", nil
		},
	})
	data, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0", "id": "7", "method": "tools/call",
		"params": map[string]any{
			"name":      "stream_tool",
			"arguments": map[string]any{},
			"_meta":     map[string]any{"progressToken": "tok"},
		},
	})
	rec := unitPost(t, s, string(data), map[string]string{McpSessionHeader: sid})
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/event-stream") {
		t.Fatalf("content type %q body %s", ct, rec.Body)
	}
	body := rec.Body.String()
	// SEP-1699 priming: the stream starts with a retry field frame.
	if !strings.HasPrefix(body, "retry: 2000\n\n") {
		t.Errorf("missing SSE priming retry frame: %s", body)
	}
	if !strings.Contains(body, "notifications/message") {
		t.Errorf("log event missing in %s", body)
	}
	if !strings.Contains(body, `"progressToken":"tok"`) {
		t.Errorf("progress event missing in %s", body)
	}
	if !strings.Contains(body, `"isError":false`) {
		t.Errorf("final envelope missing in %s", body)
	}
}

func TestCustomToolUnary(t *testing.T) {
	s := newUnitServer(t)
	s.RegisterMcpTool(McpTool{
		Name: "unary_tool", Description: "json only",
		Handler: func(ctx *McpContext, a map[string]any) (any, error) {
			_ = ctx.Progress(1, 2, "ignored") // no progressToken: stays silent
			return map[string]any{"n": 4}, nil
		},
	})
	resp := unitRequest(t, s, "", map[string]any{
		"jsonrpc": "2.0", "id": "1", "method": "tools/call",
		"params": map[string]any{"name": "unary_tool", "arguments": map[string]any{}},
	})
	res, _ := resp["result"].(map[string]any)
	content, _ := res["content"].([]any)
	item, _ := content[0].(map[string]any)
	if item["text"] != `{"n":4}` {
		t.Errorf("content %v", item["text"])
	}
	// Full CallToolResult maps pass through unchanged.
	s.RegisterMcpTool(McpTool{
		Name: "rich_tool", Description: "rich",
		Handler: func(ctx *McpContext, a map[string]any) (any, error) {
			return map[string]any{
				"content": []map[string]any{{"type": "image", "data": "AAA=", "mimeType": "image/png"}},
			}, nil
		},
	})
	resp = unitRequest(t, s, "", map[string]any{
		"jsonrpc": "2.0", "id": "2", "method": "tools/call",
		"params": map[string]any{"name": "rich_tool", "arguments": map[string]any{}},
	})
	res, _ = resp["result"].(map[string]any)
	content, _ = res["content"].([]any)
	item, _ = content[0].(map[string]any)
	if item["type"] != "image" || item["data"] != "AAA=" {
		t.Errorf("passthrough %v", res)
	}
}

func TestMcpContextRequest(t *testing.T) {
	// A session is required for server-to-client requests.
	if _, err := (&McpContext{}).Request("sampling/createMessage", nil, time.Second); err == nil {
		t.Error("request without a session succeeded")
	}
	s := newUnitServer(t)
	sid, _ := unitInitialize(t, s, "2025-11-25")
	v, _ := s.Sessions.Load(sid)
	sess := v.(*mcpSession)
	rec := httptest.NewRecorder()
	ctx := &McpContext{sess: sess, cw: &sseWriter{w: rec, sess: sess}, id: "1"}
	done := make(chan struct{})
	var (
		res any
		err error
	)
	go func() {
		defer close(done)
		res, err = ctx.Request("elicitation/create", map[string]any{"message": "hi"}, 5*time.Second)
	}()
	// Wait for the pending request to register, then answer it.
	for i := 0; i < 500; i++ {
		sess.mu.Lock()
		_, ok := sess.pending["srv-1"]
		sess.mu.Unlock()
		if ok {
			break
		}
		time.Sleep(time.Millisecond)
	}
	if !sess.deliverResponse("srv-1", map[string]any{"action": "accept"}, nil) {
		t.Fatal("pending request not found")
	}
	<-done
	if err != nil {
		t.Fatal(err)
	}
	out, _ := res.(map[string]any)
	if out["action"] != "accept" {
		t.Errorf("result %v", res)
	}
	// The request frame was written to the stream.
	if !strings.Contains(rec.Body.String(), `"method":"elicitation/create"`) {
		t.Errorf("request event missing in %s", rec.Body)
	}
	// Without a client answer the call times out.
	ctx2 := &McpContext{sess: sess, cw: &sseWriter{w: httptest.NewRecorder(), sess: sess}, id: "2"}
	if _, err := ctx2.Request("sampling/createMessage", nil, 50*time.Millisecond); err == nil {
		t.Error("timeout did not produce an error")
	}
}

func TestEndStreamSendResultReplay(t *testing.T) {
	s := newUnitServer(t)
	sid, _ := unitInitialize(t, s, "2025-11-25")
	s.RegisterMcpTool(McpTool{
		Name:        "reconnect_tool",
		Description: "ends the stream early",
		Handler: func(ctx *McpContext, a map[string]any) (any, error) {
			_ = ctx.Log("info", "unit", "started")
			ctx.EndStream()
			ctx.SendResult(map[string]any{
				"content": []map[string]any{{"type": "text", "text": "late result"}},
				"isError": false,
			})
			return nil, nil
		},
	})
	data, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0", "id": "8", "method": "tools/call",
		"params": map[string]any{"name": "reconnect_tool", "arguments": map[string]any{}},
	})
	rec := unitPost(t, s, string(data), map[string]string{McpSessionHeader: sid})
	body := rec.Body.String()
	if !strings.Contains(body, "notifications/message") {
		t.Errorf("log event missing in %s", body)
	}
	if strings.Contains(body, "late result") {
		t.Error("result written to the ended stream")
	}
	v, _ := s.Sessions.Load(sid)
	sess := v.(*mcpSession)
	sess.mu.Lock()
	if len(sess.ring) == 0 {
		sess.mu.Unlock()
		t.Fatal("final result not recorded for replay")
	}
	lastID := sess.ring[len(sess.ring)-1].ID
	sess.mu.Unlock()
	// Reconnecting with Last-Event-ID replays the final result.
	req := httptest.NewRequest(http.MethodGet, McpEndpoint, nil)
	req.Header.Set(McpSessionHeader, sid)
	req.Header.Set(McpLastEventHeader, fmt.Sprint(lastID-1))
	reqCtx, cancel := context.WithCancel(req.Context())
	req = req.WithContext(reqCtx)
	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()
	rec = httptest.NewRecorder()
	s.handleGet(rec, req)
	if !strings.Contains(rec.Body.String(), "late result") {
		t.Errorf("replay missing final result: %s", rec.Body)
	}
}
