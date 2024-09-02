package mc

import (
	"sync"
	"time"

	"github.com/cespare/xxhash/v2"
	"github.com/dgryski/go-ketama"
)

const (
	DefaultTimeout             = 500 * time.Millisecond
	DefaultConnMaxLifetime     = 30 * time.Minute
	DefaultMaxIdleConnsPerAddr = 10
)

var xxHashPool = sync.Pool{
	New: func() any {
		return xxhash.New()
	},
}

var (
	DefaultKeyHashFunc = func(key string) []byte {
		return str2byte(key)
	}
	XXKeyHashFunc = func(key string) []byte {
		hash := xxHashPool.Get().(*xxhash.Digest)
		hash.Reset()
		hash.WriteString(key)
		sum := hash.Sum(nil)
		{
			xxHashPool.Put(hash)
		}
		return sum
	}
)

type Options struct {
	Addrs               []string
	PickServer          func(key string) []string
	KeyHashFunc         func(key string) []byte
	DialTimeout         time.Duration
	ConnMaxLifetime     time.Duration
	MaxIdleConnsPerAddr int
}

func (o *Options) setDefaults() error {
	if o.KeyHashFunc == nil {
		o.KeyHashFunc = DefaultKeyHashFunc
	}
	if o.DialTimeout == 0 {
		o.DialTimeout = DefaultTimeout
	}
	if o.ConnMaxLifetime == 0 {
		o.ConnMaxLifetime = DefaultConnMaxLifetime
	}
	if o.MaxIdleConnsPerAddr == 0 {
		o.MaxIdleConnsPerAddr = DefaultMaxIdleConnsPerAddr
	}

	if o.PickServer == nil && len(o.Addrs) != 0 {
		buckets := make([]ketama.Bucket, 0, len(o.Addrs))
		for _, s := range o.Addrs {
			buckets = append(buckets, ketama.Bucket{
				Label:  s,
				Weight: 1,
			})
		}
		hash, err := ketama.New(buckets)
		if err != nil {
			return err
		}
		o.PickServer = func(key string) []string {
			return hash.HashMultiple(key, len(o.Addrs))
		}
	}
	return nil
}

type Option func(c *opts)

type opts struct {
	initial           uint64
	minUses           uint32
	namespace         string
	expiration        uint32
	scalingExpiration uint32
}

// Inc only
func WithInitial(v uint64) Option {
	return func(c *opts) {
		c.initial = v
	}
}

func WithMinUses(number uint32) Option {
	return func(c *opts) {
		c.minUses = number
	}
}

func WithNamespace(ns string) Option {
	return func(c *opts) {
		c.namespace = ns
	}
}

// https://github.com/memcached/memcached/wiki/ProgrammingTricks#scaling-expiration
// при истечении срока жизни первый зарос получает cache miss, все остальные -- hit, в
// течении указанного scale
func WithExpiration(exp, scale uint32) Option {
	return func(c *opts) {
		c.expiration = exp
		c.scalingExpiration = scale
	}
}
