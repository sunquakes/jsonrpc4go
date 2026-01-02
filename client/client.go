package client

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
	 * Call executes a single JSON-RPC method call.
	 *
	 * Parameters:
	 *   string - JSON-RPC method name
	 *   any    - Method parameters
	 *   any    - Pointer to store the result
	 *   bool   - Whether this is a notification (no response expected)
	 *
	 * Returns:
	 *   error - Error if the call fails
	 */
	Call(string, any, any, bool) error

	/*
	 * BatchAppend adds a request to the batch operation list.
	 *
	 * Parameters:
	 *   string - JSON-RPC method name
	 *   any    - Method parameters
	 *   any    - Pointer to store the result
	 *   bool   - Whether this is a notification (no response expected)
	 *
	 * Returns:
	 *   *error - Pointer to an error variable that will be populated if the request fails
	 */
	BatchAppend(string, any, any, bool) *error

	/*
	 * BatchCall executes all requests in the batch list.
	 *
	 * Returns:
	 *   error - Error if the batch operation fails
	 */
	BatchCall() error
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
