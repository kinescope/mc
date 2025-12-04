package mc_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/kinescope/mc"
	"github.com/stretchr/testify/assert"
)

func TestGetMulti(t *testing.T) {
	// Use only available servers from testServerAddrs
	availableAddrs := checkAvailableServers(t, testServerAddrs)
	if len(availableAddrs) == 0 {
		t.Skip("No available servers")
	}

	ctx := context.Background()
	cache, err := mc.New(&mc.Options{
		Addrs: availableAddrs,
	})
	if err != nil {
		t.Fatal(err)
	}
	var (
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
	if list, err := cache.GetMulti(ctx, keys); assert.NoError(t, err) {
		for k, v := range list {
			assert.Equal(t, v.Value, list[k].Value)
		}
	}
}

func TestGetMultiDisableBinaryEncodedKeys(t *testing.T) {
	// Use only available servers from testServerAddrs
	availableAddrs := checkAvailableServers(t, testServerAddrs)
	if len(availableAddrs) == 0 {
		t.Skip("No available servers")
	}

	ctx := context.Background()
	cache, err := mc.New(&mc.Options{
		Addrs:                    availableAddrs,
		DisableBinaryEncodedKeys: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	var (
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
	if list, err := cache.GetMulti(ctx, keys); assert.NoError(t, err) {
		for k, v := range list {
			assert.Equal(t, v.Value, list[k].Value)
		}
	}
}

func TestGetMultiNamespace(t *testing.T) {
	// Use only available servers from testServerAddrs
	availableAddrs := checkAvailableServers(t, testServerAddrs)
	if len(availableAddrs) == 0 {
		t.Skip("No available servers")
	}

	ctx := context.Background()
	cache, err := mc.New(&mc.Options{
		Addrs: availableAddrs,
	})
	if err != nil {
		t.Fatal(err)
	}
	var (
		ns1    = randSeq(5)
		ns2    = randSeq(5)
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
		if list, err := cache.GetMulti(ctx, keys); assert.NoError(t, err) {
			for k, v := range list {
				assert.Equal(t, keyVal[i][k], string(v.Value))
			}
		}
	}

	cache.PurgeNamespace(ctx, ns1)
	if list, err := cache.GetMulti(ctx, keys[1]); assert.NoError(t, err) {
		if assert.Len(t, list, len(keys[1])) {
			for k, v := range list {
				assert.Equal(t, keyVal[1][k], string(v.Value))
			}
		}
	}
	if list, err := cache.GetMulti(ctx, keys[0]); assert.NoError(t, err) {
		assert.Len(t, list, 0)
	}
}
