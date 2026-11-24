package client

import (
	"iter"
	"strconv"
	"sync/atomic"
)

// requestID generates unique request identifiers for this process.
var requestID atomic.Uint64

// nextRequestID returns a unique JSON-RPC request id.
func nextRequestID() string {
	return strconv.FormatUint(requestID.Add(1), 10)
}

/*
 * Protocol defines the interface for client protocol implementations.
 */
type Protocol interface {
	/*
	 * NewClient creates a new client instance for this protocol.
	 *
	 * Returns:
	 *   Client - A new client instance implementing the Client interface
	 */
	NewClient() Client
}

/*
 * BatchResult holds the outcome of one request inside a batch call.
 *
 * Fields:
 *   Result any   - The decoded result value of the request
 *   Err    error - The error returned for the request, nil on success
 */
type BatchResult struct {
	Result any
	Err    error
}

/*
 * Client defines the interface for JSON-RPC client operations.
 */
type Client interface {
	/*
	 * SetOptions configures protocol-specific options for the client.
	 *
	 * Parameters:
	 *   any - Protocol-specific options to configure
	 */
	SetOptions(any)

	/*
	 * SetPoolOptions configures connection pool options for the client.
	 *
	 * Parameters:
	 *   any - Pool-specific options to configure
	 */
	SetPoolOptions(any)

	/*
	 * Call executes a single JSON-RPC method call with positional arguments.
	 *
	 * Parameters:
	 *   method string - JSON-RPC method name
	 *   args   ...any - Method arguments, sent as a positional array
	 *
	 * Returns:
	 *   any   - The decoded result value
	 *   error - Error if the call fails
	 */
	Call(method string, args ...any) (any, error)

	/*
	 * Notify executes a JSON-RPC notification; no result is returned.
	 *
	 * Parameters:
	 *   method string - JSON-RPC method name
	 *   args   ...any - Method arguments, sent as a positional array
	 *
	 * Returns:
	 *   error - Error if the notification fails
	 */
	Notify(method string, args ...any) error

	/*
	 * BatchAppend adds a request with positional arguments to the batch list.
	 *
	 * Parameters:
	 *   method string - JSON-RPC method name
	 *   args   ...any - Method arguments, sent as a positional array
	 */
	BatchAppend(method string, args ...any)

	/*
	 * BatchCall executes all requests in the batch list.
	 *
	 * Returns:
	 *   []BatchResult - Per-request results in the order they were appended
	 *   error         - Error if the batch operation itself fails
	 */
	BatchCall() ([]BatchResult, error)
}

/*
 * Streamer defines the optional interface for transports that can push
 * streaming results value by value.
 */
type Streamer interface {
	/*
	 * Stream executes a JSON-RPC method call and yields each streamed
	 * result value as it arrives. Unary results yield exactly one value.
	 *
	 * Parameters:
	 *   method string - JSON-RPC method name
	 *   args   ...any - Method arguments, sent as a positional array
	 *
	 * Returns:
	 *   iter.Seq2[any, error] - Sequence of streamed values and errors
	 */
	Stream(method string, args ...any) iter.Seq2[any, error]
}

/*
 * normalizeArgs turns a nil argument list into an empty slice so that
 * params is always encoded as a JSON array on the wire.
 */
func normalizeArgs(args []any) []any {
	if args == nil {
		return []any{}
	}
	return args
}

/*
 * NewClient creates a new JSON-RPC client from a protocol implementation.
 *
 * Parameters:
 *   p T - Protocol implementation to use for the client
 *
 * Returns:
 *   Client - A new JSON-RPC client instance
 */
func NewClient[T Protocol](p T) Client {
	return p.NewClient()
}
