package mc

import (
	"bytes"
	"fmt"
	"io"
	"strconv"
	"strings"
)

func (c *Client) GetMulti(keys ...string) (_ map[string]*Item, retErr error) {
	keyNum := make(map[string]int)
	keyMap := make(map[string][]string)
	for n, k := range keys {
		key, err := c.encodeKey(k)
		if err != nil {
			return nil, err
		}
		addrs := c.opts.PickServer(key)
		if len(addrs) == 0 {
			return nil, ErrNoServers
		}
		keyNum[key] = n
		keyMap[addrs[0]] = append(keyMap[addrs[0]], key)
	}

	var chs []chan *Item

	for addr, items := range keyMap {
		ch := make(chan *Item)
		chs = append(chs, ch)
		go func(addr string, items []string, ch chan *Item) (retErr error) {
			defer close(ch)
			conn, err := c.pool.getConn(addr)
			if err != nil {
				return nil
			}
			defer c.pool.condRelease(conn, retErr)

			for _, key := range items {
				cmd := fmt.Sprintf("mg "+key+" O%d f t c l v b\r\n", keyNum[key])
				conn.buff.WriteString(cmd)
			}

			conn.buff.WriteString("mn\r\n")
			conn.buff.Flush()

			for range len(items) + 1 {
				line, err := conn.buff.ReadString('\n')
				if err != nil {
					return err
				}
				switch line = strings.TrimSpace(line); {
				case line == "MN":
					return nil
				case strings.HasPrefix(line, "VA "):
					fields := strings.Fields(strings.TrimPrefix(line, "VA "))
					ln, err := strconv.Atoi(fields[0])
					if err != nil {
						return err
					}

					item := Item{
						Value: make(Value, ln+2),
					}

					for _, v := range fields[1:] {
						switch v[0] {
						case 'f': // flags
						case 'c': // cas
							item.cas, err = strconv.ParseUint(v[1:], 10, 0)
						case 't':
						case 'O':
							num, _ := strconv.ParseUint(v[1:], 10, 0)
							item.Key = keys[num]
						}
						if err != nil {
							return err
						}
						//fmt.Println("VVV", v)
					}

					if _, err := io.ReadFull(conn.buff, item.Value); err != nil {
						return
					}
					if !bytes.HasSuffix(item.Value, crlf) {
						return
					}
					item.Value = item.Value[:ln]

					ch <- &item
				}

			}

			return nil

		}(addr, items, ch)
	}
	items := make(map[string]*Item)
	for _, ch := range chs {
		for item := range ch {
			items[item.Key] = item
		}
	}
	return items, nil

}
