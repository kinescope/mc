package mc

import (
	"bufio"
	"bytes"
	"io"
	"strconv"
	"strings"
)

/*
- b: interpret key as base64 encoded binary value
- c: return item cas token
- f: return client flags token
- h: return whether item has been hit before as a 0 or 1
- k: return key as a token
- l: return time since item was last accessed in seconds
- O(token): opaque value, consumes a token and copies back with response
- q: use noreply semantics for return codes.
- s: return item size token
- t: return item TTL remaining in seconds (-1 for unlimited)
- u: don't bump the item in the LRU
- v: return item value in <data block>

These flags can modify the item:
- E(token): use token as new CAS value if item is modified
- N(token): vivify on miss, takes TTL as a argument
- R(token): if remaining TTL is less than token, win for recache
- T(token): update remaining TTL

These extra flags can be added to the response:
- W: client has "won" the recache flag
- X: item is stale
- Z: item has already sent a winning flag
*/
func (c *Client) makeGetCmd(key string, o ...MgOption) []byte {

	// cmd := fmt.Sprintf("mg "+key+" O%d f t c l v b\r\n", keyNum[key])
	var (
		opt mgOpts
		cmd = []byte("mg " + key + " v f")
	)
	for _, fn := range o {
		fn(&opt)
	}
	if opt.cas {
		cmd = append(cmd, ' ', 'c')
	}
	if opt.opaque != 0 {
		cmd = append(cmd, ' ', 'O')
		cmd = strconv.AppendInt(cmd, int64(opt.opaque), 10)
	}

	if !c.opts.DisableBinaryEncodedKeys {
		cmd = append(cmd, []byte(" b")...)
	}

	return cmd
}

func parseGetResponse(buff *bufio.ReadWriter) (*Item, error) {
	line, err := buff.ReadString('\n')
	if err != nil {
		return nil, err
	}
	line = strings.TrimSpace(line)
	switch line {
	case "MN":
		return nil, errMnDone
	case "EN":
		return nil, ErrCacheMiss
	}

	if !strings.HasPrefix(line, "VA ") {
		return nil, ErrCorruptGetResultRead
	}

	fields := strings.Fields(strings.TrimPrefix(line, "VA "))

	size, err := strconv.Atoi(fields[0])
	if err != nil {
		return nil, err
	}
	item := Item{
		Value: make(Value, size+2),
	}

	for _, v := range fields[1:] {
		switch v[0] {
		case 'f': // flags
		case 'c': // cas
			item.cas, err = strconv.ParseUint(v[1:], 10, 0)
		case 't':
		case 'O':
			var o uint64
			if o, err = strconv.ParseUint(v[1:], 10, 0); err == nil {
				item.opaque = int(o)
			}
		}
		if err != nil {
			return nil, err
		}
	}

	if _, err := io.ReadFull(buff, item.Value); err != nil {
		return nil, err
	}
	if !bytes.HasSuffix(item.Value, crlf) {
		return nil, ErrCorruptGetResultRead
	}

	item.Value = item.Value[:size]

	return &item, nil
}

func parseResponse(buff *bufio.Reader) (*Value, error) {
	line, err := buff.ReadSlice('\n')
	if err != nil {
		return nil, err
	}
	switch line := strings.TrimSpace(string(line)); line {
	case "HD":
		return nil, nil
	case "NS":
		return nil, ErrNotStored
	case "NF":
		return nil, ErrCacheMiss
	case "EX":
		return nil, ErrCASConflict
	case "ERROR":
		return nil, ErrNonexistentCommandName
	}

	return nil, nil
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
