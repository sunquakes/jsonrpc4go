package common

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"reflect"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

/*
 * Method represents a JSON-RPC method definition.
 *
 * Fields:
 *   Name        string          - Method name
 *   ParamsTypes []reflect.Type  - Types of the method arguments (without the receiver)
 *   ResultType  reflect.Type    - Type of the method result; nil when the method returns no result
 *   HasError    bool            - Whether the method returns an error as its last output
 *   Method      reflect.Method  - Reflect method object
 */
type Method struct {
	Name        string
	ParamsTypes []reflect.Type
	ResultType  reflect.Type
	HasError    bool
	Method      reflect.Method
}

/*
 * Service represents a JSON-RPC service containing multiple methods.
 *
 * Fields:
 *   Name string           - Service name
 *   V    reflect.Value    - Reflect value of the service instance
 *   T    reflect.Type     - Reflect type of the service instance
 *   Mm   map[string]*Method - Map of method names to Method objects
 */
type Service struct {
	Name string
	V    reflect.Value
	T    reflect.Type
	Mm   map[string]*Method
}

/*
 * Server represents a JSON-RPC server that handles requests and dispatches them to services.
 *
 * Fields:
 *   Sm          sync.Map      - Map of service names to Service objects
 *   Hooks       Hooks         - Before and after function hooks
 *   RateLimiter *rate.Limiter - Rate limiter for request throttling
 */
type Server struct {
	Sm          sync.Map
	Hooks       Hooks
	RateLimiter *rate.Limiter
}

/*
 * Hooks contains callback functions that are executed before and after method calls.
 *
 * Fields:
 *   BeforeFunc func(id any, method string, params any) error - Function called before processing a request; params is a []any of the decoded positional arguments
 *   AfterFunc  func(id any, method string, result any) error - Function called after processing a request; result is the method's returned value
 *   StartFunc  func() - Function called after server starts
 */
type Hooks struct {
	BeforeFunc func(id any, method string, params any) error
	AfterFunc  func(id any, method string, result any) error
	StartFunc  func()
}

/*
 * Register registers a service with the server.
 *
 * Parameters:
 *   s any - Service instance to register
 *
 * Returns:
 *   error - Error if the service is already registered
 */
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

/*
 * RegisterMethods registers all public methods of a service type.
 *
 * Parameters:
 *   s reflect.Type - Reflect type of the service
 *
 * Returns:
 *   map[string]*Method - Map of method names to Method objects
 */
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

/*
 * RegisterMethod registers a single method if it conforms to the JSON-RPC
 * method signature.
 *
 * Accepted signatures: the receiver plus any number of non-pointer value
 * arguments, returning an optional result and an optional trailing error,
 * e.g. func (i *IntRpc) Add(a, b int) int or func (i *IntRpc) Add(a, b int) (int, error).
 *
 * Parameters:
 *   rm reflect.Method - Reflect method to register
 *
 * Returns:
 *   *Method - Method object if registration succeeds, nil otherwise
 */
func RegisterMethod(rm reflect.Method) *Method {
	var (
		msg string
	)
	rmt := rm.Type
	rmn := rm.Name
	if rmt.IsVariadic() {
		msg = fmt.Sprintf("RegisterMethod: method %q is variadic; variadic methods are not supported", rmn)
		Debug(msg)
		return nil
	}
	paramsTypes := make([]reflect.Type, 0, rmt.NumIn()-1)
	for k := 1; k < rmt.NumIn(); k++ {
		p := rmt.In(k)
		if p.Kind() == reflect.Pointer {
			msg = fmt.Sprintf("RegisterMethod: Params type of method %q is a reflect.Ptr:%q; method arguments must be value types", rmn, p)
			Debug(msg)
			return nil
		}
		paramsTypes = append(paramsTypes, p)
	}
	var (
		resultType reflect.Type
		hasError   bool
	)
	switch rmt.NumOut() {
	case 0:
	case 1:
		if rmt.Out(0) == reflect.TypeFor[error]() {
			hasError = true
		} else {
			resultType = rmt.Out(0)
		}
	case 2:
		if rmt.Out(1) != reflect.TypeFor[error]() {
			msg = fmt.Sprintf("RegisterMethod: Second return value of method %q is not an error:%q", rmn, rmt.Out(1))
			Debug(msg)
			return nil
		}
		resultType = rmt.Out(0)
		hasError = true
	default:
		msg = fmt.Sprintf("RegisterMethod: Method %q has %d output parameters; needs at most two", rmn, rmt.NumOut())
		Debug(msg)
		return nil
	}
	m := &Method{rmn, paramsTypes, resultType, hasError, rm}
	return m
}

