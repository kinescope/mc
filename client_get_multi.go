package mc

import (
	"errors"
	"time"
)

func (c *Client) GetMulti(keys []string, o ...MgOption) (_ map[string]*Item, retErr error) {
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
	for addr, items := range keyMap {
		ch := make(chan *Item)
		chs = append(chs, ch)
		go func(addr string, items []string, ch chan *Item) (goErr error) {
			defer close(ch)
			conn, err := c.pool.getConn(addr)
			if err != nil {
				return nil
			}
			if !opt.deadline.IsZero() {
				conn.nc.SetDeadline(opt.deadline)
			}
			defer func() {
				if !opt.deadline.IsZero() {
					conn.nc.SetDeadline(time.Time{})
				}
				c.pool.condRelease(conn, goErr)
			}()

			for _, key := range items {
				opt.opaque = keyNum[key]
				conn.buff.Write(append(c.makeGetCmd(key, opt), crlf...))
			}

			conn.buff.Write([]byte("mn\r\n"))
			if err := conn.buff.Flush(); err != nil {
				return err
			}

			var item *Item
			for range len(items) + 1 {
				if item, err = parseGetResponse(c, conn.buff); err == nil {
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
		for item := range ch {
			items[item.Key] = item
		}
	}
	return items, nil

}
