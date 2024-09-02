package mc_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/bradfitz/gomemcache/memcache"
	"github.com/kinescope/mc"
)

func BenchmarkParallel(b *testing.B) {
	cache, cancel, err := newTestClient(testServerAddr)
	if err != nil {
		b.Fatal(err)
	}
	defer cancel()
	b.ReportAllocs()
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		ctx := context.Background()
		var (
			k = randSeq(16)
			v = randSeq(24)
		)
		err = cache.Set(ctx, &mc.Item{
			Key:   k,
			Value: []byte(v),
		})
		if err != nil {
			b.Fatal(err)
		}
		for pb.Next() {
			i, err := cache.Get(ctx, k)
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
	cache, cancel, err := newTestClient(testServerAddr)
	if err != nil {
		b.Fatal(err)
	}
	defer cancel()
	b.ReportAllocs()
	b.ResetTimer()
	ctx := context.Background()
	cache.Set(ctx, &mc.Item{
		Key:   "benchmark_get",
		Value: []byte("benchmark"),
	})
	for range b.N {
		if _, err := cache.Get(ctx, "benchmark_get"); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkOriginalGet(b *testing.B) {
	cancel, err := newTestServer(testServerAddr)
	if err != nil {
		b.Fatal(err)
	}
	defer cancel()
	cache := memcache.New(testServerAddr)
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
