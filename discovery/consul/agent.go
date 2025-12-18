package consul

import (
	"net/http"
	"net/url"
)

/**
 * @Description: Consul Agent client structure
 * @Field Url: Consul server URL address
 * @Field Token: Authentication token
 */
type Agent struct {
	Url   string
	Token string
}

/**
 * @Description: Get health services
 * @Receiver a: Agent structure pointer
 * @Param name: Service name
 */
func (a *Agent) GetHealthServices(name string) {
	address, _ := url.ParseRequestURI(a.Url)
	address.Path = "/agent/service/" + name
	query := address.Query()
	query.Set("token", a.Token)
	address.RawQuery = query.Encode()
	http.Get(address.String())
}