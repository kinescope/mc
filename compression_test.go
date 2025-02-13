package mc

import (
	"bytes"
	"crypto/rand"
	"testing"
)

func BenchmarkCompressParallel(b *testing.B) {
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			data := make([]byte, 512)
			if _, err := rand.Read(data); err != nil {
				b.Fatal(err)
			}
			c, err := compress(data)
			if err != nil {
				b.Fatal(err)
			}
			d2, err := decompress(c)
			if err != nil {
				b.Fatal(err)
			}
			if !bytes.Equal(data, d2) {
				b.Fatalf("data != d")
			}
		}
	})
}
