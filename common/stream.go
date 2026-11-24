package common

import (
	"errors"
	"reflect"
)

/*
 * StreamHandler lets a transport push method results value by value.
 *
 * Fields:
 *   Begin   func()          - Called once before the first value is emitted,
 *                             so the transport can switch to a streaming response
 *   OnValue func(any) error - Called for every streamed value; a returned error
 *                             stops the stream
 */
type StreamHandler struct {
	Begin   func()
	OnValue func(any) error
}

var (
	boolType  = reflect.TypeFor[bool]()
	errorType = reflect.TypeFor[error]()
)

// IsChanStream reports whether t is a receive-capable channel type.
func isChanStream(t reflect.Type) bool {
	if t.Kind() != reflect.Chan {
		return false
	}
	dir := t.ChanDir()
	return dir == reflect.RecvDir || dir == reflect.BothDir
}

// iterSeqShape reports whether t is func(func(T) bool) or func(func(K, V) bool),
// the runtime shapes of iter.Seq[T] and iter.Seq2[K, V].
func iterSeqShape(t reflect.Type) (yield reflect.Type, ok bool) {
	if t.Kind() != reflect.Func || t.NumIn() != 1 || t.NumOut() != 0 {
		return nil, false
	}
	y := t.In(0)
	if y.Kind() != reflect.Func || y.NumOut() != 1 || y.Out(0) != boolType {
		return nil, false
	}
	if n := y.NumIn(); n == 1 || n == 2 {
		return y, true
	}
	return nil, false
}

/*
 * IsStreamType reports whether a method result type is streamed value by
 * value on transports that support streaming: channels, slices, arrays and
 * iterator functions shaped like iter.Seq / iter.Seq2.
 */
func IsStreamType(t reflect.Type) bool {
	if t == nil {
		return false
	}
	switch t.Kind() {
	case reflect.Slice, reflect.Array:
		return true
	case reflect.Chan:
		return isChanStream(t)
	case reflect.Func:
		_, ok := iterSeqShape(t)
		return ok
	}
	return false
}

/*
 * EmitStream pushes every value of a stream result to onValue. Slices and
 * arrays are emitted element by element, channels are drained until closed,
 * and iterator functions are driven with a generated yield callback. For
 * iter.Seq2-shaped iterators the second value is treated as an error when
 * its type is error and iteration stops on the first non-nil error.
 */
func EmitStream(v reflect.Value, onValue func(any) error) error {
	t := v.Type()
	switch t.Kind() {
	case reflect.Slice, reflect.Array:
		for k := range v.Len() {
			if err := onValue(v.Index(k).Interface()); err != nil {
				return err
			}
		}
		return nil
	case reflect.Chan:
		if !isChanStream(t) {
			return errors.New("rpc: channel type cannot be received from")
		}
		for {
			x, ok := v.Recv()
			if !ok {
				return nil
			}
			if err := onValue(x.Interface()); err != nil {
				return err
			}
		}
	case reflect.Func:
		yield, ok := iterSeqShape(t)
		if !ok {
			return errors.New("rpc: unsupported iterator signature")
		}
		var iterErr error
		cb := reflect.MakeFunc(yield, func(in []reflect.Value) []reflect.Value {
			if len(in) == 2 {
				if in[1].Type() == errorType && !in[1].IsNil() {
					iterErr, _ = in[1].Interface().(error)
					return []reflect.Value{reflect.ValueOf(false)}
				}
				if err := onValue(in[0].Interface()); err != nil {
					return []reflect.Value{reflect.ValueOf(false)}
				}
				return []reflect.Value{reflect.ValueOf(true)}
			}
			if err := onValue(in[0].Interface()); err != nil {
				return []reflect.Value{reflect.ValueOf(false)}
			}
			return []reflect.Value{reflect.ValueOf(true)}
		})
		v.Call([]reflect.Value{cb})
		return iterErr
	}
	return errors.New("rpc: value is not a stream")
}

/*
 * CollectStream drains a stream result into a slice, used by transports
 * without streaming support.
 */
func CollectStream(v reflect.Value) ([]any, error) {
	values := []any{}
	err := EmitStream(v, func(x any) error {
		values = append(values, x)
		return nil
	})
	return values, err
}
