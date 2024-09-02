package mc_test

import (
	"math/rand"

	"github.com/kinescope/mc"
)

const testServerAddr = "127.0.0.1:11211"

var letters = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")

func randSeq(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}

func newTestClient(addr ...string) (*mc.Client, error) {
	cache, err := mc.New(&mc.Options{
		Addrs: addr,
	})
	if err != nil {
		return nil, err
	}
	return cache, nil
}
