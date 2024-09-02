package mc

import (
	"context"
	"time"

	"github.com/kinescope/mc/protocol"
)

func (c *Client) request(ctx context.Context, opcode protocol.Opcode, key string, data, extras []byte, cas uint64) (_ []byte, _ []byte, _ uint64, retErr error) {
	select {
	case <-ctx.Done():
		return nil, nil, 0, ctx.Err()
	default:
	}
	if !checkKey(key) {
		return nil, nil, 0, ErrMalformedKey
	}
	conn, err := c.pickServer(key)
	if err != nil {
		return nil, nil, 0, err
	}
	defer func() {
		c.pool.condRelease(conn, retErr)
	}()
	if deadline, ok := ctx.Deadline(); ok {
		conn.nc.SetDeadline(deadline)
		defer conn.nc.SetDeadline(time.Time{})
	}
	if err := conn.sendPacket(opcode, c.opts.KeyHashFunc(key), data, extras, cas); err != nil {
		return nil, nil, 0, err
	}
	packet, err := conn.readPacket()
	if err != nil {
		return nil, nil, 0, err
	}
	return packet.Data, packet.Extras, packet.CAS, nil
}

func checkKey(key string) bool {
	if len(key) > 250 {
		return false
	}
	for i := 0; i < len(key); i++ {
		if key[i] <= ' ' || key[i] > 0x7e {
			return false
		}
	}
	return true
}
