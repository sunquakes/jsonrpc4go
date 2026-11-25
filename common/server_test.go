package common

import (
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	"golang.org/x/time/rate"
)

type unitParams struct {
	A int `json:"a"`
}

type unitRpc struct{}

func (u *unitRpc) Add(a, b int) int { return a + b }

func (u *unitRpc) Sum(a, b int) (int, error) { return a + b, nil }

func (u *unitRpc) Fail(a int) (int, error) { return 0, errors.New("boom") }

func (u *unitRpc) NoResult(a int) {}

func (u *unitRpc) OnlyError(a int) error { return nil }

func (u *unitRpc) Hello() string { return "hi" }

func (u *unitRpc) Echo(p unitParams, n int) unitParams { return unitParams{A: p.A + n} }

// The methods below must be rejected by RegisterMethod.

func (u *unitRpc) PtrArg(a *int) int { return *a }

func (u *unitRpc) Variadic(a ...int) int { return 0 }

func (u *unitRpc) TooManyOut() (int, int, error) { return 0, 0, nil }

func (u *unitRpc) BadErrorPos() (error, int) { return nil, 0 } //nolint:staticcheck // must stay invalid: RegisterMethod rejects a non-trailing error

func newUnitServer(t *testing.T) *Server {
	t.Helper()
	svr := &Server{}
	if err := svr.Register(new(unitRpc)); err != nil {
		t.Fatalf("Register failed: %v", err)
	}
	return svr
}

func callHandler(t *testing.T, svr *Server, req string) map[string]any {
	t.Helper()
	data := svr.Handler([]byte(req))
	var resp map[string]any
	if err := json.Unmarshal(data, &resp); err != nil {
		t.Fatalf("response %s is not a JSON object: %v", data, err)
	}
	return resp
}

func respErrorCode(t *testing.T, resp map[string]any) int {
	t.Helper()
	errObj, ok := resp["error"].(map[string]any)
	if !ok {
		t.Fatalf("response %v has no error object", resp)
	}
	code, ok := errObj["code"].(float64)
	if !ok {
		t.Fatalf("error code is not a number: %v", errObj["code"])
	}
	return int(code)
}

func TestRegisterMethodsSignatureRules(t *testing.T) {
	mm := RegisterMethods(reflect.TypeOf(&unitRpc{}))
	valid := []string{"Add", "Sum", "Fail", "NoResult", "OnlyError", "Hello", "Echo"}
	for _, name := range valid {
		if _, ok := mm[name]; !ok {
			t.Errorf("method %q should be registered", name)
		}
	}
	invalid := []string{"PtrArg", "Variadic", "TooManyOut", "BadErrorPos"}
	for _, name := range invalid {
		if _, ok := mm[name]; ok {
			t.Errorf("method %q should not be registered", name)
		}
	}
}

func TestRegisterMethodMetadata(t *testing.T) {
	mm := RegisterMethods(reflect.TypeOf(&unitRpc{}))

	add := mm["Add"]
	if len(add.ParamsTypes) != 2 || add.ParamsTypes[0] != reflect.TypeFor[int]() || add.ParamsTypes[1] != reflect.TypeFor[int]() {
		t.Errorf("Add ParamsTypes expected [int int], got %v", add.ParamsTypes)
	}
	if add.ResultType != reflect.TypeFor[int]() || add.HasError {
		t.Errorf("Add expected ResultType int without error, got %v hasError=%v", add.ResultType, add.HasError)
	}

	fail := mm["Fail"]
	if fail.ResultType != reflect.TypeFor[int]() || !fail.HasError {
		t.Errorf("Fail expected ResultType int with error, got %v hasError=%v", fail.ResultType, fail.HasError)
	}

	noResult := mm["NoResult"]
	if noResult.ResultType != nil || noResult.HasError {
		t.Errorf("NoResult expected no outputs, got %v hasError=%v", noResult.ResultType, noResult.HasError)
	}

	onlyError := mm["OnlyError"]
	if onlyError.ResultType != nil || !onlyError.HasError {
		t.Errorf("OnlyError expected error-only outputs, got %v hasError=%v", onlyError.ResultType, onlyError.HasError)
	}

	echo := mm["Echo"]
	if len(echo.ParamsTypes) != 2 || echo.ParamsTypes[0] != reflect.TypeFor[unitParams]() || echo.ParamsTypes[1] != reflect.TypeFor[int]() {
		t.Errorf("Echo ParamsTypes expected [unitParams int], got %v", echo.ParamsTypes)
	}
	if echo.ResultType != reflect.TypeFor[unitParams]() {
		t.Errorf("Echo ResultType expected unitParams, got %v", echo.ResultType)
	}
}

