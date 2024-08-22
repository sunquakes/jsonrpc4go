package client

import (
	"math/rand"
	"time"
)

func GetAddress(addressList []string) string {
	source := rand.NewSource(time.Now().UnixNano())
	r := rand.New(source)
	key := r.Intn(len(addressList))
	return addressList[key]
}
