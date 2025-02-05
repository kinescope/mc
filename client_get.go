package mc

import "time"

func (c *Client) Get(k string, o ...MgOption) (_ *Item, retErr error) {
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

	if !opt.deadline.IsZero() {
		conn.nc.SetDeadline(opt.deadline)
	}

	defer func() {
		if !opt.deadline.IsZero() {
			conn.nc.SetDeadline(time.Time{})
		}
		c.pool.condRelease(conn, retErr)
	}()

	conn.buff.Write(append(c.makeGetCmd(key, opt), crlf...))

	if err := conn.buff.Flush(); err != nil {
		return nil, err
	}

	item, err := parseGetResponse(c, conn.buff)
	if err != nil {
		return nil, err
	}
	item.Key = k
	return item, nil
}
