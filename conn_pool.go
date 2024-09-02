package mc

import (
	"time"
)

func (c *Client) pickServer(key string) (conn *conn, err error) {
	addrs := c.opts.PickServer(key)
	if len(addrs) == 0 {
		return nil, ErrNoServers
	}
	for _, addr := range addrs {
		if conn, err = c.pool.getConn(addr); err == nil {
			return
		}
	}
	return
}

type pool struct {
	idle            map[string]chan *conn
	dialTimeout     time.Duration
	connMaxLifetime time.Duration
}

func (p *pool) getConn(addr string) (conn *conn, err error) {
	select {
	case conn := <-p.idle[addr]:
		return conn, nil
	default:
	}
	if conn, err = openConn(addr, p.dialTimeout); err != nil {
		return nil, err
	}
	return conn, nil
}

func (p *pool) condRelease(conn *conn, err error) {
	conn.packet.Reset()
	if time.Since(conn.connectedAt) >= p.connMaxLifetime {
		conn.close()
		return
	}

	switch err {
	case nil, ErrCacheMiss, ErrNotStored, ErrBadIncrDec, ErrCASConflict:
	default:
		conn.close()
		return
	}

	select {
	case p.idle[conn.addr] <- conn:
	default:
		conn.close()
	}
}
