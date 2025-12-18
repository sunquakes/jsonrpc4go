package discovery

/**
 * @Description: Service discovery driver interface
 */
type Driver interface {
	/**
	 * @Description: Register service
	 * @Param name: Service name
	 * @Param protocol: Protocol type
	 * @Param hostname: Hostname
	 * @Param port: Port number
	 * @Return error: Error message
	 */
	Register(name string, protocol string, hostname string, port int) error
	/**
	 * @Description: Get service address
	 * @Param name: Service name
	 * @Return string: Service address
	 * @Return error: Error message
	 */
	Get(name string) (string, error)
}
