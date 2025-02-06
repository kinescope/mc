package mc

import (
	"time"

	"github.com/dgryski/go-ketama"
)

const (
	DefaultTimeout             = 200 * time.Millisecond
	DefaultConnMaxLifetime     = time.Minute
	DefaultMaxIdleConnsPerAddr = 10
)

type Options struct {
	Addrs       []string
	PickServer  func(key string) []string
	DialTimeout time.Duration
	//	ReadTimeout              time.Duration
	//	WriteTimeout             time.Duration
	ConnMaxLifetime          time.Duration
	MaxIdleConnsPerAddr      int
	DisableBinaryEncodedKeys bool
}

func (o *Options) setDefaults() error {
	if o.DialTimeout == 0 {
		o.DialTimeout = DefaultTimeout
	}
	/*if o.ReadTimeout == 0 {
		o.ReadTimeout = DefaultTimeout
	}
	if o.WriteTimeout == 0 {
		o.WriteTimeout = DefaultTimeout
	}*/
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

type (
	MgOption func(c *mgOpts)
	MsOption func(c *msOpts)
	MdOption func(c *mdOpts)
	MaOption func(c *maOpts)
)

type (
	mgOpts struct {
		cas          bool
		opaque       int
		deadline     time.Time
		lastAccess   bool
		earlyRecache int
	}
	msOpts struct {
		namespace         string
		minUses           uint64
		expiration        uint32
		compressionMinLen int
	}
	mdOpts struct {
		expiration uint32
	}
	maOpts struct {
		initialValue uint64
	}
)

// Get

func WithCAS() MgOption {
	return func(c *mgOpts) {
		c.cas = true
	}
}

func WithDeadline(t time.Time) MgOption {
	return func(c *mgOpts) {
		c.deadline = t
	}
}

func WithLastAccess() MgOption {
	return func(c *mgOpts) {
		c.lastAccess = true
	}
}

func WithEarlyRecache(seconds int) MgOption {
	return func(c *mgOpts) {
		c.earlyRecache = seconds
	}
}

// Set

func WithMinUses(number uint64) MsOption {
	return func(c *msOpts) {
		c.minUses = number
	}
}

func WithNamespace(ns string) MsOption {
	return func(c *msOpts) {
		c.namespace = ns
	}
}

func WithExpiration(seconds uint32) MsOption {
	return func(c *msOpts) {
		c.expiration = seconds
	}
}

func WithCompression(minLen int) MsOption {
	return func(c *msOpts) {
		c.compressionMinLen = minLen
	}
}

// Arithmetic

func WithInitialValue(v uint64) MaOption {
	return func(c *maOpts) {
		c.initialValue = v
	}
}

// Del

// mark as stale
func WithInvalidate(seconds uint32) MdOption {
	return func(c *mdOpts) {
		c.expiration = seconds
	}
}
