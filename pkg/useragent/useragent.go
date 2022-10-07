package useragent

import (
	"math/rand"
	"time"
)

// Random return random useragnet
func Random() string {
	rand.Seed(time.Now().Unix())
	index := rand.Int31n(int32(len(useragents)))
	return useragents[index]
}
