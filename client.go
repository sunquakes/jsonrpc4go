package jsonrpc4go

import (
	"errors"
	"github.com/sunquakes/jsonrpc4go/client"
	"reflect"
	"strings"
)

func NewClient(protocol string, address any) (client.Client, error) {
	var p client.Protocol
	var addressList []string
	if reflect.TypeOf(address).Kind() == reflect.String {
		addressList = strings.Split(address.(string), ",")
	} else {

	}
	switch protocol {
	case "http":
		p = &client.Http{protocol, addressList}
	case "tcp":
		p = &client.Tcp{protocol, addressList}
	default:
		return nil, errors.New("The protocol can not be supported")
	}
	return client.NewClient(p), nil
}
