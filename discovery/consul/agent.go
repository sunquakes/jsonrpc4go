package consul

import (
	"net/http"
	"net/url"
)

type Agent struct {
	Url   string
	Token string
}

func (a *Agent) GetHealthServices(name string) {
	address, _ := url.ParseRequestURI(a.Url)
	address.Path = "/agent/service/" + name
	query := address.Query()
	query.Set("token", a.Token)
	address.RawQuery = query.Encode()
	http.Get(address.String())
}
