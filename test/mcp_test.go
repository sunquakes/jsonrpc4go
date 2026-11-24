package test

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/sunquakes/jsonrpc4go/common"
)

// Person is a struct parameter used to check inputSchema field expansion.
type Person struct {
	Name string `json:"name"`
	Age  int    `json:"age"`
}

type GreetRpc struct{}

func (g *GreetRpc) Hello(p Person) string {
	return fmt.Sprintf("hi %s %d", p.Name, p.Age)
}

// mcpStart launches a streamable HTTP server with the MCP test services.
func mcpStart(t *testing.T) (string, string) {
	t.Helper()
	addr := shStart(t, nil, new(IntRpc), new(StreamRpc), new(GreetRpc))
	sid, _, _ := shInitialize(t, addr, "2025-06-18")
	return addr, sid
}

// mcpRequest sends one MCP JSON-RPC request and returns the decoded response.
func mcpRequest(t *testing.T, addr, sid string, req map[string]any) map[string]any {
	t.Helper()
	return shRequestJSON(t, addr, sid, req)
}

func mcpResult(t *testing.T, resp map[string]any) map[string]any {
	t.Helper()
	res, _ := resp["result"].(map[string]any)
	if res == nil {
		t.Fatalf("missing result in %v", resp)
	}
	return res
}

func mcpFindTool(t *testing.T, tools []any, name string) map[string]any {
	t.Helper()
	for _, v := range tools {
		tool, _ := v.(map[string]any)
		if tool["name"] == name {
			return tool
		}
	}
	t.Fatalf("tool %s not found in %v", name, tools)
	return nil
}

func TestMcpInitializeResult(t *testing.T) {
	addr := shStart(t, nil, new(IntRpc))
	sid, _, result := shInitialize(t, addr, "2024-11-05")
	if sid == "" {
		t.Fatal("no session id")
	}
	if result["protocolVersion"] != "2024-11-05" {
		t.Errorf("protocolVersion %v", result["protocolVersion"])
	}
	if result["serverInfo"] == nil || result["capabilities"] == nil {
		t.Errorf("initialize result %v", result)
	}
}

func TestMcpPing(t *testing.T) {
	addr, sid := mcpStart(t)
	resp := mcpRequest(t, addr, sid, map[string]any{"jsonrpc": "2.0", "id": "1", "method": "ping"})
	if resp["error"] != nil {
		t.Fatalf("ping error %v", resp["error"])
	}
	res, _ := resp["result"].(map[string]any)
	if res == nil {
		t.Errorf("ping result %v", resp["result"])
	}
}

func TestMcpToolsList(t *testing.T) {
	addr, sid := mcpStart(t)
	resp := mcpRequest(t, addr, sid, map[string]any{"jsonrpc": "2.0", "id": "1", "method": "tools/list"})
	tools, _ := mcpResult(t, resp)["tools"].([]any)
	if len(tools) == 0 {
		t.Fatal("empty tools list")
	}
	// Value parameters are named arg0..argN.
	add := mcpFindTool(t, tools, "IntRpc_Add")
	schema, _ := add["inputSchema"].(map[string]any)
	props, _ := schema["properties"].(map[string]any)
	if props["arg0"] == nil || props["arg1"] == nil {
		t.Errorf("Add schema properties %v", props)
	}
	if arg0, _ := props["arg0"].(map[string]any); arg0["type"] != "integer" {
		t.Errorf("arg0 type %v", arg0)
	}
	required, _ := schema["required"].([]any)
	if len(required) != 2 {
		t.Errorf("required %v", required)
	}
	// Struct parameters expand to their JSON-tagged fields.
	hello := mcpFindTool(t, tools, "GreetRpc_Hello")
	schema, _ = hello["inputSchema"].(map[string]any)
	props, _ = schema["properties"].(map[string]any)
	if props["name"] == nil || props["age"] == nil {
		t.Errorf("Hello schema properties %v", props)
	}
	// Streaming methods are published too.
	mcpFindTool(t, tools, "StreamRpc_Count")
}

func TestMcpToolsCallUnary(t *testing.T) {
	addr, sid := mcpStart(t)
	resp := mcpRequest(t, addr, sid, map[string]any{
		"jsonrpc": "2.0", "id": "1", "method": "tools/call",
		"params": map[string]any{"name": "IntRpc_Add", "arguments": map[string]any{"arg0": 1, "arg1": 2}},
	})
	result := mcpResult(t, resp)
	if result["isError"] != false {
		t.Errorf("isError %v", result["isError"])
	}
	content, _ := result["content"].([]any)
	if len(content) != 1 {
		t.Fatalf("content %v", result["content"])
	}
	item, _ := content[0].(map[string]any)
	if item["type"] != "text" || item["text"] != "3" {
		t.Errorf("content item %v", item)
	}
}

func TestMcpToolsCallStruct(t *testing.T) {
	addr, sid := mcpStart(t)
	resp := mcpRequest(t, addr, sid, map[string]any{
		"jsonrpc": "2.0", "id": "1", "method": "tools/call",
		"params": map[string]any{"name": "GreetRpc_Hello", "arguments": map[string]any{"name": "bob", "age": 7}},
	})
	result := mcpResult(t, resp)
	content, _ := result["content"].([]any)
	item, _ := content[0].(map[string]any)
	text, _ := item["text"].(string)
	if !strings.Contains(text, "hi bob 7") {
		t.Errorf("tool text %q", text)
	}
}

