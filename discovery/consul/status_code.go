package consul

/**
 * @Description: Service status code - passing
 */
const STATUS_CODE_PASSING = 200

/**
 * @Description: Consul status code mapping table, used to explain the meaning of different status codes
 */
var StatusCodeMap = map[int]string{
	200: "All health checks of every matching service instance are passing",
	400: "Bad parameter (missing service name of id)",
	404: "No such service id or name",
	429: "Some health checks are passing, at least one is warning",
	503: "At least one of the health checks is critical",
}
