package mc

// https://docs.memcached.org/protocols/meta/
import (
	"context"
	"encoding/binary"
	"time"
)

var crlf = []byte("\r\n")

var endian = binary.BigEndian

const (
	compressed = 2
	serialized = 4
)

// New creates a new memcache client with the provided options.
// It initializes connection pools for each server address and sets up
// key encoding based on the configuration.
// Returns an error if no servers are provided or if server selection
// function setup fails.
func New(opts *Options) (*Client, error) {
	opts.setDefaults()
	if len(opts.Addrs) == 0 {
		return nil, ErrNoServers
	}
	var (
		cli = &Client{
			opts: opts,
			pool: pool{
				idle:            make(map[string]chan *conn),
				dialTimeout:     opts.DialTimeout,
				connMaxLifetime: opts.ConnMaxLifetime,
			},
			encodeKey: binaryEncodeKey,
		}
	)
	if opts.DisableBinaryEncodedKeys {
		cli.encodeKey = func(s string) (string, error) {
			if !checkKey(s) {
				return "", ErrMalformedKey
			}
			return s, nil
		}
	}
	for _, s := range opts.Addrs {
		cli.pool.idle[s] = make(chan *conn, opts.MaxIdleConnsPerAddr)
	}
	return cli, nil
}

type Client struct {
	pool      pool
	opts      *Options
	encodeKey func(string) (string, error)
}

// PurgeNamespace invalidates all cache entries for the given namespace.
// This is done by incrementing the namespace version, which causes all
// items stored with that namespace to become invalid on the next access.
// This is useful for cache invalidation by user, tenant, or any logical grouping.
func (c *Client) PurgeNamespace(ctx context.Context, ns string) error {
	if _, err := c.nsVersion(ctx, ns, 1); err != nil {
		return err
	}
	return nil
}

func (c *Client) nsVersion(ctx context.Context, ns string, delta uint64) (uint64, error) {
	return c.Inc(ctx, "namespace::"+ns, delta, 0, WithInitialValue(uint64(time.Now().UnixNano())))
}

// Close closes all connections in the pool and releases resources.
// After Close is called, the Client should not be used.
func (c *Client) Close() error {
	return c.pool.close()
}
