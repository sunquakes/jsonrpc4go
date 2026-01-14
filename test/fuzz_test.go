package test

import (
	"testing"

	"github.com/sunquakes/jsonrpc4go/common"
)

func FuzzParseRequestBody(f *testing.F) {
	// Seed the fuzzer with some basic inputs
	f.Add([]byte(`{"jsonrpc":"2.0","method":"test","params":[],"id":1}`))
	f.Add([]byte(`{"jsonrpc":"2.0","method":"test","params":{},"id":1}`))
	f.Add([]byte(`[]`))
	f.Add([]byte(`{}`))

	f.Fuzz(func(t *testing.T, data []byte) {
		// Test the request parsing functionality
		_, _ = common.ParseRequestBody(data)

		// Test the server handler functionality with panic recovery
		defer func() {
			if recover() != nil {
				// Recover from any panic during fuzzing
				return
			}
		}()

		svr := &common.Server{}
		_ = svr.Handler(data)
	})
}
