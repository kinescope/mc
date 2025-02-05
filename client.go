package mc

// https://docs.memcached.org/protocols/meta/
import (
	"encoding/binary"
	"strconv"
	"time"
)

var crlf = []byte("\r\n")

var endian = binary.BigEndian

const (
	compressed = 2
	serialized = 4
)

func New(opts *Options) (*Client, error) {
	opts.setDefaults()
	if len(opts.Addrs) == 0 {
		return nil, ErrNoServers
	}
	var (
		cli = &Client{
			opts: opts,
			pool: pool{
				idle:            make(map[string]chan *conn),
				dialTimeout:     opts.DialTimeout,
				connMaxLifetime: opts.ConnMaxLifetime,
			},
			encodeKey: binaryEncodeKey,
		}
	)
	if opts.DisableBinaryEncodedKeys {
		cli.encodeKey = func(s string) (string, error) {
			if !checkKey(s) {
				return "", ErrMalformedKey
			}
			return s, nil
		}
	}
	for _, s := range opts.Addrs {
		cli.pool.idle[s] = make(chan *conn, opts.MaxIdleConnsPerAddr)
	}
	return cli, nil
}

type Client struct {
	pool      pool
	opts      *Options
	encodeKey func(string) (string, error)
}

func (c *Client) Get(k string, o ...MgOption) (_ *Item, retErr error) {
	var (
		opt      mgOpts
		key, err = c.encodeKey(k)
	)

	if err != nil {
		return nil, err
	}

	conn, err := c.pickServer(key)
	if err != nil {
		return nil, err
	}

	for _, fn := range o {
		fn(&opt)
	}

	if !opt.deadline.IsZero() {
		conn.nc.SetDeadline(opt.deadline)
	}

	defer func() {
		if !opt.deadline.IsZero() {
			conn.nc.SetDeadline(time.Time{})
		}
		c.pool.condRelease(conn, retErr)
	}()

	conn.buff.Write(append(c.makeGetCmd(key, opt), crlf...))

	if err := conn.buff.Flush(); err != nil {
		return nil, err
	}

	item, err := parseGetResponse(conn.buff)
	if err != nil {
		return nil, err
	}
	item.Key = k
	return item, nil

}

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

func (c *Client) Del(k string, o ...MdOption) error {
	var opts mdOpts
	for _, fn := range o {
		fn(&opts)
	}
	key, err := c.encodeKey(k)
	if err != nil {
		return err
	}
	cmd := []byte("md " + key)
	if !c.opts.DisableBinaryEncodedKeys {
		cmd = append(cmd, []byte(" b")...)
	}
	conn, err := c.pickServer(key)
	if err != nil {
		return err
	}

	conn.buff.Write(cmd)
	conn.buff.Write(crlf)

	if err := conn.buff.Flush(); err != nil {
		return err
	}
	return parseResponse(conn.buff.Reader)
}

func (c *Client) Inc(k string, delta uint64, expiration uint32, o ...MaOption) (new uint64, _ error) {
	return c.arithmetic("M+", k, delta, expiration, o...)
}
func (c *Client) Dec(k string, delta uint64, expiration uint32, o ...MaOption) (new uint64, _ error) {
	return c.arithmetic("M-", k, delta, expiration, o...)
}
