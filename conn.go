package mc

import (
	"bufio"
	"net"
	"time"
)

func openConn(addr string, timeout time.Duration) (*conn, error) {
	c, err := net.DialTimeout("tcp", addr, timeout)
	if err != nil {
		return nil, err
	}
	conn := &conn{
		nc:   c,
		name: addr,
		buff: bufio.NewReadWriter(
			bufio.NewReader(c),
			bufio.NewWriter(c),
		),
		connectedAt: time.Now(),
	}
	version, err := conn.version()
	if err != nil {
		return nil, err
	}
	if !version.GE(minVersion) {
		conn.close()
		return nil, ErrUnsupportedServerVersion
	}
	return conn, nil
}

type conn struct {
	nc          net.Conn
	name        string
	buff        *bufio.ReadWriter
	connectedAt time.Time
}

func (c *conn) close() error { return c.nc.Close() }
