package mc_test

import (
	"testing"
	"time"

	"github.com/kinescope/mc"
	"github.com/stretchr/testify/assert"
)

func TestBase(t *testing.T) {
	k := randSeq(16)
	cache, err := mc.New(&mc.Options{
		Addrs: testServerAddrs,
	})
	if err != nil {
		t.Fatal(err)
	}
	cache.Set(&mc.Item{
		Key:   k,
		Value: []byte(randSeq(16)),
	}, mc.WithCompression(1_000))
	t.Log(cache.Get(k))
}

func TestAddSet(t *testing.T) {
	cache, err := mc.New(&mc.Options{
		Addrs: testServerAddrs,
	})
	if err != nil {
		t.Fatal(err)
	}
	var (
		k = randSeq(16)
		v = randSeq(24)
	)

	err = cache.Add(&mc.Item{
		Key:   k,
		Value: []byte(v),
	})

	if assert.NoError(t, err) {
		if i, err := cache.Get(k); assert.NoError(t, err) {
			if assert.Equal(t, v, string(i.Value)) {
				err = cache.Add(&mc.Item{
					Key: k,
				})
				if assert.Error(t, err) {
					assert.Equal(t, mc.ErrEmptyValue, err)
				}
				v = randSeq(24)
				err = cache.Set(&mc.Item{
					Key:   k,
					Value: []byte(v),
				})
				if assert.NoError(t, err) {
					if i, err := cache.Get(k); assert.NoError(t, err) {
						assert.Equal(t, v, string(i.Value))
					}
				}
				err = cache.Add(&mc.Item{
					Key: k,
				})
				if assert.Error(t, err) {
					assert.Equal(t, mc.ErrEmptyValue, err)
				}
			}
		}
	}
}

func TestCompareAndSwap(t *testing.T) {
	cache, err := mc.New(&mc.Options{
		Addrs: testServerAddrs,
	})
	if err != nil {
		t.Fatal(err)
	}
	var (
		k = randSeq(16)
		v = randSeq(24)
	)

	err = cache.Add(&mc.Item{
		Key:   k,
		Value: []byte(v),
	})

	if assert.NoError(t, err) {
		if i, err := cache.Get(k, mc.WithCAS()); assert.NoError(t, err) {
			if assert.Equal(t, v, string(i.Value)) {
				v = randSeq(24)
				i.Value = []byte(v)
				if err = cache.CompareAndSwap(i); assert.NoError(t, err) {
					if i, err := cache.Get(k, mc.WithCAS()); assert.NoError(t, err) {
						if assert.Equal(t, v, string(i.Value)) {
							err = cache.Set(&mc.Item{
								Key:   k,
								Value: []byte(v),
							})
							if assert.NoError(t, err) {
								if err = cache.CompareAndSwap(i); assert.Error(t, err) {
									assert.Equal(t, mc.ErrCASConflict, err)
								}
							}
						}
					}
				}
			}
		}
	}
}

func TestIncrDecr(t *testing.T) {
	cache, err := mc.New(&mc.Options{
		Addrs: testServerAddrs,
	})
	if err != nil {
		t.Fatal(err)
	}
	{
		k := randSeq(6)
		if _, err := cache.Inc(k, 1, 10); assert.Error(t, err) {
			if assert.Equal(t, mc.ErrCacheMiss, err) {
				err = cache.Set(&mc.Item{
					Key:   k,
					Value: []byte("0"),
				})
				if assert.NoError(t, err) {
					for n := range 10 {
						if v, err := cache.Inc(k, 1, 10); assert.NoError(t, err) {
							assert.Equal(t, uint64(n+1), v)
						}
					}
					for n := range 10 {
						if v, err := cache.Dec(k, 1, 10); assert.NoError(t, err) {
							assert.Equal(t, uint64(9-n), v)
						}
					}
				}
			}
		}
	}
}
func TestIncrDecrBad(t *testing.T) {
	cache, err := mc.New(&mc.Options{
		Addrs: testServerAddrs,
	})
	if err != nil {
		t.Fatal(err)
	}
	{
		k := randSeq(6)
		if _, err := cache.Inc(k, 1, 10); assert.Error(t, err) {
			if assert.Equal(t, mc.ErrCacheMiss, err) {
				err = cache.Set(&mc.Item{
					Key:   k,
					Value: []byte("non-numeric"),
				})
				if assert.NoError(t, err) {
					if _, err := cache.Inc(k, 1, 10); assert.Error(t, err) {
						assert.Equal(t, mc.ErrBadIncrDec, err)
					}
				}
			}
		}
	}
}
func TestIncrDecrWithInitial(t *testing.T) {
	cache, err := mc.New(&mc.Options{
		Addrs: testServerAddrs,
	})
	if err != nil {
		t.Fatal(err)
	}
	{
		k := randSeq(6)
		for n := range 10 {
			if v, err := cache.Inc(k, 1, 10, mc.WithInitialValue(1)); assert.NoError(t, err) {
				assert.Equal(t, uint64(n+1), v)
			}
		}
		for n := range 10 {
			if v, err := cache.Dec(k, 1, 10); assert.NoError(t, err) {
				assert.Equal(t, uint64(9-n), v)
			}
		}
	}
}

func TestDelete(t *testing.T) {

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
	})

	if assert.NoError(t, err) {
		if i, err := cache.Get(k); assert.NoError(t, err) {
			if assert.Equal(t, v, string(i.Value)) {
				if err := cache.Del(k); assert.NoError(t, err) {
					if _, err := cache.Get(k); assert.Error(t, err) {
						assert.Equal(t, mc.ErrCacheMiss, err)
					}
				}
			}
		}
	}
}

func TestDeadline(t *testing.T) {
	cache, err := mc.New(&mc.Options{
		Addrs: testServerAddrs,
	})
	if err != nil {
		t.Fatal(err)
	}

	if _, err = cache.Get(randSeq(6), mc.WithDeadline(time.Now().Add(-time.Second))); assert.Error(t, err) {
		if e, ok := err.(interface {
			Timeout() bool
		}); assert.True(t, ok) {
			assert.True(t, e.Timeout())
		}
	}
}
