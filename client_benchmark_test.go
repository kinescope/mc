package mc_test

import (
	"bytes"
	"testing"

	"github.com/bradfitz/gomemcache/memcache"
	"github.com/kinescope/mc"
)

func BenchmarkParallel(b *testing.B) {
	cache, err := mc.New(&mc.Options{
		Addrs:                    testServerAddrs,
		DisableBinaryEncodedKeys: true,
	})
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		var (
			k = randSeq(16)
			v = randSeq(24)
		)
		err = cache.Set(&mc.Item{
			Key:   k,
			Value: []byte(v),
		})
		if err != nil {
			b.Fatal(err)
		}
		for pb.Next() {
			i, err := cache.Get(k)
			if err != nil {
				b.Fatal(err)
			}
			if !bytes.Equal(i.Value, []byte(v)) {
				b.Fatal(err)
			}
		}
	})
}

func BenchmarkGet(b *testing.B) {
	cache, err := mc.New(&mc.Options{
		Addrs:                    testServerAddrs,
		DisableBinaryEncodedKeys: true,
	})
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()

	cache.Set(&mc.Item{
		Key:   "benchmark_get",
		Value: []byte("benchmark"),
	})
	for range b.N {
		if _, err := cache.Get("benchmark_get"); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkOriginalGet(b *testing.B) {
	cache := memcache.New(testServerAddrs...)
	b.ReportAllocs()
	b.ResetTimer()
	cache.Set(&memcache.Item{
		Key:   "benchmark_get",
		Value: []byte("benchmark"),
	})
	for range b.N {
		if _, err := cache.Get("benchmark_get"); err != nil {
			b.Fatal(err)
		}
	}
}
