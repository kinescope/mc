package mc

import (
	"context"
	"time"

	"github.com/kinescope/mc/internal/clock"
	"github.com/kinescope/mc/proto/cache"
	"github.com/kinescope/mc/protocol"
	"google.golang.org/protobuf/proto"
)

func (c *Client) GetMulti(ctx context.Context, keys ...string) (_ map[string]*Item, retErr error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	keyMap := make(map[string][]string)
	for _, key := range keys {
		if !checkKey(key) {
			return nil, ErrMalformedKey
		}
		addrs := c.opts.PickServer(key)
		if len(addrs) == 0 {
			return nil, ErrNoServers
		}
		keyMap[addrs[0]] = append(keyMap[addrs[0]], key)
	}

	var chs []chan *Item

	for addr, keys := range keyMap {
		ch := make(chan *Item)
		chs = append(chs, ch)
		go func(addr string, keys []string, ch chan *Item) {
			defer close(ch)
			conn, err := c.pool.getConn(addr)
			if err != nil {
				return
			}
			defer c.pool.condRelease(conn, err)
			if deadline, ok := ctx.Deadline(); ok {
				conn.nc.SetDeadline(deadline)
				defer conn.nc.SetDeadline(time.Time{})
			}

			names := make(map[string]string, len(keys))
			for _, k := range keys {
				h := c.opts.KeyHashFunc(k)
				names[string(h)] = k
				err = conn.sendPacket(protocol.GetKQ, h, nil, nil, 0)
				if err != nil {
					return
				}
			}
			if err = conn.sendPacket(protocol.Noop, nil, nil, nil, 0); err != nil {
				return
			}
			var packet *protocol.Packet
			for {
				packet, err = conn.readPacket()
				if err != nil || len(packet.Key) == 0 {
					break
				}
				var (
					flags uint16
					value = packet.Data
				)

				if len(packet.Extras) >= 4 {
					flags = endian.Uint16(packet.Extras[2:])
					if packet.Extras[0] == MagicValue {
						var item cache.Item
						if err := proto.Unmarshal(packet.Data, &item); err != nil {
							continue
						}
						if item.Expiration != nil && item.Expiration.Until < clock.Unix() {
							continue
						}
						if item.Namespace != nil {
							if v, err := c.nsVersion(ctx, item.Namespace.Key, 0); err == nil {
								if v != item.Namespace.Ver {
									c.Delete(ctx, names[string(packet.Key)])
									continue
								}
							}
						}
						value = item.Data
					}
				}
				ch <- &Item{
					Key:   names[string(packet.Key)],
					Value: value,
					Flags: flags,
					cas:   packet.CAS,
				}
			}
		}(addr, keys, ch)
	}
	items := make(map[string]*Item)
	for _, ch := range chs {
		for item := range ch {
			items[item.Key] = item
		}
	}
	return items, nil
}
