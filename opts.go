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
	Addrs                    []string
	PickServer               func(key string) []string
	DialTimeout              time.Duration
	ConnMaxLifetime          time.Duration
	MaxIdleConnsPerAddr      int
	DisableBinaryEncodedKeys bool
	Compression              struct {
		Compress   func([]byte) ([]byte, error)
		Decompress func([]byte) ([]byte, error)
	}
}

func (o *Options) setDefaults() error {
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

type (
	MgOption func(c *mgOpts)
	MsOption func(c *msOpts)
	MdOption func(c *mdOpts)
	MaOption func(c *maOpts)
)

type (
	mgOpts struct {
		cas          bool
		hit          bool
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
		deadline          time.Time
	}
	mdOpts struct {
		expiration uint32
		deadline   time.Time
	}
	maOpts struct {
		initialValue *uint64
		deadline     time.Time
	}
)

// Get options

// WithCAS requests the CAS (Compare-And-Swap) value to be returned with the item.
// The CAS value can be used with CompareAndSwap() for optimistic locking.
func WithCAS() MgOption {
	return func(c *mgOpts) {
		c.cas = true
	}
}

// WithHit requests the hit status to be returned with the item.
// Hit status indicates whether the item has been accessed before,
// useful for identifying hot cache items.
func WithHit() MgOption {
	return func(c *mgOpts) {
		c.hit = true
	}
}

// WithDeadline sets a deadline for the Get operation.
// The operation will timeout if not completed by the deadline.
// If context also has a deadline, the minimum of both is used.
func WithDeadline(t time.Time) MgOption {
	return func(c *mgOpts) {
		c.deadline = t
	}
}

// WithLastAccess requests the last access time to be returned with the item.
// Returns the time in seconds since the item was last accessed.
func WithLastAccess() MgOption {
	return func(c *mgOpts) {
		c.lastAccess = true
	}
}

// WithEarlyRecache enables early recaching to prevent cache stampede.
// If the remaining TTL is less than the specified seconds, this client
// may "win" the recache flag and should refresh the cache in the background.
// Other clients will get stale data immediately.
func WithEarlyRecache(seconds int) MgOption {
	return func(c *mgOpts) {
		c.earlyRecache = seconds
	}
}

// Set options

// WithMinUses requires an item to be set a minimum number of times
// before it can be retrieved. This is useful for preventing race conditions
// where an item might be read before it's fully initialized.
func WithMinUses(number uint64) MsOption {
	return func(c *msOpts) {
		c.minUses = number
	}
}

// WithNamespace stores the item with a namespace, allowing all items
// in a namespace to be invalidated at once using PurgeNamespace().
// This is useful for cache invalidation by user, tenant, or logical grouping.
func WithNamespace(ns string) MsOption {
	return func(c *msOpts) {
		c.namespace = ns
	}
}

// WithExpiration sets the Time-To-Live (TTL) for the item in seconds.
// For append/prepend operations, this also enables autovivify (automatic
// creation) if the key doesn't exist.
func WithExpiration(seconds uint32) MsOption {
	return func(c *msOpts) {
		c.expiration = seconds
	}
}

// WithCompression enables compression for values larger than minLen bytes.
// Uses the compression functions provided in Options.Compression.
func WithCompression(minLen int) MsOption {
	return func(c *msOpts) {
		c.compressionMinLen = minLen
	}
}

// WithDeadlineSet sets a deadline for the Set operation.
// The operation will timeout if not completed by the deadline.
// If context also has a deadline, the minimum of both is used.
func WithDeadlineSet(t time.Time) MsOption {
	return func(c *msOpts) {
		c.deadline = t
	}
}

// Arithmetic options

// WithInitialValue sets the initial value for Inc/Dec operations
// if the key doesn't exist. Without this, Inc/Dec will fail if the key is missing.
func WithInitialValue(v uint64) MaOption {
	return func(c *maOpts) {
		c.initialValue = &v
	}
}

// WithDeadlineArithmetic sets a deadline for the arithmetic operation.
// The operation will timeout if not completed by the deadline.
// If context also has a deadline, the minimum of both is used.
func WithDeadlineArithmetic(t time.Time) MaOption {
	return func(c *maOpts) {
		c.deadline = t
	}
}

// Delete options

// WithInvalidate marks the item as stale instead of deleting it.
// The item will be served as stale data for the specified number of seconds,
// allowing clients to serve stale data while refreshing in the background.
func WithInvalidate(seconds uint32) MdOption {
	return func(c *mdOpts) {
		c.expiration = seconds
	}
}

// WithDeadlineDel sets a deadline for the Delete operation.
// The operation will timeout if not completed by the deadline.
// If context also has a deadline, the minimum of both is used.
func WithDeadlineDel(t time.Time) MdOption {
	return func(c *mdOpts) {
		c.deadline = t
	}
}
