package mc

import (
	"context"
	"time"
)

// Get retrieves an item from the cache by its key.
// Returns ErrCacheMiss if the key doesn't exist.
// Supports various options like WithCAS(), WithEarlyRecache(), WithHit(),
// and WithLastAccess() to get additional metadata or modify behavior.
func (c *Client) Get(ctx context.Context, k string, o ...MgOption) (_ *Item, retErr error) {
	var (
		opt      mgOpts
		key, err = c.encodeKey(k)
	)

	if err != nil {
		return nil, err
	}

	conn, err := c.pickServer(key)
	if err != nil {
		return nil, err
	}

	for _, fn := range o {
		fn(&opt)
	}

	// Use minimum of opt.deadline and context.Deadline() if both are set
	deadline := opt.deadline
	if ctxDeadline, ok := ctx.Deadline(); ok {
		if deadline.IsZero() || ctxDeadline.Before(deadline) {
			deadline = ctxDeadline
		}
	}
	if !deadline.IsZero() {
		conn.nc.SetDeadline(deadline)
	}

	defer func() {
		if !deadline.IsZero() {
			conn.nc.SetDeadline(time.Time{})
		}
		c.pool.condRelease(conn, retErr)
	}()

	// Check context before operations
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	cmd := c.makeGetCmd(key, opt)
	conn.buff.Write(cmd)
	conn.buff.Write(crlf)

	if err := conn.buff.Flush(); err != nil {
		return nil, err
	}

	item, err := parseGetResponse(ctx, c, conn.buff)
	if err != nil {
		return nil, err
	}
	item.Key = k
	return item, nil
}
