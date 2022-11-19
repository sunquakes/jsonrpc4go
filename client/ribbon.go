package client

import (
	"math/rand"
	"time"
)

func GetAddress(addressList []string) string {
	rand.Seed(time.Now().UnixNano())
	key := rand.Intn(len(addressList))
	return addressList[key]
}
