package mc

import (
	"context"
	"errors"
	"time"
)

// GetMulti retrieves multiple items from the cache in a single operation.
// Keys are distributed across servers based on the PickServer function,
// and requests to the same server are batched together for efficiency.
// Returns a map of found items (keys that don't exist are not included).
// This is more efficient than multiple Get() calls, especially when
// keys are on different servers.
func (c *Client) GetMulti(ctx context.Context, keys []string, o ...MgOption) (_ map[string]*Item, retErr error) {
	if len(keys) == 0 {
		return make(map[string]*Item), nil
	}

	keyNum := make(map[string]int, len(keys))
	keyMap := make(map[string][]string)
	for n, k := range keys {
		key, err := c.encodeKey(k)
		if err != nil {
			return nil, err
		}
		addrs := c.opts.PickServer(key)
		if len(addrs) == 0 {
			return nil, ErrNoServers
		}
		opaque := n + 1
		keyNum[key] = opaque
		keyMap[addrs[0]] = append(keyMap[addrs[0]], key)
	}

	var (
		opt mgOpts
		chs []chan *Item
	)
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

	for addr, items := range keyMap {
		ch := make(chan *Item)
		chs = append(chs, ch)
		go func(addr string, items []string, ch chan *Item, keyNumMap map[string]int, allKeys []string) {
			defer close(ch)
			localOpt := opt

			conn, err := c.pool.getConn(addr)
			if err != nil {
				return
			}
			if !deadline.IsZero() {
				conn.nc.SetDeadline(deadline)
			}
			defer func() {
				if !deadline.IsZero() {
					conn.nc.SetDeadline(time.Time{})
				}
				c.pool.condRelease(conn, err)
			}()

			// Check context before operations
			select {
			case <-ctx.Done():
				return
			default:
			}

			// Send all mg commands
			for _, key := range items {
				localOpt.opaque = keyNumMap[key]
				cmd := c.makeGetCmd(key, localOpt)
				conn.buff.Write(cmd)
				conn.buff.Write(crlf)
			}

			// Send mn command to signal end of batch
			conn.buff.WriteString("mn\r\n")
			if err := conn.buff.Flush(); err != nil {
				return
			}

			// Read responses until we get "MN" (end of multi-get)
			var item *Item
			for {
				if item, err = parseGetResponse(ctx, c, conn.buff); err == nil {
					// Restore original key using opaque value
					if item.opaque > 0 && item.opaque <= len(allKeys) {
						item.Key = allKeys[item.opaque-1]
						ch <- item
					}
					continue
				}
				// Check for end of multi-get (MN response)
				if errors.Is(err, errMnDone) {
					break
				}
				// ErrCacheMiss is expected for non-existent keys
				if errors.Is(err, ErrCacheMiss) {
					continue
				}
				// Any other error is unexpected, stop reading
				return
			}
		}(addr, items, ch, keyNum, keys)
	}
	items := make(map[string]*Item, len(keys))
	for _, ch := range chs {
		for item := range ch {
			items[item.Key] = item
		}
	}
	return items, nil
}
