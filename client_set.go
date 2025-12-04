package mc

import (
	"context"
	"strconv"
	"time"

	"github.com/kinescope/mc/proto/cache"
)

// Add stores an item only if the key doesn't already exist.
// If the key exists, it returns ErrNotStored.
// This is useful for initializing cache entries or implementing
// distributed locks.
func (c *Client) Add(ctx context.Context, i *Item, o ...MsOption) error {
	return c.populateOne(ctx, "E", i, 0, o...)
}

// Set stores an item, overwriting any existing value for the key.
// This is the most common operation for storing cache entries.
func (c *Client) Set(ctx context.Context, i *Item, o ...MsOption) error {
	return c.populateOne(ctx, "S", i, 0, o...)
}

// CompareAndSwap updates an item only if its CAS (Compare-And-Swap) value
// matches the CAS value stored in the item. This provides optimistic locking
// to prevent race conditions in concurrent applications.
// Returns ErrCASConflict if the CAS value doesn't match (item was modified
// by another client).
// The item must be retrieved with WithCAS() to get the CAS value.
func (c *Client) CompareAndSwap(ctx context.Context, i *Item, o ...MsOption) error {
	return c.populateOne(ctx, "S", i, i.cas, o...)
}

// Append appends data to an existing item. If the item doesn't exist and WithExpiration
// is provided, the item will be created with that TTL (autovivify). Otherwise returns ErrNotStored.
func (c *Client) Append(ctx context.Context, i *Item, o ...MsOption) error {
	return c.populateOne(ctx, "A", i, 0, o...)
}

// Prepend prepends data to an existing item. If the item doesn't exist and WithExpiration
// is provided, the item will be created with that TTL (autovivify). Otherwise returns ErrNotStored.
func (c *Client) Prepend(ctx context.Context, i *Item, o ...MsOption) error {
	return c.populateOne(ctx, "P", i, 0, o...)
}

// Replace replaces an existing item. If the item doesn't exist, it returns ErrNotStored.
func (c *Client) Replace(ctx context.Context, i *Item, o ...MsOption) error {
	return c.populateOne(ctx, "R", i, 0, o...)
}

/*

- b: interpret key as base64 encoded binary value (see metaget)
- c: return CAS value if successfully stored.
- C(token): compare CAS value when storing item
- E(token): use token as new CAS value (see metaget for detail)
- F(token): set client flags to token (32 bit unsigned numeric)
- I: invalidate. set-to-invalid if supplied CAS is older than item's CAS
- k: return key as a token
- O(token): opaque value, consumes a token and copies back with response
- q: use noreply semantics for return codes
- s: return the size of the stored item on success (ie; new size on append)
- T(token): Time-To-Live for item, see "Expiration" above.
- M(token): mode switch to change behavior to add, replace, append, prepend
- N(token): if in append mode, autovivify on miss with supplied TTL

E: "add" command. LRU bump and return NS if item exists. Else
add.
A: "append" command. If item exists, append the new value to its data.
P: "prepend" command. If item exists, prepend the new value to its data.
R: "replace" command. Set only if item already exists.
S: "set" command. The default mode, added for completeness.
*/

// https://github.com/memcached/memcached/blob/master/doc/protocol.txt#L685
func (c *Client) populateOne(ctx context.Context, mode string, i *Item, cas uint64, o ...MsOption) (retErr error) {
	if len(i.Value) == 0 {
		return ErrEmptyValue
	}

	var opts msOpts

	for _, fn := range o {
		fn(&opts)
	}

	if opts.minUses != 0 {
		if v, err := c.Inc(ctx, i.Key+"::_min_uses", 1, opts.expiration, WithInitialValue(1)); err == nil && v < opts.minUses {
			return nil
		}
	}

	// Check context before operations
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	key, err := c.encodeKey(i.Key)
	if err != nil {
		return err
	}
	conn, err := c.pickServer(key)
	if err != nil {
		return err
	}
	defer c.pool.condRelease(conn, retErr)

	// Use minimum of opts.deadline and context.Deadline() if both are set
	deadline := opts.deadline
	if ctxDeadline, ok := ctx.Deadline(); ok {
		if deadline.IsZero() || ctxDeadline.Before(deadline) {
			deadline = ctxDeadline
		}
	}
	if !deadline.IsZero() {
		conn.nc.SetDeadline(deadline)
		defer conn.nc.SetDeadline(time.Time{})
	}

	var (
		flags  int
		source [4]byte
	)

	if len(opts.namespace) != 0 {
		flags |= serialized
		ver, err := c.nsVersion(ctx, opts.namespace, 0)
		if err != nil {
			return err
		}
		i.Value, err = (&cache.Item{
			Data: i.Value,
			Namespace: &cache.Namespace{
				Key: opts.namespace,
				Ver: ver,
			},
		}).Marshal()
		if err != nil {
			return err
		}
	}

	if opts.compressionMinLen != 0 && len(i.Value) > opts.compressionMinLen {
		flags |= compressed
		if i.Value, err = c.compress(i.Value); err != nil {
			return err
		}
	}

	cmd := []byte("ms " + key + " ")
	cmd = strconv.AppendInt(cmd, int64(len(i.Value)), 10)
	cmd = append(cmd, ' ', 'M')
	cmd = append(cmd, mode...)
	if opts.expiration != 0 {
		cmd = append(cmd, ' ', 'T')
		cmd = strconv.AppendUint(cmd, uint64(opts.expiration), 10)
		// For append/prepend operations, use expiration as autovivify TTL
		if mode == "A" || mode == "P" {
			cmd = append(cmd, ' ', 'N')
			cmd = strconv.AppendUint(cmd, uint64(opts.expiration), 10)
		}
	}

	if i.Flags != 0 || flags != 0 {

		endian.PutUint16(source[:2], uint16(flags))
		endian.PutUint16(source[2:], uint16(i.Flags))

		cmd = append(cmd, ' ', 'F')
		cmd = strconv.AppendUint(cmd, uint64(endian.Uint32(source[:])), 10)
	}

	if !c.opts.DisableBinaryEncodedKeys {
		cmd = append(cmd, ' ', 'b')
	}
	if cas != 0 {
		cmd = append(cmd, ' ', 'C')
		cmd = strconv.AppendUint(cmd, cas, 10)
	}

	conn.buff.Write(cmd)
	conn.buff.Write(crlf)
	conn.buff.Write(i.Value)
	conn.buff.Write(crlf)

	if err := conn.buff.Flush(); err != nil {
		return err
	}

	return parseResponse(conn.buff.Reader)
}
