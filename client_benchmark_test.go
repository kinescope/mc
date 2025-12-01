package mc_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/bradfitz/gomemcache/memcache"
	"github.com/kinescope/mc"
)

func BenchmarkParallel(b *testing.B) {
	ctx := context.Background()
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

func BenchmarkParallelMulti(b *testing.B) {
	ctx := context.Background()
	cache, err := mc.New(&mc.Options{
		Addrs:                    []string{"127.0.0.1:11211"},
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
		err = cache.Set(ctx, &mc.Item{
			Key:   k,
			Value: []byte(v),
		})
		if err != nil {
			b.Fatal(err)
		}
		for pb.Next() {
			i, err := cache.GetMulti(ctx, []string{k})
			if err != nil {
				b.Fatal(err)
			}
			if !bytes.Equal(i[k].Value, []byte(v)) {
				b.Fatal(err)
			}
		}
	})
}

func BenchmarkGet(b *testing.B) {
	ctx := context.Background()
	cache, err := mc.New(&mc.Options{
		Addrs:                    testServerAddrs,
		DisableBinaryEncodedKeys: true,
	})
	if err != nil {
		b.Fatal(err)
	}

	cache.Set(ctx, &mc.Item{
		Key:   "benchmark_get",
		Value: []byte("benchmark"),
	})

	b.ReportAllocs()
	b.ResetTimer()

	for range b.N {
		if _, err := cache.Get(ctx, "benchmark_get"); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkGetMulti(b *testing.B) {
	ctx := context.Background()
	cache, err := mc.New(&mc.Options{
		Addrs:                    testServerAddrs,
		DisableBinaryEncodedKeys: true,
	})
	if err != nil {
		b.Fatal(err)
	}

	cache.Set(ctx, &mc.Item{
		Key:   "benchmark_get",
		Value: []byte("benchmark"),
	})

	b.ReportAllocs()
	b.ResetTimer()

	for range b.N {
		if _, err := cache.GetMulti(ctx, []string{"benchmark_get"}); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkOriginalGet(b *testing.B) {
	cache := memcache.New(testServerAddrs...)

	cache.Set(&memcache.Item{
		Key:   "benchmark_get",
		Value: []byte("benchmark"),
	})

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		if _, err := cache.Get("benchmark_get"); err != nil {
			b.Fatal(err)
		}
	}
}
