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
	address, err := url.ParseRequestURI(a.Url)
	if err != nil {
	}
	address.Path = "/agent/service/" + name
	address.Query().Set("token", a.Token)
	http.Get(address.String())
}
