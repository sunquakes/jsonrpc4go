package servers

import (
	"github.com/sunquakes/jsonrpc4go/discovery"
)

/**
 * @Description: Static server list type, implements discovery.Driver interface
 */
type Servers string

/**
 * @Description: Create Servers instance
 * @Param rawURL: Server URL address
 * @Return discovery.Driver: Service discovery driver instance
 * @Return error: Error message
 */
func NewServers(rawURL string) (discovery.Driver, error) {
	servers := Servers(rawURL)
	return &servers, nil
}

/**
 * @Description: Register service (static server list doesn't need actual registration)
 * @Param name: Service name
 * @Param protocol: Protocol type
 * @Param hostname: Hostname
 * @Param port: Port number
 * @Return error: Error message
 */
func (d *Servers) Register(name string, protocol string, hostname string, port int) error {
	return nil
}

/**
 * @Description: Get server address
 * @Param name: Service name
 * @Return string: Server URL address
 * @Return error: Error message
 */
func (d *Servers) Get(name string) (string, error) {
	return string(*d), nil
}