func TestMcpToolsCallUnknown(t *testing.T) {
	addr, sid := mcpStart(t)
	resp := mcpRequest(t, addr, sid, map[string]any{
		"jsonrpc": "2.0", "id": "1", "method": "tools/call",
		"params": map[string]any{"name": "Missing_Tool", "arguments": map[string]any{}},
	})
	errObj, _ := resp["error"].(map[string]any)
	if errObj == nil || errObj["code"] != float64(common.MethodNotFound) {
		t.Errorf("unknown tool response %v", resp)
	}
	// A name without the Service_Method separator is an invalid parameter.
	resp = mcpRequest(t, addr, sid, map[string]any{
		"jsonrpc": "2.0", "id": "2", "method": "tools/call",
		"params": map[string]any{"name": "noseparator", "arguments": map[string]any{}},
	})
	errObj, _ = resp["error"].(map[string]any)
	if errObj == nil || errObj["code"] != float64(common.InvalidParams) {
		t.Errorf("bad tool name response %v", resp)
	}
}

func TestMcpToolErrorIsError(t *testing.T) {
	addr, sid := mcpStart(t)
	resp := mcpRequest(t, addr, sid, map[string]any{
		"jsonrpc": "2.0", "id": "1", "method": "tools/call",
		"params": map[string]any{"name": "StreamRpc_Fail", "arguments": map[string]any{"arg0": -1}},
	})
	result := mcpResult(t, resp)
	if result["isError"] != true {
		t.Errorf("expected tool error, got %v", result)
	}
}

func TestMcpToolsCallStreamingProgress(t *testing.T) {
	addr := shStart(t, nil, new(StreamRpc))
	sid, _, _ := shInitialize(t, addr, "2025-06-18")
	resp, b := shPost(t, addr, map[string]any{
		"jsonrpc": "2.0", "id": "42", "method": "tools/call",
		"params": map[string]any{
			"name":      "StreamRpc_Count",
			"arguments": map[string]any{"arg0": 3},
			"_meta":     map[string]any{"progressToken": "tok-1"},
		},
	}, shSession(sid))
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/event-stream") {
		t.Fatalf("expected SSE, got %q body %s", ct, b)
	}
	events := shParseSSE(t, b)
	if len(events) != 4 {
		t.Fatalf("expected 3 progress events + final, got %d: %s", len(events), b)
	}
	for i, ev := range events[:3] {
		if ev.data["method"] != "notifications/progress" {
			t.Fatalf("event %d not progress: %v", i, ev.data)
		}
		params, _ := ev.data["params"].(map[string]any)
		if params["progressToken"] != "tok-1" || params["progress"] != float64(i+1) {
			t.Errorf("progress event %d params %v", i, params)
		}
	}
	final := events[3].data
	if final["id"] != "42" {
		t.Errorf("final event %v", final)
	}
	result, _ := final["result"].(map[string]any)
	if result == nil || result["isError"] != false {
		t.Fatalf("final result %v", final)
	}
	content, _ := result["content"].([]any)
	item, _ := content[0].(map[string]any)
	if item["text"] != "null" {
		t.Errorf("final content text %v", item["text"])
	}
}

func TestMcpToolsCallStreamNoProgressToken(t *testing.T) {
	addr, sid := mcpStart(t)
	resp := mcpRequest(t, addr, sid, map[string]any{
		"jsonrpc": "2.0", "id": "1", "method": "tools/call",
		"params": map[string]any{"name": "StreamRpc_Count", "arguments": map[string]any{"arg0": 2}},
	})
	result := mcpResult(t, resp)
	if result["isError"] != false {
		t.Errorf("isError %v", result)
	}
}

func TestMcpNotificationsAccepted(t *testing.T) {
	addr, sid := mcpStart(t)
	resp, b := shPost(t, addr, map[string]any{
		"jsonrpc": "2.0", "method": "notifications/cancelled",
		"params": map[string]any{"requestId": "1"},
	}, shSession(sid))
	if resp.StatusCode != 202 {
		t.Errorf("notifications status %d body %s", resp.StatusCode, b)
	}
}

func TestMcpLegacySlashDispatch(t *testing.T) {
	// Old Service/method dispatch keeps working next to the MCP layer.
	addr, sid := mcpStart(t)
	resp := mcpRequest(t, addr, sid, map[string]any{
		"jsonrpc": "2.0", "id": "1", "method": "GreetRpc/Hello", "params": []any{map[string]any{"name": "amy", "age": 3}},
	})
	if resp["result"] != "hi amy 3" {
		t.Errorf("legacy dispatch result %v", resp["result"])
	}
}

func TestMcpToolsListStateless(t *testing.T) {
	// tools/list works without a session.
	addr := shStart(t, nil, new(IntRpc))
	out := shRequestJSON(t, addr, "", map[string]any{"jsonrpc": "2.0", "id": "1", "method": "tools/list"})
	res, _ := out["result"].(map[string]any)
	tools, _ := res["tools"].([]any)
	if len(tools) == 0 {
		t.Fatalf("stateless tools/list %s", mustJSON(out))
	}
}

func mustJSON(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}
