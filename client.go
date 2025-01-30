package mc

// https://docs.memcached.org/protocols/meta/
import (
	"bytes"
	"fmt"
	"io"
	"strconv"
	"strings"
)

var crlf = []byte("\r\n")

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
	key, err := c.encodeKey(k)
	if err != nil {
		return nil, err
	}

	//fmt.Println(k)
	conn, err := c.pickServer(key)
	if err != nil {
		return nil, err
	}

	defer func() {
		c.pool.condRelease(conn, retErr)
	}()

	cmd := "mg " + key + " f t c l v b\r\n"

	conn.buff.Write([]byte(cmd))

	conn.buff.Flush()

	line, err := conn.buff.ReadString('\n')
	if err != nil {
		return nil, err
	}

	switch line = strings.TrimSpace(line); {
	case line == "EN": //not found
		return nil, ErrCacheMiss
	case strings.HasPrefix(line, "VA "):
		fields := strings.Fields(strings.TrimPrefix(line, "VA "))
		ln, err := strconv.Atoi(fields[0])
		if err != nil {
			return nil, err
		}

		item := Item{
			Key:   k,
			Value: make(Value, ln+2),
		}

		for _, v := range fields[1:] {
			switch v[0] {
			case 'f': // flags
			case 'c': // cas
				item.cas, err = strconv.ParseUint(v[1:], 10, 0)
			case 't':
				/*ttl, err := strconv.ParseUint(v[1:], 10, 32)
				if err != nil {
					return nil, err
				}
				fmt.Println("TTL", ttl)*/
			}
			if err != nil {
				return nil, err
			}
			//fmt.Println("VVV", v)
		}

		if _, err := io.ReadFull(conn.buff, item.Value); err != nil {
			return nil, err
		}
		if !bytes.HasSuffix(item.Value, crlf) {
			return nil, ErrCorruptGetResultRead
		}
		item.Value = item.Value[:ln]
		return &item, nil
	}
	//fmt.Println(line)
	return &Item{}, nil
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
func (c *Client) populateOne(mode string, i *Item, cas uint64, o ...MsOption) (err error) {
	var opts msOpts

	for _, fn := range o {
		fn(&opts)
	}

	if opts.minUses != 0 {
		if v, _ := c.Inc(i.Key+"::_min_uses", 1, opts.expiration, WithInitialValue(1)); v < opts.minUses {
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
	//fmt.Println(i.Key)
	cmd := fmt.Sprintf("ms %s %d M%s T%d F%d b", key, len(i.Value), mode, 30, i.Flags)

	if cas != 0 {
		cmd += fmt.Sprintf(" C%d", cas)
	}

	//	fmt.Println(cmd)

	conn.buff.Write([]byte(cmd))
	conn.buff.Write(crlf)

	conn.buff.Write(i.Value)

	conn.buff.Write(crlf)

	conn.buff.Flush()

	if _, err := parseResponse(conn.buff.Reader); err != nil {
		return err
	}
	return nil
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
	line, err := conn.buff.ReadString('\n')
	if err != nil {
		return err
	}

	switch line = strings.TrimSpace(line); line {
	case "HD": // ok
	case "NS":
		return ErrNotStored
	case "NF":
		return ErrCacheMiss
	case "EX":
		return ErrCASConflict
	case "ERROR":
		return ErrNonexistentCommandName
	default:
		switch {
		case strings.HasPrefix(line, "CLIENT_ERROR "):
			return &ClientError{
				Message: strings.TrimPrefix(line, "CLIENT_ERROR "),
			}
		case strings.HasPrefix(line, "SERVER_ERROR "):
			return &ClientError{
				Message: strings.TrimPrefix(line, "SERVER_ERROR "),
			}
		}
	}

	return nil
}

func (c *Client) Inc(k string, delta uint64, expiration uint32, o ...MaOption) (new uint64, _ error) {
	return c.arithmetic("M+", k, delta, expiration, o...)
}
func (c *Client) Dec(k string, delta uint64, expiration uint32, o ...MaOption) (new uint64, _ error) {
	return c.arithmetic("M-", k, delta, expiration, o...)
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
