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
	address.Query().Set("token", a.Token)
	http.Get(address.String())
}
