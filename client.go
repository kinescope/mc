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
