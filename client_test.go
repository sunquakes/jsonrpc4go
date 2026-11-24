package jsonrpc4go

import (
	"errors"
	"testing"

	"github.com/sunquakes/jsonrpc4go/client"
)

type fakeResult struct {
	C int `json:"c"`
}

type fakeClient struct {
	result     any
	err        error
	gotMethod  string
	gotArgs    []any
	notifyArgs []any
	batchApp   []string
}

func (f *fakeClient) SetOptions(any)     {}
func (f *fakeClient) SetPoolOptions(any) {}

func (f *fakeClient) Call(method string, args ...any) (any, error) {
	f.gotMethod = method
	f.gotArgs = args
	return f.result, f.err
}

func (f *fakeClient) Notify(method string, args ...any) error {
	f.gotMethod = method
	f.notifyArgs = args
	return f.err
}

func (f *fakeClient) BatchAppend(method string, args ...any) {
	f.batchApp = append(f.batchApp, method)
}

func (f *fakeClient) BatchCall() ([]client.BatchResult, error) {
	return nil, nil
}

var _ client.Client = (*fakeClient)(nil)

func TestGenericCallInt(t *testing.T) {
	c := &fakeClient{result: float64(7)}
	result, err := Call[int](c, "Add", 1, 6)
	if err != nil {
		t.Fatalf("Call failed: %v", err)
	}
	if result != 7 {
		t.Errorf("result expected 7, got %d", result)
	}
	if c.gotMethod != "Add" || len(c.gotArgs) != 2 || c.gotArgs[0] != 1 {
		t.Errorf("inner Call received method=%q args=%v", c.gotMethod, c.gotArgs)
	}
}

func TestGenericCallStruct(t *testing.T) {
	c := &fakeClient{result: map[string]any{"c": float64(7)}}
	result, err := Call[fakeResult](c, "Add2", 1, 6)
	if err != nil {
		t.Fatalf("Call failed: %v", err)
	}
	if result.C != 7 {
		t.Errorf("result expected {7}, got %#v", result)
	}
}

func TestGenericCallNilResult(t *testing.T) {
	c := &fakeClient{result: nil}
	result, err := Call[int](c, "NoResult", 1)
	if err != nil {
		t.Fatalf("Call failed: %v", err)
	}
	if result != 0 {
		t.Errorf("result expected zero value, got %d", result)
	}
}

func TestGenericCallError(t *testing.T) {
	want := errors.New("Method not found")
	c := &fakeClient{err: want}
	result, err := Call[int](c, "Nope", 1)
	if err != want {
		t.Errorf("error expected %v, got %v", want, err)
	}
	if result != 0 {
		t.Errorf("result expected zero value on error, got %d", result)
	}
}

func TestGenericCallDecodeMismatch(t *testing.T) {
	c := &fakeClient{result: "not a number"}
	if _, err := Call[int](c, "Add", 1, 2); err == nil {
		t.Errorf("Call expected an error when the result cannot decode into int")
	}
}

func TestFakeNotifyPassthrough(t *testing.T) {
	c := &fakeClient{}
	if err := c.Notify("Add2", 1, 6); err != nil {
		t.Fatalf("Notify failed: %v", err)
	}
	if c.gotMethod != "Add2" || len(c.notifyArgs) != 2 {
		t.Errorf("Notify received method=%q args=%v", c.gotMethod, c.notifyArgs)
	}
}