func TestHandlerCall(t *testing.T) {
	svr := newUnitServer(t)
	resp := callHandler(t, svr, `{"id":"1","jsonrpc":"2.0","method":"unitRpc/Add","params":[1,2]}`)
	if v, ok := resp["result"].(float64); !ok || v != 3 {
		t.Errorf("result expected 3, got %v", resp["result"])
	}
	if resp["id"] != "1" {
		t.Errorf("id expected \"1\", got %v", resp["id"])
	}
}

func TestHandlerCallResultAndError(t *testing.T) {
	svr := newUnitServer(t)
	resp := callHandler(t, svr, `{"id":"1","jsonrpc":"2.0","method":"unitRpc/Sum","params":[1,2]}`)
	if v, ok := resp["result"].(float64); !ok || v != 3 {
		t.Errorf("result expected 3, got %v", resp["result"])
	}
}

func TestHandlerMethodErrorReturnsInternalError(t *testing.T) {
	svr := newUnitServer(t)
	resp := callHandler(t, svr, `{"id":"1","jsonrpc":"2.0","method":"unitRpc/Fail","params":[1]}`)
	if code := respErrorCode(t, resp); code != InternalError {
		t.Errorf("error code expected %d, got %d", InternalError, code)
	}
}

func TestHandlerNoResultMethod(t *testing.T) {
	svr := newUnitServer(t)
	resp := callHandler(t, svr, `{"id":"1","jsonrpc":"2.0","method":"unitRpc/NoResult","params":[1]}`)
	if _, ok := resp["error"]; ok {
		t.Errorf("unexpected error for NoResult: %v", resp["error"])
	}
	if v, ok := resp["result"]; !ok || v != nil {
		t.Errorf("result expected null, got %v (present=%v)", v, ok)
	}
}

func TestHandlerOnlyErrorMethod(t *testing.T) {
	svr := newUnitServer(t)
	resp := callHandler(t, svr, `{"id":"1","jsonrpc":"2.0","method":"unitRpc/OnlyError","params":[1]}`)
	if _, ok := resp["error"]; ok {
		t.Errorf("unexpected error for OnlyError: %v", resp["error"])
	}
}

func TestHandlerNoArgsMethod(t *testing.T) {
	svr := newUnitServer(t)
	resp := callHandler(t, svr, `{"id":"1","jsonrpc":"2.0","method":"unitRpc/Hello","params":[]}`)
	if v, ok := resp["result"].(string); !ok || v != "hi" {
		t.Errorf("result expected \"hi\", got %v", resp["result"])
	}
}

func TestHandlerStructArg(t *testing.T) {
	svr := newUnitServer(t)
	resp := callHandler(t, svr, `{"id":"1","jsonrpc":"2.0","method":"unitRpc/Echo","params":[{"a":5},2]}`)
	result, ok := resp["result"].(map[string]any)
	if !ok {
		t.Fatalf("result expected an object, got %v", resp["result"])
	}
	if v, ok := result["a"].(float64); !ok || v != 7 {
		t.Errorf("result.a expected 7, got %v", result["a"])
	}
}

func TestHandlerInvalidParams(t *testing.T) {
	svr := newUnitServer(t)
	cases := []string{
		`{"id":"1","jsonrpc":"2.0","method":"unitRpc/Add","params":[1]}`,
		`{"id":"1","jsonrpc":"2.0","method":"unitRpc/Add","params":{"a":1,"b":2}}`,
		`{"id":"1","jsonrpc":"2.0","method":"unitRpc/Add","params":[1,"x"]}`,
		`{"id":"1","jsonrpc":"2.0","method":"unitRpc/Add","params":[]}`,
	}
	for _, req := range cases {
		resp := callHandler(t, svr, req)
		if code := respErrorCode(t, resp); code != InvalidParams {
			t.Errorf("request %s expected InvalidParams, got code %d", req, code)
		}
	}
}

func TestHandlerMethodNotFound(t *testing.T) {
	svr := newUnitServer(t)
	cases := []string{
		`{"id":"1","jsonrpc":"2.0","method":"unitRpc/PtrArg","params":[1]}`,
		`{"id":"1","jsonrpc":"2.0","method":"unitRpc/Nope","params":[1]}`,
		`{"id":"1","jsonrpc":"2.0","method":"Other/Add","params":[1,2]}`,
	}
	for _, req := range cases {
		resp := callHandler(t, svr, req)
		if code := respErrorCode(t, resp); code != MethodNotFound {
			t.Errorf("request %s expected MethodNotFound, got code %d", req, code)
		}
	}
}

