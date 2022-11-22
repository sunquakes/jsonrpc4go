package discovery

type Driver interface {
	Register(name string, protocol string, hostname string, port int)
	Get(name string) (string, error)
}
