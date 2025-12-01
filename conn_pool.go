package mc

import (
	"sync"
	"time"
)

func (c *Client) pickServer(key string) (conn *conn, err error) {
	addrs := c.opts.PickServer(key)
	if len(addrs) == 0 {
		return nil, ErrNoServers
	}
	name := addrs[0]
	for _, addr := range addrs {
		if conn, err = c.pool.getConn(addr); err == nil {
			if conn.name != name {
				conn.name = name
			}
			return
		}
	}
	return
}

type pool struct {
	idle            map[string]chan *conn
	dialTimeout     time.Duration
	connMaxLifetime time.Duration
	closeOnce       sync.Once
}

func (p *pool) getConn(addr string) (conn *conn, err error) {
	if idleChan, exists := p.idle[addr]; exists {
		select {
		case conn := <-idleChan:
			return conn, nil
		default:
		}
	}
	if conn, err = openConn(addr, p.dialTimeout); err != nil {
		return nil, err
	}
	return conn, nil
}

func (p *pool) condRelease(conn *conn, err error) {
	if time.Since(conn.connectedAt) >= p.connMaxLifetime {
		conn.close()
		return
	}
	switch err {
	case nil, ErrCacheMiss, ErrNotStored, ErrCASConflict, ErrMalformedKey, errMnDone:
	default:
		conn.close()
		return
	}

	if idleChan, exists := p.idle[conn.name]; exists {
		select {
		case idleChan <- conn:
		default:
			conn.close()
		}
	} else {
		conn.close()
	}
}

func (p *pool) close() error {
	var err error
	p.closeOnce.Do(func() {
		for _, ch := range p.idle {
			close(ch)
			for conn := range ch {
				conn.close()
			}
		}
	})
	return err
}
