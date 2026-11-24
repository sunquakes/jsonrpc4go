package common

import (
	"testing"
)

func TestGetResultSuccess(t *testing.T) {
	result, err := GetResult([]byte(`{"id":"1","jsonrpc":"2.0","result":7}`))
	if err != nil {
		t.Fatalf("GetResult failed: %v", err)
	}
	if v, ok := result.(float64); !ok || v != 7 {
		t.Errorf("result expected 7, got %#v", result)
	}
}

func TestGetResultError(t *testing.T) {
	_, err := GetResult([]byte(`{"id":"1","jsonrpc":"2.0","error":{"code":-32601,"message":"Method not found","data":null}}`))
	if err == nil || err.Error() != CodeMap[MethodNotFound] {
		t.Errorf("error expected %q, got %v", CodeMap[MethodNotFound], err)
	}
}

func TestGetResultNilResult(t *testing.T) {
	result, err := GetResult([]byte(`{"id":"1","jsonrpc":"2.0","result":null}`))
	if err != nil {
		t.Fatalf("GetResult failed: %v", err)
	}
	if result != nil {
		t.Errorf("result expected nil, got %#v", result)
	}
}

func TestGetResultRejectsBatch(t *testing.T) {
	if _, err := GetResult([]byte(`[{"id":"1","jsonrpc":"2.0","result":7}]`)); err == nil {
		t.Errorf("GetResult expected an error for batch response")
	}
}

func TestGetResultBadJson(t *testing.T) {
	if _, err := GetResult([]byte(`{`)); err == nil {
		t.Errorf("GetResult expected an error for malformed JSON")
	}
}

func TestGetBatchResult(t *testing.T) {
	items, err := GetBatchResult([]byte(`[{"id":"1","jsonrpc":"2.0","result":5},{"id":"2","jsonrpc":"2.0","error":{"code":-32601,"message":"Method not found","data":null}}]`))
	if err != nil {
		t.Fatalf("GetBatchResult failed: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
	if items[0].Err != nil {
		t.Errorf("item 0 expected no error, got %v", items[0].Err)
	}
	if v, ok := items[0].Result.(float64); !ok || v != 5 {
		t.Errorf("item 0 result expected 5, got %#v", items[0].Result)
	}
	if items[1].Err == nil || items[1].Err.Error() != CodeMap[MethodNotFound] {
		t.Errorf("item 1 expected MethodNotFound error, got %v", items[1].Err)
	}
	if items[1].Result != nil {
		t.Errorf("item 1 result expected nil, got %#v", items[1].Result)
	}
}

func TestGetBatchResultRejectsNonArray(t *testing.T) {
	if _, err := GetBatchResult([]byte(`{"id":"1","jsonrpc":"2.0","result":5}`)); err == nil {
		t.Errorf("GetBatchResult expected an error for non-array response")
	}
}

func TestGetBatchResultInvalidItem(t *testing.T) {
	items, err := GetBatchResult([]byte(`[7]`))
	if err != nil {
		t.Fatalf("GetBatchResult failed: %v", err)
	}
	if len(items) != 1 || items[0].Err == nil {
		t.Errorf("invalid batch item expected an error, got %#v", items)
	}
}

func TestGetSingleResponseMissingResult(t *testing.T) {
	jsonData := map[string]any{"id": "1", "jsonrpc": "2.0"}
	result, err := GetSingleResponse(jsonData)
	if err != nil {
		t.Fatalf("GetSingleResponse failed: %v", err)
	}
	if result != nil {
		t.Errorf("result expected nil, got %#v", result)
	}
}
