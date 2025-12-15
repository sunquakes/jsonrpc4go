package common

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"

	"golang.org/x/time/rate"
)

type Method struct {
	Name       string
	ParamsType reflect.Type
	ResultType reflect.Type
	Method     reflect.Method
}

type Service struct {
	Name string
	V    reflect.Value
	T    reflect.Type
	Mm   map[string]*Method
}

type Server struct {
	Sm          sync.Map
	Hooks       Hooks
	RateLimiter *rate.Limiter
}

type Hooks struct {
	BeforeFunc func(id any, method string, params any) error
	AfterFunc  func(id any, method string, result any) error
}

func (svr *Server) Register(s any) error {
	svc := new(Service)
	svc.V = reflect.ValueOf(s)
	svc.T = reflect.TypeOf(s)
	sname := reflect.Indirect(svc.V).Type().Name()
	svc.Name = sname
	svc.Mm = RegisterMethods(svc.T)
	if _, err := svr.Sm.LoadOrStore(sname, svc); err {
		return errors.New("rpc: service already defined: " + sname)
	}
	return nil
}

func RegisterMethods(s reflect.Type) map[string]*Method {
	mm := make(map[string]*Method)
	for m := range s.NumMethod() {
		rm := s.Method(m)
		if mt := RegisterMethod(rm); mt != nil {
			mm[rm.Name] = mt
		}
	}
	return mm
}

func RegisterMethod(rm reflect.Method) *Method {
	var (
		msg string
	)
	rmt := rm.Type
	rmn := rm.Name
	if rm.Type.NumIn() != 3 {
		msg = fmt.Sprintf("RegisterMethod: method %q has %d input parameters; needs exactly three", rmn, rmt.NumIn())
		Debug(msg)
		return nil
	}
	p := rmt.In(1)
	if p.Kind() != reflect.Ptr {
		msg = fmt.Sprintf("RegisterMethod: Params type of method %q is not a reflect.Ptr:%q", rmn, p)
		Debug(msg)
		return nil
	}
	r := rmt.In(2)
	if r.Kind() != reflect.Ptr {
		msg = fmt.Sprintf("RegisterMethod: Result type of method %q is not a reflect.Ptr:%q", rmn, r)
		Debug(msg)
		return nil
	}

	if rm.Type.NumOut() != 1 {
		msg = fmt.Sprintf("RegisterMethod: Method %q has %d output parameters; needs exactly one", rmn, rmt.NumOut())
		Debug(msg)
		return nil
	}
	ret := rmt.Out(0)
	if ret != reflect.TypeOf((*error)(nil)).Elem() {
		msg = fmt.Sprintf("RegisterMethod: Return type of method %q is not a must be error:%q", rmn, ret)
		Debug(msg)
		return nil
	}
	m := &Method{rmn, p, r, rm}
	return m
}

func (svr *Server) Handler(b []byte) []byte {
	data, err := ParseRequestBody(b)
	if err != nil {
		return jsonE(nil, JsonRpc, ParseError)
	}
	var res any
	if reflect.ValueOf(data).Kind() == reflect.Slice {
		var resList []any
		for _, v := range data.([]any) {
			r := svr.SingleHandler(v.(map[string]any))
			resList = append(resList, r)
		}
		res = resList
	} else if reflect.ValueOf(data).Kind() == reflect.Map {
		r := svr.SingleHandler(data.(map[string]any))
		res = r
	} else {
		return jsonE(nil, JsonRpc, InvalidRequest)
	}

	response, _ := json.Marshal(res)
	return response
}

func (svr *Server) SingleHandler(jsonMap map[string]any) any {
	id, jsonRpc, method, paramsData, errCode := ParseSingleRequestBody(jsonMap)
	if errCode != WithoutError {
		return E(id, jsonRpc, errCode)
	}

	if svr.RateLimiter != nil && !svr.RateLimiter.Allow() {
		return CE(id, JsonRpc, "Too many requests")
	}

	//if jsonRpc != JsonRpc {
	//	return E(id, jsonRpc, InvalidRequest)
	//}
	sName, mName, err := ParseRequestMethod(method)
	if err != nil {
		return E(id, jsonRpc, MethodNotFound)
	}
	s, ok := svr.Sm.Load(sName)
	if !ok {
		sName = lineToHump(sName) // support HelloWorld and hello_world
		s, ok = svr.Sm.Load(sName)
		if !ok {
			return E(id, jsonRpc, MethodNotFound)
		}
	}
	m, ok := s.(*Service).Mm[mName]
	if !ok {
		return E(id, jsonRpc, MethodNotFound)
	}
	params := reflect.New(m.ParamsType.Elem())
	pv := params.Interface()
	err = GetStruct(paramsData, pv)
	if err != nil {
		return E(id, jsonRpc, InvalidParams)
	}
	result := reflect.New(m.ResultType.Elem())

	// before
	err = svr.Before(id, mName, params.Elem().Interface())
	if err != nil {
		return CE(id, jsonRpc, err.Error())
	}

	r := m.Method.Func.Call([]reflect.Value{s.(*Service).V, params, result})

	if i := r[0].Interface(); i != nil {
		Debug(i.(error))
		return E(id, jsonRpc, InternalError)
	}
	// after
	err = svr.After(id, mName, result.Elem().Interface())
	if err != nil {
		return CE(id, jsonRpc, err.Error())
	}

	return S(id, jsonRpc, result.Elem().Interface())
}

func (svr *Server) Before(id any, mName string, params any) error {
	if svr.Hooks.BeforeFunc != nil {
		err := svr.Hooks.BeforeFunc(id, mName, params)
		if err != nil {
			return err
		}
	}
	return nil
}

func (svr *Server) After(id any, mName string, result any) error {
	if svr.Hooks.AfterFunc != nil {
		err := svr.Hooks.AfterFunc(id, mName, result)
		if err != nil {
			return err
		}
	}
	return nil
}

func lineToHump(in string) string {
	s := strings.Split(in, "_")
	for k, v := range s {
		s[k] = Capitalize(v)
	}
	return strings.Join(s, "")
}

func Capitalize(str string) string {
	var upperStr string
	vv := []rune(str)
	for i := 0; i < len(vv); i++ {
		if i == 0 {
			if vv[i] >= 97 && vv[i] <= 122 {
				vv[i] -= 32
				upperStr += string(vv[i])
			} else {
				return str
			}
		} else {
			upperStr += string(vv[i])
		}
	}
	return upperStr
}
