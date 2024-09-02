package mc

import (
	"net"
	"time"

	"github.com/kinescope/mc/protocol"
)

func openConn(addr string, timeout time.Duration) (*conn, error) {
	c, err := net.DialTimeout("tcp", addr, timeout)
	if err != nil {
		return nil, err
	}
	return &conn{
		nc:          c,
		addr:        addr,
		connectedAt: time.Now(),
	}, nil
}

type conn struct {
	nc          net.Conn
	addr        string
	packet      protocol.Packet
	connectedAt time.Time
}

func (c *conn) sendPacket(opcode protocol.Opcode, key, data, extras []byte, cas uint64) error {
	c.packet.Reset()
	{
		c.packet.CAS = cas
		c.packet.Key = key
		c.packet.Data = data
		c.packet.Extras = extras
		c.packet.Opcode = opcode
	}
	return checkError(c.packet.Write(c.nc))
}

func (c *conn) readPacket() (*protocol.Packet, error) {
	c.packet.Reset()
	if err := c.packet.Read(c.nc); err != nil {
		return nil, checkError(err)
	}
	return &c.packet, nil
}

func (c *conn) close() error { return c.nc.Close() }
