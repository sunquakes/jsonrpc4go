package server

import (
	"github.com/sunquakes/jsonrpc4go/discovery"
	"golang.org/x/time/rate"
)

const REGISTRY_RETRY_INTERVAL = 3000

/*
 * Protocol defines the interface for server protocol implementations.
 */
type Protocol interface {
	/*
	 * NewServer creates a new server instance for this protocol.
	 *
	 * Returns:
	 *   Server - A new server instance implementing the Server interface
	 */
	NewServer() Server
}

/*
 * Server defines the interface for JSON-RPC server operations.
 */
type Server interface {
	/*
	 * SetBeforeFunc sets a callback function to be executed before processing a request.
	 *
	 * Parameters:
	 *   func(id any, method string, params any) error - Callback function to execute before processing
	 */
	SetBeforeFunc(func(id any, method string, params any) error)

	/*
	 * SetAfterFunc sets a callback function to be executed after processing a request.
	 *
	 * Parameters:
	 *   func(id any, method string, result any) error - Callback function to execute after processing
	 */
	SetAfterFunc(func(id any, method string, result any) error)

	/*
	 * SetOptions configures protocol-specific options for the server.
	 *
	 * Parameters:
	 *   any - Protocol-specific options to configure
	 */
	SetOptions(any)

	/*
	 * SetDiscovery configures service discovery for the server.
	 *
	 * Parameters:
	 *   d         discovery.Driver - Service discovery driver to use
	 *   hostname  string          - Hostname to register with the discovery service
	 */
	SetDiscovery(d discovery.Driver, hostname string)

	/*
	 * SetRateLimit configures rate limiting for the server.
	 *
	 * Parameters:
	 *   rate.Limit - The rate limit (requests per second)
	 *   int        - The burst size (maximum number of requests allowed at once)
	 */
	SetRateLimit(rate.Limit, int)

	/*
	 * Start starts the server and begins listening for requests.
	 */
	Start()

	/*
	 * Register registers a service with the server.
	 *
	 * Parameters:
	 *   s any - Service object to register (methods will be exposed as RPC endpoints)
	 */
	Register(s any)

	/*
	 * DiscoveryRegister registers the server with the discovery service.
	 *
	 * Parameters:
	 *   key   interface{} - Registration key
	 *   value interface{} - Registration value
	 *
	 * Returns:
	 *   bool - True if registration was successful, false otherwise
	 */
	DiscoveryRegister(key, value interface{}) bool

	/*
	 * GetEvent returns a channel that receives events from the server.
	 *
	 * Returns:
	 *   <-chan int - Channel that receives server events
	 */
	GetEvent() <-chan int
}

/*
 * NewServer creates a new JSON-RPC server from a protocol implementation.
 *
 * Parameters:
 *   p T - Protocol implementation to use for the server
 *
 * Returns:
 *   Server - A new JSON-RPC server instance
 */
func NewServer[T Protocol](p T) Server {
	return p.NewServer()
}