/*
 * Handler handles JSON-RPC requests and returns responses.
 *
 * Parameters:
 *   b []byte - JSON-RPC request data
 *
 * Returns:
 *   []byte - JSON-RPC response data
 */
func (svr *Server) Handler(b []byte) []byte {
	data, err := ParseRequestBody(b)
	if err != nil {
		return jsonE(nil, JsonRpc, ParseError)
	}
	var res any
	switch v := data.(type) {
	case []any:
		// An empty batch is an invalid request per the JSON-RPC 2.0 spec.
		if len(v) == 0 {
			return jsonE(nil, JsonRpc, InvalidRequest)
		}
		resList := make([]any, 0, len(v))
		for _, item := range v {
			m, ok := item.(map[string]any)
			if !ok {
				// Guard against batch entries that are not request objects.
				resList = append(resList, E(nil, JsonRpc, InvalidRequest))
				continue
			}
			resList = append(resList, svr.SingleHandler(m))
		}
		res = resList
	case map[string]any:
		res = svr.SingleHandler(v)
	default:
		return jsonE(nil, JsonRpc, InvalidRequest)
	}

	response, _ := json.Marshal(res)
	return response
}

/*
 * SingleHandler handles a single JSON-RPC request.
 *
 * Parameters:
 *   jsonMap map[string]any - Parsed JSON-RPC request
 *
 * Returns:
 *   any - JSON-RPC response object
 */
func (svr *Server) SingleHandler(jsonMap map[string]any) any {
	resp, _ := svr.HandleRequest(jsonMap, nil)
	return resp
}

/*
 * HandleRequest handles a single JSON-RPC request with optional streaming.
 *
 * When stream is non-nil and the method result is a stream type (channel,
 * slice, array or iterator), each value is pushed through the handler and
 * the returned response is the final message sent after the stream ends.
 * When stream is nil, stream results are collected into a slice so that
 * non-streaming transports keep a unary response.
 *
 * Parameters:
 *   jsonMap map[string]any - Parsed JSON-RPC request
 *   stream  *StreamHandler - Streaming handler, nil for unary dispatch
 *
 * Returns:
 *   any  - JSON-RPC response object; nil when the stream was aborted
 *   bool - Whether values were pushed through the stream handler; when
 *          false the response is a complete unary payload
 */
func (svr *Server) HandleRequest(jsonMap map[string]any, stream *StreamHandler) (any, bool) {
	id, jsonRpc, method, paramsData, errCode := ParseSingleRequestBody(jsonMap)
	if errCode != WithoutError {
		return E(id, jsonRpc, errCode), false
	}

	if svr.RateLimiter != nil && !svr.RateLimiter.Allow() {
		return CE(id, jsonRpc, "Too many requests"), false
	}

	sName, mName, err := ParseRequestMethod(method)
	if err != nil {
		return E(id, jsonRpc, MethodNotFound), false
	}
	s, ok := svr.Sm.Load(sName)
	if !ok {
		sName = LineToHump(sName) // support HelloWorld and hello_world
		s, ok = svr.Sm.Load(sName)
		if !ok {
			return E(id, jsonRpc, MethodNotFound), false
		}
	}
	m, ok := s.(*Service).Mm[mName]
	if !ok {
		return E(id, jsonRpc, MethodNotFound), false
	}
	args, err := DecodeArgs(paramsData, m.ParamsTypes)
	if err != nil {
		Debug(err)
		return E(id, jsonRpc, InvalidParams), false
	}
	hookParams := make([]any, len(args))
	for k, v := range args {
		hookParams[k] = v.Interface()
	}

	// before
	err = svr.Before(id, mName, hookParams)
	if err != nil {
		return CE(id, jsonRpc, err.Error()), false
	}

	in := make([]reflect.Value, 0, len(args)+1)
	in = append(in, s.(*Service).V)
	in = append(in, args...)
	r := m.Method.Func.Call(in)
	if m.HasError {
		if e := r[len(r)-1]; !e.IsNil() {
			Debug(e.Interface())
			return E(id, jsonRpc, InternalError), false
		}
		r = r[:len(r)-1]
	}

	streamed := m.ResultType != nil && len(r) > 0 && IsStreamType(m.ResultType)
	var result any
	// pushed reports whether values were emitted through the stream handler,
	// so the transport must deliver the final response over the same channel.
	pushed := false
	if streamed {
		if stream != nil {
			pushed = true
			if stream.Begin != nil {
				stream.Begin()
			}
			if err := EmitStream(r[0], stream.OnValue); err != nil {
				Debug(err)
				return E(id, jsonRpc, InternalError), pushed
			}
			result = nil
		} else {
			values, err := CollectStream(r[0])
			if err != nil {
				Debug(err)
				return E(id, jsonRpc, InternalError), false
			}
			result = values
		}
	} else if m.ResultType != nil && len(r) > 0 {
		result = r[0].Interface()
	}

	// after
	err = svr.After(id, mName, result)
	if err != nil {
		return CE(id, jsonRpc, err.Error()), pushed
	}

	return S(id, jsonRpc, result), pushed
}

