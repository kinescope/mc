package mc_test

import (
	"context"
	"testing"
	"time"

	"github.com/kinescope/mc"
	"github.com/stretchr/testify/assert"
)

const testServerAddr = "127.1.2.1:11211"

func TestAddSet(t *testing.T) {
	cache, cancel, err := newTestClient(testServerAddr)
	if err != nil {
		t.Fatal(err)
	}
	defer cancel()
	var (
		k = randSeq(16)
		v = randSeq(24)
	)

	ctx := context.Background()

	err = cache.Add(ctx, &mc.Item{
		Key:   k,
		Value: []byte(v),
	})

	if assert.NoError(t, err) {
		if i, err := cache.Get(ctx, k); assert.NoError(t, err) {
			if assert.Equal(t, v, string(i.Value)) {
				err = cache.Add(ctx, &mc.Item{
					Key: k,
				})
				if assert.Error(t, err) {
					assert.Equal(t, mc.ErrAlreadyExists, err)
				}
				v = randSeq(24)
				err = cache.Set(ctx, &mc.Item{
					Key:   k,
					Value: []byte(v),
				})
				if assert.NoError(t, err) {
					if i, err := cache.Get(ctx, k); assert.NoError(t, err) {
						assert.Equal(t, v, string(i.Value))
					}
				}
			}
		}
	}
}

func TestCompareAndSwap(t *testing.T) {
	cache, cancel, err := newTestClient(testServerAddr)
	if err != nil {
		t.Fatal(err)
	}
	defer cancel()
	var (
		k = randSeq(16)
		v = randSeq(24)
	)

	ctx := context.Background()

	err = cache.Add(ctx, &mc.Item{
		Key:   k,
		Value: []byte(v),
	})

	if assert.NoError(t, err) {
		if i, err := cache.Get(ctx, k); assert.NoError(t, err) {
			if assert.Equal(t, v, string(i.Value)) {
				v = randSeq(24)
				i.Value = []byte(v)
				if err = cache.CompareAndSwap(ctx, i); assert.NoError(t, err) {
					if i, err := cache.Get(ctx, k); assert.NoError(t, err) {
						if assert.Equal(t, v, string(i.Value)) {
							err = cache.Set(ctx, &mc.Item{
								Key:   k,
								Value: []byte(v),
							})
							if assert.NoError(t, err) {
								if err = cache.CompareAndSwap(ctx, i); assert.Error(t, err) {
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
	ctx := context.Background()
	cache, cancel, err := newTestClient(testServerAddr)
	if err != nil {
		t.Fatal(err)
	}
	defer cancel()
	{
		k := randSeq(6)
		if _, err := cache.Inc(ctx, k, 1); assert.Error(t, err) {
			if assert.Equal(t, mc.ErrCacheMiss, err) {
				err = cache.Set(ctx, &mc.Item{
					Key:   k,
					Value: []byte("0"),
				})
				if assert.NoError(t, err) {
					for n := range 10 {
						if v, err := cache.Inc(ctx, k, 1); assert.NoError(t, err) {
							assert.Equal(t, uint64(n+1), v)
						}
					}
					for n := range 10 {
						if v, err := cache.Dec(ctx, k, 1); assert.NoError(t, err) {
							assert.Equal(t, uint64(9-n), v)
						}
					}
				}
			}
		}
	}
}
func TestIncrDecrBad(t *testing.T) {
	ctx := context.Background()
	cache, cancel, err := newTestClient(testServerAddr)
	if err != nil {
		t.Fatal(err)
	}
	defer cancel()
	{
		k := randSeq(6)
		if _, err := cache.Inc(ctx, k, 1); assert.Error(t, err) {
			if assert.Equal(t, mc.ErrCacheMiss, err) {
				err = cache.Set(ctx, &mc.Item{
					Key:   k,
					Value: []byte("non-numeric"),
				})
				if assert.NoError(t, err) {
					if _, err := cache.Inc(ctx, k, 1); assert.Error(t, err) {
						assert.Equal(t, mc.ErrBadIncrDec, err)
					}
				}
			}
		}
	}
}
func TestIncrDecrWithInitial(t *testing.T) {
	ctx := context.Background()
	cache, cancel, err := newTestClient(testServerAddr)
	if err != nil {
		t.Fatal(err)
	}
	defer cancel()
	{
		k := randSeq(6)
		for n := range 10 {
			if v, err := cache.Inc(ctx, k, 1, mc.WithInitial(1)); assert.NoError(t, err) {
				assert.Equal(t, uint64(n+1), v)
			}
		}
		for n := range 10 {
			if v, err := cache.Dec(ctx, k, 1); assert.NoError(t, err) {
				assert.Equal(t, uint64(9-n), v)
			}
		}
	}
}

func TestDelete(t *testing.T) {
	ctx := context.Background()
	cache, cancel, err := newTestClient(testServerAddr)
	if err != nil {
		t.Fatal(err)
	}
	defer cancel()

	var (
		k = randSeq(6)
		v = randSeq(6)
	)

	err = cache.Set(ctx, &mc.Item{
		Key:   k,
		Value: []byte(v),
	})

	if assert.NoError(t, err) {
		if i, err := cache.Get(ctx, k); assert.NoError(t, err) {
			if assert.Equal(t, v, string(i.Value)) {
				if err := cache.Delete(ctx, k); assert.NoError(t, err) {
					if _, err := cache.Get(ctx, k); assert.Error(t, err) {
						assert.Equal(t, mc.ErrCacheMiss, err)
					}
				}
			}
		}
	}
}

func TestDeadline(t *testing.T) {
	cache, cancel, err := newTestClient(testServerAddr)
	if err != nil {
		t.Fatal(err)
	}
	defer cancel()

	ctx, done := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer done()

	if _, err = cache.Get(ctx, randSeq(6)); assert.Error(t, err) {
		if e, ok := err.(interface {
			Timeout() bool
		}); assert.True(t, ok) {
			assert.True(t, e.Timeout())
		}
	}
}
