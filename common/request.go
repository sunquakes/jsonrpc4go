package common

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/mitchellh/mapstructure"
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

type SingleRequest struct {
	Method   string
	Params   any
	Result   any
	Error    *error
	IsNotify bool
}

type Request struct {
	Id      string `json:"id"`
	JsonRpc string `json:"jsonrpc"`
	Method  string `json:"method"`
	Params  any    `json:"params"`
}

type NotifyRequest struct {
	JsonRpc string `json:"jsonrpc"`
	Method  string `json:"method"`
	Params  any    `json:"params"`
}

func ParseRequestMethod(method string) (sName string, mName string, err error) {
	var (
		m  string
		sp int
	)
	first := method[0:1]
	if first == "." || first == "/" {
		method = method[1:]
	}
	if strings.Count(method, ".") != 1 && strings.Count(method, "/") != 1 {
		m = fmt.Sprintf("rpc: method request ill-formed: %s; need x.y or x/y", method)
		Debug(m)
		return sName, mName, errors.New(m)
	}
	if strings.Count(method, ".") == 1 {
		sp = strings.LastIndex(method, ".")
		if sp < 0 {
			m = fmt.Sprintf("rpc: method request ill-formed: %s; need x.y or x/y", method)
			return sName, mName, errors.New(m)
		}

		sName = method[:sp]
		mName = method[sp+1:]
	} else if strings.Count(method, "/") == 1 {
		sp = strings.LastIndex(method, "/")
		if sp < 0 {
			m = fmt.Sprintf("rpc: method request ill-formed: %s; need x.y or x/y", method)
			return sName, mName, errors.New(m)
		}

		sName = method[:sp]
		mName = method[sp+1:]
	}
	return sName, mName, err
}

func FilterRequestBody(jsonMap map[string]any) map[string]any {
	for k := range jsonMap {
		if _, ok := RequiredFields[k]; !ok {
			delete(jsonMap, k)
		}
	}
	return jsonMap
}

func ParseSingleRequestBody(jsonMap map[string]any) (id any, jsonrpc string, method string, params any, errCode int) {
	jsonMap = FilterRequestBody(jsonMap)
	if _, ok := jsonMap["id"]; ok != true {
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

func ParseRequestBody(b []byte) (any, error) {
	var err error
	var jsonData any
	err = json.Unmarshal(b, &jsonData)
	if err != nil {
		Debug(err)
	}
	return jsonData, err
}

func GetStruct(d any, s any) error {
	var (
		m string
		t reflect.Type
	)
	if reflect.TypeOf(s).Kind() != reflect.Ptr {
		m = fmt.Sprintf("reflect: Elem of invalid type %s, need reflect.Ptr", reflect.TypeOf(s))
		Debug(m)
		return errors.New(m)
	}
	t = reflect.TypeOf(s).Elem()
	var jsonMap = make(map[string]any)
	switch reflect.TypeOf(d).Kind() {
	case reflect.Map:
		if t.NumField() != len(d.(map[string]any)) {
			m = fmt.Sprintf("json: The number of parameters does not match")
			Debug(m)
			return errors.New(m)
		}
		for k := 0; k < t.NumField(); k++ {
			lk := strings.ToLower(t.Field(k).Name)
			if _, ok := d.(map[string]any)[lk]; ok != true {
				m = fmt.Sprintf("json: can not find field \"%s\"", lk)
				Debug(m)
				return errors.New(m)
			}
		}
		jsonMap = d.(map[string]any)
		break
	case reflect.Slice:
		if t.NumField() != reflect.ValueOf(d).Len() {
			m = fmt.Sprintf("json: The number of parameters does not match")
			Debug(m)
			return errors.New(m)
		}
		for k := 0; k < t.NumField(); k++ {
			jsonMap[t.Field(k).Name] = reflect.ValueOf(d).Index(k).Interface()
		}
		break
	default:
		break
	}
	if err := mapstructure.Decode(jsonMap, s); err != nil {
		Debug(err)
		return err
	}
	return nil
}

func Rs(id any, method string, params any) any {
	var req any
	if id != nil {
		req = Request{id.(string), JsonRpc, method, params}
	} else {
		req = NotifyRequest{JsonRpc, method, params}
	}
	return req
}

func JsonRs(id any, method string, params any) []byte {
	e, _ := json.Marshal(Rs(id, method, params))
	return e
}

func JsonBatchRs(data []any) []byte {
	e, _ := json.Marshal(data)
	return e
}
