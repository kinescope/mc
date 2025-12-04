package mc

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"strconv"
	"strings"

	"github.com/kinescope/mc/proto/cache"
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
func (c *Client) makeGetCmd(key string, opt mgOpts) []byte {
	cmd := []byte("mg " + key + " v f")
	if opt.cas {
		cmd = append(cmd, ' ', 'c')
	}

	if opt.hit {
		cmd = append(cmd, ' ', 'h')
	}

	if opt.lastAccess {
		cmd = append(cmd, ' ', 'l')
	}

	if opt.opaque != 0 {
		cmd = append(cmd, ' ', 'O')
		cmd = strconv.AppendInt(cmd, int64(opt.opaque), 10)
	}

	if opt.earlyRecache > 0 {
		cmd = append(cmd, ' ', 'R')
		cmd = strconv.AppendInt(cmd, int64(opt.earlyRecache), 10)
	}

	// Request key in response as fallback for key restoration (critical for CI)
	if opt.returnKey {
		cmd = append(cmd, ' ', 'k')
	}

	if !c.opts.DisableBinaryEncodedKeys {
		cmd = append(cmd, ' ', 'b')
	}

	return cmd
}

func parseGetResponse(ctx context.Context, c *Client, buff *bufio.ReadWriter) (*Item, error) {
	line, err := buff.ReadString('\n')
	if err != nil {
		return nil, err
	}
	line = strings.TrimSpace(line)
	switch line {
	case "MN":
		return nil, errMnDone
	case "EN", "NF":
		return nil, ErrCacheMiss
	case "ERROR":
		return nil, ErrNonexistentCommandName
	default:
		switch {
		case strings.HasPrefix(line, "CLIENT_ERROR "):
			msg := strings.TrimPrefix(line, "CLIENT_ERROR ")
			if msg == "cannot increment or decrement non-numeric value" {
				return nil, ErrBadIncrDec
			}
			return nil, &ClientError{
				Message: msg,
			}
		case strings.HasPrefix(line, "SERVER_ERROR "):
			return nil, &ClientError{
				Message: strings.TrimPrefix(line, "SERVER_ERROR "),
			}
		}
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

	var (
		_compressed bool
		_serialized bool
	)

	for _, v := range fields[1:] {
		switch v[0] {
		case 'W':
			item.won = true
		case 'X':
			item.stale = true
		case 'f': // flags
			v, err := strconv.ParseUint(v[1:], 10, 0)
			if err != nil {
				return nil, err
			}
			var source [4]byte
			endian.PutUint32(source[:], uint32(v))

			item.Flags = endian.Uint16(source[2:])
			{
				f := endian.Uint16(source[:2])
				{
					_compressed = f&compressed == compressed
					_serialized = f&serialized == serialized
				}
			}

		case 'c': // cas
			item.cas, err = strconv.ParseUint(v[1:], 10, 0)
		case 'l':
			item.lastAccess, err = strconv.ParseInt(v[1:], 10, 0)
		case 'h':
			item.hit = v[1:2] == "1"
		case 'O':
			var o uint64
			if o, err = strconv.ParseUint(v[1:], 10, 0); err == nil {
				item.opaque = int(o)
			}
		case 'k':
			// Key is returned as a token after 'k'
			if len(v) > 1 {
				item.Key = v[1:]
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

	if _compressed {
		size := float32(len(item.Value))
		if item.Value, err = c.decompress(item.Value); err != nil {
			return nil, err
		}
		item.cr = size / float32(len(item.Value))
	}

	if _serialized {
		var p cache.Item
		if err := p.Unmarshal(item.Value); err != nil {
			return nil, err
		}
		item.Value = p.Data

		ver, err := c.nsVersion(ctx, p.Namespace.Key, 0)
		if err != nil {
			return nil, err
		}

		if ver != p.Namespace.Ver {
			return nil, ErrCacheMiss
		}
	}

	return &item, nil
}

func parseResponse(buff *bufio.Reader) error {
	line, err := buff.ReadSlice('\n')
	if err != nil {
		return err
	}
	switch line := strings.TrimSpace(string(line)); line {
	case "HD":
		return nil
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