/*
 * Before executes the before hook function if it exists.
 *
 * Parameters:
 *   id     any    - Request ID
 *   method string - Method name
 *   params any    - Request parameters
 *
 * Returns:
 *   error - Error from the hook function if it fails
 */
func (svr *Server) Before(id any, method string, params any) error {
	if svr.Hooks.BeforeFunc != nil {
		err := svr.Hooks.BeforeFunc(id, method, params)
		if err != nil {
			return err
		}
	}
	return nil
}

/*
 * After executes the after hook function if it exists.
 *
 * Parameters:
 *   id     any    - Request ID
 *   method string - Method name
 *   result any    - Request result
 *
 * Returns:
 *   error - Error from the hook function if it fails
 */
func (svr *Server) After(id any, method string, result any) error {
	if svr.Hooks.AfterFunc != nil {
		err := svr.Hooks.AfterFunc(id, method, result)
		if err != nil {
			return err
		}
	}
	return nil
}

/*
 * LineToHump converts a snake_case string to camelCase.
 *
 * Parameters:
 *   in string - Input snake_case string
 *
 * Returns:
 *   string - camelCase string
 */
func LineToHump(in string) string {
	s := strings.Split(in, "_")
	for k, v := range s {
		s[k] = Capitalize(v)
	}
	return strings.Join(s, "")
}

/*
 * Capitalize capitalizes the first letter of a string.
 *
 * Parameters:
 *   str string - Input string
 *
 * Returns:
 *   string - String with first letter capitalized
 */
func Capitalize(str string) string {
	var upperStr strings.Builder
	vv := []rune(str)
	for i := range vv {
		if i == 0 {
			if vv[i] >= 97 && vv[i] <= 122 {
				vv[i] -= 32
				upperStr.WriteString(string(vv[i]))
			} else {
				return str
			}
		} else {
			upperStr.WriteString(string(vv[i]))
		}
	}
	return upperStr.String()
}

/*
 * BindListener binds a TCP listener on host:port and back-fills port with
 * the OS-assigned value when the caller asked for port 0.
 * Panics on bind failure, matching the style used throughout this package.
 *
 * Parameters:
 *   host string - Host to bind on (e.g. "0.0.0.0")
 *   port *int   - Pointer to the requested port; updated in place when 0
 *
 * Returns:
 *   net.Listener - The bound listener, callers cast to *net.TCPListener if needed
 */
func BindListener(host string, port *int) net.Listener {
	ln, err := net.Listen("tcp", fmt.Sprintf("%s:%d", host, *port))
	if err != nil {
		log.Panic(err.Error())
	}
	if *port == 0 {
		if tcpAddr, ok := ln.Addr().(*net.TCPAddr); ok {
			*port = tcpAddr.Port
		}
	}
	return ln
}

/*
 * NotifyStarted sends the bound port on ch after a short delay so callers
 * can begin connecting once the server is accepting connections.
 * The send is non-blocking; a full channel is silently dropped to keep
 * the server goroutine from stalling.
 *
 * Parameters:
 *   port int      - The bound port number to report
 *   ch   chan<- int - Channel on which callers are waiting for readiness
 */
func NotifyStarted(port int, ch chan<- int) {
	go func() {
		time.Sleep(time.Second)
		select {
		case ch <- port:
		default:
		}
	}()
}