func TestHandlerNotify(t *testing.T) {
	svr := newUnitServer(t)
	resp := callHandler(t, svr, `{"jsonrpc":"2.0","method":"unitRpc/Add","params":[1,2]}`)
	if _, ok := resp["id"]; ok {
		t.Errorf("notify response should not contain id: %v", resp)
	}
	if v, ok := resp["result"].(float64); !ok || v != 3 {
		t.Errorf("result expected 3, got %v", resp["result"])
	}
}

func TestHandlerBatch(t *testing.T) {
	svr := newUnitServer(t)
	data := svr.Handler([]byte(`[{"id":"1","jsonrpc":"2.0","method":"unitRpc/Add","params":[1,2]},{"id":"2","jsonrpc":"2.0","method":"unitRpc/Nope","params":[]}]`))
	var resp []any
	if err := json.Unmarshal(data, &resp); err != nil {
		t.Fatalf("batch response is not an array: %v", err)
	}
	if len(resp) != 2 {
		t.Fatalf("batch response length expected 2, got %d", len(resp))
	}
	first := resp[0].(map[string]any)
	if v, ok := first["result"].(float64); !ok || v != 3 {
		t.Errorf("batch item 0 result expected 3, got %v", first["result"])
	}
	second := resp[1].(map[string]any)
	if code := respErrorCode(t, second); code != MethodNotFound {
		t.Errorf("batch item 1 expected MethodNotFound, got code %d", code)
	}
}

func TestHandlerEmptyBatchAndBadJson(t *testing.T) {
	svr := newUnitServer(t)
	resp := callHandler(t, svr, `[]`)
	if code := respErrorCode(t, resp); code != InvalidRequest {
		t.Errorf("empty batch expected InvalidRequest, got code %d", code)
	}
	resp = callHandler(t, svr, `{`)
	if code := respErrorCode(t, resp); code != ParseError {
		t.Errorf("malformed JSON expected ParseError, got code %d", code)
	}
}

func TestHandlerRateLimit(t *testing.T) {
	svr := newUnitServer(t)
	svr.RateLimiter = rate.NewLimiter(0, 0)
	resp := callHandler(t, svr, `{"id":"1","jsonrpc":"2.0","method":"unitRpc/Add","params":[1,2]}`)
	if code := respErrorCode(t, resp); code != CustomError {
		t.Errorf("rate limited request expected CustomError, got code %d", code)
	}
	errObj := resp["error"].(map[string]any)
	if errObj["message"] != "Too many requests" {
		t.Errorf("message expected \"Too many requests\", got %v", errObj["message"])
	}
}

func TestHandlerHooks(t *testing.T) {
	svr := newUnitServer(t)
	var beforeParams any
	svr.Hooks.BeforeFunc = func(id any, method string, params any) error {
		beforeParams = params
		if method != "Add" {
			t.Errorf("hook method expected Add, got %s", method)
		}
		return nil
	}
	var afterResult any
	svr.Hooks.AfterFunc = func(id any, method string, result any) error {
		afterResult = result
		return nil
	}
	resp := callHandler(t, svr, `{"id":"1","jsonrpc":"2.0","method":"unitRpc/Add","params":[1,2]}`)
	if v, ok := resp["result"].(float64); !ok || v != 3 {
		t.Errorf("result expected 3, got %v", resp["result"])
	}
	args, ok := beforeParams.([]any)
	if !ok || len(args) != 2 || args[0] != 1 || args[1] != 2 {
		t.Errorf("before hook params expected []any{1, 2}, got %#v", beforeParams)
	}
	if afterResult != 3 {
		t.Errorf("after hook result expected int(3), got %#v", afterResult)
	}
}

func TestHandlerBeforeHookError(t *testing.T) {
	svr := newUnitServer(t)
	svr.Hooks.BeforeFunc = func(id any, method string, params any) error {
		return errors.New("hook error")
	}
	resp := callHandler(t, svr, `{"id":"1","jsonrpc":"2.0","method":"unitRpc/Add","params":[1,2]}`)
	if code := respErrorCode(t, resp); code != CustomError {
		t.Errorf("before hook error expected CustomError, got code %d", code)
	}
	errObj := resp["error"].(map[string]any)
	if errObj["message"] != "hook error" {
		t.Errorf("message expected \"hook error\", got %v", errObj["message"])
	}
}

func TestRegisterDuplicate(t *testing.T) {
	svr := newUnitServer(t)
	if err := svr.Register(new(unitRpc)); err == nil {
		t.Errorf("duplicate Register expected an error")
	}
}
