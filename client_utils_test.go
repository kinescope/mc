package mc_test

import (
	"math/rand"
)

var testServerAddrs = []string{
	"127.0.0.1:11211",
	"127.0.0.1:11212",
	"127.0.0.1:11213",
}

var letters = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")

func randSeq(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}
