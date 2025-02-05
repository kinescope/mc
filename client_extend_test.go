package mc_test

import (
	"testing"

	"github.com/kinescope/mc"
	"github.com/stretchr/testify/assert"
)

func TestMinUses(t *testing.T) {
	cache, err := mc.New(&mc.Options{
		Addrs: testServerAddrs,
	})
	if err != nil {
		t.Fatal(err)
	}

	var (
		k = randSeq(6)
		v = randSeq(6)
	)

	err = cache.Set(&mc.Item{
		Key:   k,
		Value: []byte(v),
	}, mc.WithMinUses(5))

	if assert.NoError(t, err) {
		if _, err := cache.Get(k); assert.Equal(t, mc.ErrCacheMiss, err) {
			for range 4 {
				err = cache.Set(&mc.Item{
					Key:   k,
					Value: []byte(v),
				}, mc.WithMinUses(5))
				if err != nil {
					t.Fatal(err)
				}
			}
			if i, err := cache.Get(k); assert.NoError(t, err) {
				assert.Equal(t, v, string(i.Value))
			}
		}
	}
}

func TestCompression(t *testing.T) {
	cache, err := mc.New(&mc.Options{
		Addrs: testServerAddrs,
	})
	if err != nil {
		t.Fatal(err)
	}

	for f := range 300 {
		var (
			k = randSeq(6)
			v = randSeq(1024)
		)

		if err := cache.Set(&mc.Item{
			Key:   k,
			Value: mc.Value(v),
			Flags: uint16(f),
		}, mc.WithCompression(256)); assert.NoError(t, err) {

			if i, err := cache.Get(k); assert.NoError(t, err) {
				assert.Equal(t, f, int(i.Flags))
				assert.Equal(t, k, i.Key)
				assert.Equal(t, v, string(i.Value))
			}
		}
	}

	for n := range 10 {
		n += 1
		var (
			k = randSeq(6)
			v = randSeq(512 * n)
		)

		if err := cache.Set(&mc.Item{
			Key:   k,
			Value: mc.Value(v),
			Flags: uint16(n),
		}, mc.WithCompression(1024)); assert.NoError(t, err) {

			if i, err := cache.Get(k); assert.NoError(t, err) {
				assert.Equal(t, n, int(i.Flags))
				assert.Equal(t, k, i.Key)
				assert.Equal(t, v, string(i.Value))
				t.Logf("compression ratio: size=%d, ratio=%f", len(v), i.CompressionRatio())
			}
		}
	}
}

/*
	func TestScalingExpiration(t *testing.T) {
		ctx := context.Background()
		cache, err := mc.New(&mc.Options{
			Addrs: testServerAddrs,
		})
		if err != nil {
			t.Fatal(err)
		}

		var (
			k = randSeq(6)
			v = randSeq(6)
		)
		err = cache.Set(ctx, &mc.Item{
			Key:   k,
			Value: []byte(v),
		}, mc.WithExpiration(2, 2))
		if assert.NoError(t, err) {
			if i, err := cache.Get(ctx, k); assert.NoError(t, err) {
				if assert.Equal(t, v, string(i.Value)) {
					time.Sleep(3 * time.Second)
					if _, err := cache.Get(ctx, k); assert.Equal(t, mc.ErrCacheMiss, err) {
						if i, err := cache.Get(ctx, k); assert.NoError(t, err) {
							if assert.Equal(t, v, string(i.Value)) {
								time.Sleep(2 * time.Second)
								if _, err := cache.Get(ctx, k); assert.Error(t, err) {
									assert.Equal(t, mc.ErrCacheMiss, err)
								}
							}
						}
					}
				}
			}
		}
	}
*/
func TestNamespace(t *testing.T) {
	cache, err := mc.New(&mc.Options{
		Addrs: testServerAddrs,
	})
	if err != nil {
		t.Fatal(err)
	}

	ns := randSeq(10)
	keyVal := make(map[string]string)

	for range 10 {
		keyVal[randSeq(10)] = randSeq(10)
	}
	for k, v := range keyVal {
		err = cache.Set(&mc.Item{
			Key:   k,
			Value: []byte(v),
		}, mc.WithNamespace(ns))
		if err != nil {
			t.Fatal(err)
		}
	}

	var (
		k   = randSeq(6)
		v   = randSeq(6)
		ns2 = randSeq(6)
	)
	err = cache.Set(&mc.Item{
		Key:   k,
		Value: []byte(v),
	}, mc.WithNamespace(ns2))
	if err != nil {
		t.Fatal(err)
	}
	for k, v := range keyVal {
		if i, err := cache.Get(k); assert.NoError(t, err) {
			assert.Equal(t, v, string(i.Value))
		}
	}
	if i, err := cache.Get(k); assert.NoError(t, err) {
		assert.Equal(t, v, string(i.Value))
	}

	if err := cache.PurgeNamespace(ns); assert.NoError(t, err) {
		if i, err := cache.Get(k); assert.NoError(t, err) {
			if assert.Equal(t, v, string(i.Value)) {
				for k := range keyVal {
					if _, err := cache.Get(k); assert.Error(t, err) {
						assert.Equal(t, mc.ErrCacheMiss, err)
					}
				}
			}
		}
	}
	if i, err := cache.Get(k); assert.NoError(t, err) {
		assert.Equal(t, v, string(i.Value))
	}
}
