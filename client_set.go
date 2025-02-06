package mc

import (
	"strconv"

	"github.com/kinescope/mc/proto/cache"
)

func (c *Client) Add(i *Item, o ...MsOption) error {
	return c.populateOne("E", i, 0, o...)
}

func (c *Client) Set(i *Item, o ...MsOption) error {
	return c.populateOne("S", i, 0, o...)
}

func (c *Client) CompareAndSwap(i *Item, o ...MsOption) error {
	return c.populateOne("S", i, i.cas, o...)
}

/*

- b: interpret key as base64 encoded binary value (see metaget)
- c: return CAS value if successfully stored.
- C(token): compare CAS value when storing item
- E(token): use token as new CAS value (see metaget for detail)
- F(token): set client flags to token (32 bit unsigned numeric)
- I: invalidate. set-to-invalid if supplied CAS is older than item's CAS
- k: return key as a token
- O(token): opaque value, consumes a token and copies back with response
- q: use noreply semantics for return codes
- s: return the size of the stored item on success (ie; new size on append)
- T(token): Time-To-Live for item, see "Expiration" above.
- M(token): mode switch to change behavior to add, replace, append, prepend
- N(token): if in append mode, autovivify on miss with supplied TTL

E: "add" command. LRU bump and return NS if item exists. Else
add.
A: "append" command. If item exists, append the new value to its data.
P: "prepend" command. If item exists, prepend the new value to its data.
R: "replace" command. Set only if item already exists.
S: "set" command. The default mode, added for completeness.
*/

// https://github.com/memcached/memcached/blob/master/doc/protocol.txt#L685
func (c *Client) populateOne(mode string, i *Item, cas uint64, o ...MsOption) (retErr error) {
	if len(i.Value) == 0 {
		return ErrEmptyValue
	}

	var opts msOpts

	for _, fn := range o {
		fn(&opts)
	}

	if opts.minUses != 0 {
		if v, err := c.Inc(i.Key+"::_min_uses", 1, opts.expiration, WithInitialValue(1)); err == nil && v < opts.minUses {
			return nil
		}
	}

	key, err := c.encodeKey(i.Key)
	if err != nil {
		return err
	}
	conn, err := c.pickServer(key)
	if err != nil {
		return err
	}
	defer c.pool.condRelease(conn, retErr)

	var (
		flags  int
		source [4]byte
	)

	if len(opts.namespace) != 0 {
		flags |= serialized
		ver, err := c.nsVersion(opts.namespace, 0)
		if err != nil {
			return err
		}
		i.Value, err = (&cache.Item{
			Data: i.Value,
			Namespace: &cache.Namespace{
				Key: opts.namespace,
				Ver: ver,
			},
		}).Marshal()
		if err != nil {
			return err
		}
	}

	if opts.compressionMinLen != 0 && len(i.Value) > opts.compressionMinLen {
		flags |= compressed
		if i.Value, err = compress(i.Value); err != nil {
			return err
		}
	}

	cmd := []byte("ms " + key + " ")
	cmd = strconv.AppendInt(cmd, int64(len(i.Value)), 10)
	cmd = append(append(cmd, ' ', 'M'), []byte(mode)...)
	if opts.expiration != 0 {
		cmd = append(cmd, ' ', 'T')
		cmd = strconv.AppendUint(cmd, uint64(opts.expiration), 10)
	}

	if i.Flags != 0 || flags != 0 {

		endian.PutUint16(source[:2], uint16(flags))
		endian.PutUint16(source[2:], uint16(i.Flags))

		cmd = append(cmd, ' ', 'F')
		cmd = strconv.AppendUint(cmd, uint64(endian.Uint32(source[:])), 10)
	}

	if !c.opts.DisableBinaryEncodedKeys {
		cmd = append(cmd, ' ', 'b')
	}
	if cas != 0 {
		cmd = append(cmd, ' ', 'C')
		cmd = strconv.AppendUint(cmd, cas, 10)
	}

	conn.buff.Write(append(cmd, crlf...))
	conn.buff.Write(append(i.Value, crlf...))

	if err := conn.buff.Flush(); err != nil {
		return err
	}

	return parseResponse(conn.buff.Reader)
}
