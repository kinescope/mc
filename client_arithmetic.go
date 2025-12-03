package mc

import (
	"context"
	"strconv"
	"time"
)

// Inc increments a numeric value stored at the given key by delta.
// If the key doesn't exist, it can be created with an initial value
// using WithInitialValue(). Returns the new value after increment.
// The value must be numeric (stored as a string representation of a number).
func (c *Client) Inc(ctx context.Context, k string, delta uint64, expiration uint32, o ...MaOption) (new uint64, _ error) {
	return c.arithmetic(ctx, "M+", k, delta, expiration, o...)
}

// Dec decrements a numeric value stored at the given key by delta.
// If the key doesn't exist, it can be created with an initial value
// using WithInitialValue(). Returns the new value after decrement.
// The value must be numeric (stored as a string representation of a number).
func (c *Client) Dec(ctx context.Context, k string, delta uint64, expiration uint32, o ...MaOption) (new uint64, _ error) {
	return c.arithmetic(ctx, "M-", k, delta, expiration, o...)
}

func (c *Client) arithmetic(ctx context.Context, op, k string, delta uint64, expiration uint32, o ...MaOption) (new uint64, retErr error) {
	var opts maOpts
	for _, fn := range o {
		fn(&opts)
	}

	// Check context before operations
	select {
	case <-ctx.Done():
		return 0, ctx.Err()
	default:
	}

	key, err := c.encodeKey(k)
	if err != nil {
		return 0, err
	}
	conn, err := c.pickServer(key)
	if err != nil {
		return 0, err
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

	cmd := []byte("ma " + key + " " + op + " v D")
	cmd = strconv.AppendUint(cmd, delta, 10)

	if opts.initialValue != nil {
		cmd = append(cmd, []byte(" N0 J")...)
		cmd = strconv.AppendUint(cmd, *opts.initialValue, 10)
	}

	if expiration > 0 {
		cmd = append(cmd, ' ', 'T')
		cmd = strconv.AppendUint(cmd, uint64(expiration), 10)
	}

	if !c.opts.DisableBinaryEncodedKeys {
		cmd = append(cmd, ' ', 'b')
	}

	conn.buff.Write(append(cmd, crlf...))

	if err := conn.buff.Flush(); err != nil {
		return 0, err
	}

	item, err := parseGetResponse(ctx, c, conn.buff)
	if err != nil {
		return 0, err
	}

	return strconv.ParseUint(string(item.Value), 10, 64)
}
