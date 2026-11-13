package common

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
)

const (
	JsonRpc = "2.0"
)

var RequiredFields = map[string]string{
	"id":      "id",
	"jsonrpc": "jsonrpc",
	"method":  "method",
	"params":  "params",
}

/**
 * @Description: Single request structure
 * @Field Method: Method name
 * @Field Params: Parameters
 * @Field Result: Result
 * @Field Error: Error pointer
 * @Field IsNotify: Whether it is a notification
 */
type SingleRequest struct {
	Method   string
	Params   any
	Result   any
	Error    *error
	IsNotify bool
}

/**
 * @Description: Request structure
 * @Field Id: Request ID
 * @Field JsonRpc: JSON-RPC version
 * @Field Method: Method name
 * @Field Params: Parameters
 */
type Request struct {
	Id      string `json:"id"`
	JsonRpc string `json:"jsonrpc"`
	Method  string `json:"method"`
	Params  any    `json:"params"`
}

/**
 * @Description: Notification request structure
 * @Field JsonRpc: JSON-RPC version
 * @Field Method: Method name
 * @Field Params: Parameters
 */
type NotifyRequest struct {
	JsonRpc string `json:"jsonrpc"`
	Method  string `json:"method"`
	Params  any    `json:"params"`
}

/**
 * @Description: Parse request method name
 * @Param method: Method name
 * @Return sName: Service name
 * @Return mName: Method name
 * @Return err: Error message
 */
func ParseRequestMethod(method string) (sName string, mName string, err error) {
	const msg = "rpc: method request ill-formed: %s; need x.y or x/y"
	// Tolerate an optional leading separator, as sent by some clients.
	method = strings.TrimLeft(method, "./")
	sep := "/"
	if !strings.Contains(method, sep) {
		sep = "."
	}
	sp := strings.LastIndex(method, sep)
	if sp <= 0 || sp == len(method)-1 {
		m := fmt.Sprintf(msg, method)
		Debug(m)
		return "", "", errors.New(m)
	}
	return method[:sp], method[sp+1:], nil
}

/**
 * @Description: Filter request body
 * @Param jsonMap: JSON map
 * @Return map[string]any: Filtered JSON map
 */
func FilterRequestBody(jsonMap map[string]any) map[string]any {
	for k := range jsonMap {
		if _, ok := RequiredFields[k]; !ok {
			delete(jsonMap, k)
		}
	}
	return jsonMap
}

/**
 * @Description: Parse single request body
 * @Param jsonMap: JSON map
 * @Return id: Request ID
 * @Return jsonrpc: JSON-RPC version
 * @Return method: Method name
 * @Return params: Parameters
 * @Return errCode: Error code
 */
func ParseSingleRequestBody(jsonMap map[string]any) (id any, jsonrpc string, method string, params any, errCode int) {
	jsonMap = FilterRequestBody(jsonMap)
	if _, ok := jsonMap["id"]; !ok {
		st := NotifyRequest{}
		err := GetStruct(jsonMap, &st)
		if err != nil {
			errCode = InvalidRequest
		}
		return nil, st.JsonRpc, st.Method, st.Params, errCode
	} else {
		st := Request{}
		err := GetStruct(jsonMap, &st)
		if err != nil {
			errCode = InvalidRequest
		}
		return st.Id, st.JsonRpc, st.Method, st.Params, errCode
	}
}

/**
 * @Description: Parse request body
 * @Param b: Request data
 * @Return any: Parsed data
 * @Return error: Error message
 */
func ParseRequestBody(b []byte) (any, error) {
	var err error
	var jsonData any
	err = json.Unmarshal(b, &jsonData)
	if err != nil {
		Debug(err)
	}
	return jsonData, err
}

/**
 * @Description: Get struct
 * @Param d: Data
 * @Param s: Struct pointer
 * @Return error: Error message
 */
func GetStruct(d any, s any) error {
	var (
		m string
		t reflect.Type
	)
	if s == nil || reflect.TypeOf(s).Kind() != reflect.Ptr {
		m = fmt.Sprintf("reflect: Elem of invalid type %v, need reflect.Ptr", s)
		Debug(m)
		return errors.New(m)
	}
	// A missing "params" member is valid: keep the zero value instead of panicking.
	if d == nil {
		return nil
	}
	t = reflect.TypeOf(s).Elem()
	var jsonMap = make(map[string]any)
	switch reflect.TypeOf(d).Kind() {
	case reflect.Map:
		if t.NumField() != len(d.(map[string]any)) {
			m = "json: The number of parameters does not match"
			Debug(m)
			return errors.New(m)
		}
		for k := 0; k < t.NumField(); k++ {
			lk := strings.ToLower(t.Field(k).Name)
			if _, ok := d.(map[string]any)[lk]; !ok {
				m = fmt.Sprintf("json: can not find field \"%s\"", lk)
				Debug(m)
				return errors.New(m)
			}
		}
		jsonMap = d.(map[string]any)
	case reflect.Slice:
		if t.NumField() != reflect.ValueOf(d).Len() {
			m = "json: The number of parameters does not match"
			Debug(m)
			return errors.New(m)
		}
		for k := 0; k < t.NumField(); k++ {
			jsonMap[t.Field(k).Name] = reflect.ValueOf(d).Index(k).Interface()
		}
	default:
		break
	}
	jsonStr, err := json.Marshal(jsonMap)
	if err != nil {
		Debug(err)
		return err
	}
	err = json.Unmarshal(jsonStr, s)
	if err != nil {
		Debug(err)
		return err
	}
	return nil
}

/**
 * @Description: Create request
 * @Param id: Request ID
 * @Param method: Method name
 * @Param params: Parameters
 * @Return any: Request structure
 */
func Rs(id any, method string, params any) any {
	var req any
	if id != nil {
		req = Request{id.(string), JsonRpc, method, params}
	} else {
		req = NotifyRequest{JsonRpc, method, params}
	}
	return req
}

/**
 * @Description: Create JSON request
 * @Param id: Request ID
 * @Param method: Method name
 * @Param params: Parameters
 * @Return []byte: JSON request data
 */
func JsonRs(id any, method string, params any) []byte {
	e, _ := json.Marshal(Rs(id, method, params))
	return e
}

/**
 * @Description: Create JSON batch request
 * @Param data: Request data list
 * @Return []byte: JSON batch request data
 */
func JsonBatchRs(data []any) []byte {
	e, _ := json.Marshal(data)
	return e
}
