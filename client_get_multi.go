package mc

import (
	"context"
	"errors"
	"time"
)

func (c *Client) GetMulti(ctx context.Context, keys []string, o ...MgOption) (_ map[string]*Item, retErr error) {
	keyNum := make(map[string]int)
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
		keyNum[key] = n + 1
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
		go func(addr string, items []string, ch chan *Item) (goErr error) {
			defer close(ch)
			// Создать локальную копию opt для этой горутины
			localOpt := opt
			conn, err := c.pool.getConn(addr)
			if err != nil {
				return nil
			}
			if !deadline.IsZero() {
				conn.nc.SetDeadline(deadline)
			}
			defer func() {
				if !deadline.IsZero() {
					conn.nc.SetDeadline(time.Time{})
				}
				c.pool.condRelease(conn, goErr)
			}()

			// Check context before operations
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}

			for _, key := range items {
				localOpt.opaque = keyNum[key]
				conn.buff.Write(append(c.makeGetCmd(key, localOpt), crlf...))
			}

			conn.buff.Write([]byte("mn\r\n"))
			if err := conn.buff.Flush(); err != nil {
				return err
			}

			var item *Item
			for range len(items) + 1 {
				if item, err = parseGetResponse(ctx, c, conn.buff); err == nil {
					item.Key = keys[item.opaque-1]
					ch <- item
					continue
				}
				if !errors.Is(err, ErrCacheMiss) {
					return err
				}
			}

			return nil

		}(addr, items, ch)
	}
	items := make(map[string]*Item)
	for _, ch := range chs {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
			for item := range ch {
				items[item.Key] = item
			}
		}
	}
	return items, nil

}
