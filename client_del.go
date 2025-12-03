package mc

import (
	"context"
	"strconv"
	"time"
)

// Del deletes an item from the cache by its key.
// Returns ErrCacheMiss if the key doesn't exist.
// Supports WithInvalidate() to mark the item as stale instead of deleting it,
// which allows serving stale data while refreshing.
func (c *Client) Del(ctx context.Context, k string, o ...MdOption) (retErr error) {
	var opts mdOpts
	for _, fn := range o {
		fn(&opts)
	}

	// Check context before operations
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	key, err := c.encodeKey(k)
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

	cmd := []byte("md " + key)
	if !c.opts.DisableBinaryEncodedKeys {
		cmd = append(cmd, ' ', 'b')
	}

	if opts.expiration != 0 {
		cmd = append(cmd, ' ', 'I', ' ', 'T')
		cmd = strconv.AppendUint(cmd, uint64(opts.expiration), 10)
	}

	conn.buff.Write(cmd)
	conn.buff.Write(crlf)

	if err := conn.buff.Flush(); err != nil {
		return err
	}
	return parseResponse(conn.buff.Reader)
}
