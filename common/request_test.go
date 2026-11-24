package common

import (
	"reflect"
	"strings"
	"testing"
)

func TestDecodeArgsNoTypes(t *testing.T) {
	vals, err := DecodeArgs(nil, nil)
	if err != nil || len(vals) != 0 {
		t.Errorf("zero-arg decode expected empty values without error, got len=%d err=%v", len(vals), err)
	}
	vals, err = DecodeArgs([]any{1}, nil)
	if err != nil || len(vals) != 0 {
		t.Errorf("extra params for zero-arg method expected ignored, got len=%d err=%v", len(vals), err)
	}
}

func TestDecodeArgsSuccess(t *testing.T) {
	types := []reflect.Type{reflect.TypeFor[int](), reflect.TypeFor[string](), reflect.TypeFor[unitParams]()}
	vals, err := DecodeArgs([]any{2.0, "x", map[string]any{"a": 5.0}}, types)
	if err != nil {
		t.Fatalf("DecodeArgs failed: %v", err)
	}
	if len(vals) != 3 {
		t.Fatalf("expected 3 args, got %d", len(vals))
	}
	if got := vals[0].Interface(); got != 2 {
		t.Errorf("arg 0 expected int(2), got %#v", got)
	}
	if got := vals[1].Interface(); got != "x" {
		t.Errorf("arg 1 expected \"x\", got %#v", got)
	}
	if got := vals[2].Interface().(unitParams); got.A != 5 {
		t.Errorf("arg 2 expected unitParams{5}, got %#v", got)
	}
}

func TestDecodeArgsErrors(t *testing.T) {
	types := []reflect.Type{reflect.TypeFor[int]()}
	cases := []struct {
		name   string
		params any
	}{
		{"missing params", nil},
		{"named object params", map[string]any{"a": 1.0}},
		{"too few args", []any{}},
		{"too many args", []any{1.0, 2.0}},
		{"wrong element type", []any{"x"}},
	}
	for _, c := range cases {
		if _, err := DecodeArgs(c.params, types); err == nil {
			t.Errorf("%s expected an error", c.name)
		}
	}
}

func TestJsonRsPositionalParams(t *testing.T) {
	b := JsonRs("1", "IntRpc/Add", []any{1, 6})
	s := string(b)
	if !strings.Contains(s, `"params":[1,6]`) {
		t.Errorf("request expected positional params, got %s", s)
	}
	if !strings.Contains(s, `"id":"1"`) {
		t.Errorf("request expected id, got %s", s)
	}
}

func TestJsonRsNotify(t *testing.T) {
	b := JsonRs(nil, "IntRpc/Add", []any{})
	s := string(b)
	if strings.Contains(s, `"id"`) {
		t.Errorf("notify request should not contain id, got %s", s)
	}
	if !strings.Contains(s, `"params":[]`) {
		t.Errorf("notify request expected empty positional params, got %s", s)
	}
}

func TestSingleRequestArgs(t *testing.T) {
	r := SingleRequest{Method: "Add", Args: []any{1, 2}}
	if len(r.Args) != 2 || r.Args[0] != 1 {
		t.Errorf("SingleRequest args not stored: %#v", r)
	}
}
