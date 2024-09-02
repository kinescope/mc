package mc

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/kinescope/mc/internal/clock"
	"github.com/kinescope/mc/proto/cache"
	"github.com/kinescope/mc/protocol"
	"google.golang.org/protobuf/proto"
)

const MagicValue = 0x42

var endian = binary.BigEndian

func New(o *Options) (_ *Client, err error) {
	o.setDefaults()
	if len(o.Addrs) == 0 {
		return nil, ErrNoServers
	}
	var (
		cli = &Client{
			opts: o,
			pool: pool{
				idle:            make(map[string]chan *conn),
				dialTimeout:     o.DialTimeout,
				connMaxLifetime: o.ConnMaxLifetime,
			},
		}
	)
	for _, s := range o.Addrs {
		cli.pool.idle[s] = make(chan *conn, o.MaxIdleConnsPerAddr)
	}
	return cli, nil
}

type Client struct {
	pool pool
	opts *Options
}

func (c *Client) Get(ctx context.Context, key string) (*Item, error) {
	data, extra, cas, err := c.request(ctx, protocol.Get, key, nil, nil, 0)
	if err != nil {
		return nil, err
	}

	var (
		flags uint16
		value = data
	)

	if len(extra) >= 4 {
		flags = endian.Uint16(extra[2:])
		if extra[0] == MagicValue {

			var item cache.Item
			if err := proto.Unmarshal(data, &item); err != nil {
				return nil, err
			}

			if item.Expiration != nil && item.Expiration.Until < clock.Unix() {
				err := c.Add(ctx, &Item{
					Key: key + ":es",
				}, WithExpiration(item.Expiration.Scale, 0))
				switch err {
				case nil:
					return nil, ErrCacheMiss
				case ErrAlreadyExists:
				default:
					return nil, err
				}
			}

			if item.Namespace != nil {
				v, err := c.nsVersion(ctx, item.Namespace.Key, 0)
				if err != nil {
					return nil, err
				}
				if v != item.Namespace.Ver {
					c.Delete(ctx, key)
					return nil, ErrCacheMiss
				}
			}

			value = item.Data
		}
	}

	return &Item{
		Key:   key,
		Value: value,
		Flags: flags,
		cas:   cas,
	}, nil
}

func (c *Client) Set(ctx context.Context, i *Item, o ...Option) error {
	return c.populateOne(ctx, protocol.Set, i, 0, o...)
}
func (c *Client) Add(ctx context.Context, i *Item, o ...Option) error {
	return c.populateOne(ctx, protocol.Add, i, 0, o...)
}

func (c *Client) CompareAndSwap(ctx context.Context, i *Item, o ...Option) error {
	if err := c.populateOne(ctx, protocol.Set, i, i.cas, o...); err != nil {
		if errors.Is(err, ErrAlreadyExists) {
			return ErrCASConflict
		}
		return err
	}
	return nil
}

func (c *Client) Inc(ctx context.Context, key string, delta uint64, o ...Option) (uint64, error) {
	var opt opts
	for _, fn := range o {
		fn(&opt)
	}
	return c.incrDecr(ctx, protocol.Increment, key, delta, opt.initial, opt.expiration)
}

func (c *Client) Dec(ctx context.Context, key string, delta uint64) (uint64, error) {
	return c.incrDecr(ctx, protocol.Decrement, key, delta, 0, 0)
}

func (c *Client) Delete(ctx context.Context, key string) error {
	if _, _, _, err := c.request(ctx, protocol.Delete, key, nil, nil, 0); err != nil {
		return err
	}
	return nil
}

// https://github.com/memcached/memcached/wiki/ProgrammingTricks#namespacing
func (c *Client) PurgeNamespace(ctx context.Context, ns string) error {
	ns = fmt.Sprintf("%x", XXKeyHashFunc(ns))
	if _, err := c.nsVersion(ctx, ns, 1); err != nil {
		return err
	}
	return nil
}

func (c *Client) incrDecr(ctx context.Context, opcode protocol.Opcode, key string, delta, initial uint64, expiration uint32) (uint64, error) {
	extras := make([]byte, 20)
	switch {
	case initial > 0:
		endian.PutUint64(extras[8:16], initial)
		endian.PutUint32(extras[16:20], expiration)
	default:
		copy(extras[16:], []byte{
			0xff,
			0xff,
			0xff,
			0xff,
		})
	}
	endian.PutUint64(extras, delta)

	data, _, _, err := c.request(ctx, opcode, key, nil, extras, 0)
	if err != nil {
		return 0, err
	}
	return endian.Uint64(data), nil
}

func (c *Client) populateOne(ctx context.Context, opcode protocol.Opcode, i *Item, cas uint64, o ...Option) (err error) {
	var opt opts
	for _, fn := range o {
		fn(&opt)
	}
	if opt.minUses > 0 {
		var (
			key        = i.Key + ":muc"
			expiration = opt.expiration
		)
		if expiration == 0 || expiration > 1_800 {
			expiration = 1_800
		}
		switch uses, err := c.incrDecr(ctx, protocol.Increment, key, 1, 1, expiration); {
		case err != nil:
			return err
		case uses < uint64(opt.minUses):
			return nil
		}
	}
	scaled := opt.expiration + opt.scalingExpiration
	extras := make([]byte, 8)
	endian.PutUint16(extras[2:4], i.Flags) //uint16 flags
	if opt.expiration != 0 {
		endian.PutUint32(extras[4:8], uint32(scaled))
	}
	value := i.Value

	if (opt.expiration != 0 && opt.scalingExpiration != 0) || len(opt.namespace) != 0 {
		extras[0] = MagicValue
		item := &cache.Item{
			Data: i.Value,
		}
		if opt.scalingExpiration != 0 {
			item.Expiration = &cache.Expiration{
				Scale: opt.scalingExpiration,
				Until: clock.Unix() + int64(opt.expiration),
			}
		}
		if len(opt.namespace) != 0 {
			ns := fmt.Sprintf("%x", XXKeyHashFunc(opt.namespace))
			ver, err := c.nsVersion(ctx, ns, 0)
			if err != nil {
				return err
			}
			item.Namespace = &cache.Namespace{
				Key: ns,
				Ver: ver,
			}
		}
		if value, err = proto.Marshal(item); err != nil {
			return err
		}
	}

	if _, _, i.cas, err = c.request(ctx, opcode, i.Key, value, extras, cas); err != nil {
		return err
	}
	return nil
}

func (c *Client) nsVersion(ctx context.Context, ns string, delta uint64) (uint64, error) {
	return c.Inc(ctx, ns+":ns", delta, WithInitial(uint64(clock.Unix())))
}
