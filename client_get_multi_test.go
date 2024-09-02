package mc_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/kinescope/mc"
	"github.com/stretchr/testify/assert"
)

func TestGetMulti(t *testing.T) {
	cache, err := mc.New(&mc.Options{
		Addrs: testServerAddrs,
	})
	if err != nil {
		t.Fatal(err)
	}
	var (
		ctx    = context.Background()
		keyVal = make(map[string]string)
		keys   []string
	)
	for n := range 20 {
		var (
			k = fmt.Sprintf("%s_%d", randSeq(16), n)
			v = fmt.Sprintf("%s_%d", randSeq(16), n)
		)
		err := cache.Set(ctx, &mc.Item{
			Key:   k,
			Value: []byte(v),
		})
		if err != nil {
			t.Fatal(err)
		}
		keyVal[k] = v
		keys = append(keys, k)
	}
	if list, err := cache.GetMulti(ctx, keys...); assert.NoError(t, err) {
		for k, v := range keyVal {
			assert.Equal(t, v, string(list[k].Value))
		}
	}
}

func TestGetMultiXXKeyHash(t *testing.T) {
	cache, err := mc.New(&mc.Options{
		Addrs:       testServerAddrs,
		KeyHashFunc: mc.XXKeyHashFunc,
	})
	if err != nil {
		t.Fatal(err)
	}
	var (
		ctx    = context.Background()
		keyVal = make(map[string]string)
		keys   []string
	)
	for n := range 20 {
		var (
			k = fmt.Sprintf("%s_%d", randSeq(16), n)
			v = fmt.Sprintf("%s_%d", randSeq(16), n)
		)
		err := cache.Set(ctx, &mc.Item{
			Key:   k,
			Value: []byte(v),
		})
		if err != nil {
			t.Fatal(err)
		}
		keyVal[k] = v
		keys = append(keys, k)
	}
	if list, err := cache.GetMulti(ctx, keys...); assert.NoError(t, err) {
		for k, v := range keyVal {
			assert.Equal(t, v, string(list[k].Value))
		}
	}
}
func TestGetMultiScalingExpiration(t *testing.T) {
	cache, err := mc.New(&mc.Options{
		Addrs: testServerAddrs,
	})
	if err != nil {
		t.Fatal(err)
	}
	var (
		ctx    = context.Background()
		keyVal = make(map[string]string)
		keys   []string
	)
	for n := range 20 {
		var (
			k = fmt.Sprintf("%s_%d", randSeq(16), n)
			v = fmt.Sprintf("%s_%d", randSeq(16), n)
		)
		expiration := 2
		if n%2 == 0 {
			expiration = 10
			keyVal[k] = v
		}
		err := cache.Set(ctx, &mc.Item{
			Key:   k,
			Value: []byte(v),
		}, mc.WithExpiration(uint32(expiration), 2))
		if err != nil {
			t.Fatal(err)
		}

		keys = append(keys, k)
	}
	time.Sleep(3 * time.Second)
	if list, err := cache.GetMulti(ctx, keys...); assert.NoError(t, err) {
		if assert.Len(t, list, len(keys)/2) {
			for k, v := range keyVal {
				assert.Equal(t, v, string(list[k].Value))
			}
		}
	}
}

func TestGetMultiNamespace(t *testing.T) {
	cache, err := mc.New(&mc.Options{
		Addrs: testServerAddrs,
	})
	if err != nil {
		t.Fatal(err)
	}
	var (
		ns1    = randSeq(5)
		ns2    = randSeq(5)
		ctx    = context.Background()
		keyVal = []map[string]string{
			make(map[string]string),
			make(map[string]string),
		}
		keys = make([][]string, 2)
	)
	for i, ns := range []string{ns1, ns2} {
		for n := range 20 {
			var (
				k = fmt.Sprintf("%s_%d", randSeq(16), n)
				v = fmt.Sprintf("%s_%d", randSeq(16), n)
			)
			err := cache.Set(ctx, &mc.Item{
				Key:   k,
				Value: []byte(v),
			}, mc.WithNamespace(ns))
			if err != nil {
				t.Fatal(err)
			}
			keyVal[i][k], keys[i] = v, append(keys[i], k)
		}
	}
	for i, keys := range keys {
		if list, err := cache.GetMulti(ctx, keys...); assert.NoError(t, err) {
			for k, v := range keyVal[i] {
				assert.Equal(t, v, string(list[k].Value))
			}
		}
	}

	cache.PurgeNamespace(ctx, ns1)
	if list, err := cache.GetMulti(ctx, keys[1]...); assert.NoError(t, err) {
		if assert.Len(t, list, len(keys[1])) {
			for k, v := range keyVal[1] {
				assert.Equal(t, v, string(list[k].Value))
			}
		}
	}
	if list, err := cache.GetMulti(ctx, keys[0]...); assert.NoError(t, err) {
		assert.Len(t, list, 0)
	}
}
