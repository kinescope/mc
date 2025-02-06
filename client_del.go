package mc

import "strconv"

func (c *Client) Del(k string, o ...MdOption) (retErr error) {
	var opts mdOpts
	for _, fn := range o {
		fn(&opts)
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
