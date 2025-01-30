package mc

import (
	"fmt"
	"io"
	"strconv"
	"strings"
)

func (c *Client) arithmetic(op, k string, delta uint64, expiration uint32, o ...MaOption) (new uint64, _ error) {
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

	//fmt.Println(key, string(cmd))
	conn.buff.Write(append(cmd, crlf...))

	if err := conn.buff.Flush(); err != nil {
		return 0, err
	}

	line, err := conn.buff.ReadString('\n')
	if err != nil {
		return 0, err
	}

	switch line = strings.TrimSpace(line); {
	case line == "NF": //not found
		return 0, ErrCacheMiss
	case strings.HasPrefix(line, "CLIENT_ERROR "):
		msg := strings.TrimPrefix(line, "CLIENT_ERROR ")
		if msg == "cannot increment or decrement non-numeric value" {
			return 0, ErrBadIncrDec
		}
		return 0, &ClientError{
			Message: msg,
		}
	case strings.HasPrefix(line, "VA "):
		fields := strings.Fields(line)
		if len(fields) < 2 {
			return 0, fmt.Errorf("invalid sever response: %s", line)
		}

		ln, err := strconv.Atoi(fields[1])
		if err != nil {
			return 0, err
		}

		value := make(Value, ln)

		if _, err := io.ReadFull(conn.buff, value); err != nil {
			return 0, err
		}

		return strconv.ParseUint(string(value), 10, 64)
	}
	return 0, nil

}
