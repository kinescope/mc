package mc

import (
	"strconv"
)

func (c *Client) Inc(k string, delta uint64, expiration uint32, o ...MaOption) (new uint64, _ error) {
	return c.arithmetic("M+", k, delta, expiration, o...)
}
func (c *Client) Dec(k string, delta uint64, expiration uint32, o ...MaOption) (new uint64, _ error) {
	return c.arithmetic("M-", k, delta, expiration, o...)
}

func (c *Client) arithmetic(op, k string, delta uint64, expiration uint32, o ...MaOption) (new uint64, retErr error) {
	var opts maOpts
	for _, fn := range o {
		fn(&opts)
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

	cmd := []byte("ma " + key + " " + op + " v D")
	cmd = strconv.AppendUint(cmd, delta, 10)

	if opts.initialValue > 0 {
		cmd = append(cmd, []byte(" N0 J")...)
		cmd = strconv.AppendUint(cmd, opts.initialValue, 10)
	}

	if expiration > 0 {
		cmd = append(cmd, []byte(" T")...)
		cmd = strconv.AppendUint(cmd, uint64(expiration), 10)
	}

	if !c.opts.DisableBinaryEncodedKeys {
		cmd = append(cmd, []byte(" b")...)
	}

	conn.buff.Write(append(cmd, crlf...))

	if err := conn.buff.Flush(); err != nil {
		return 0, err
	}

	item, err := parseGetResponse(c, conn.buff)
	if err != nil {
		return 0, err
	}

	return strconv.ParseUint(string(item.Value), 10, 64)
}
