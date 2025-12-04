package mc

import (
	"context"
	"errors"
	"sort"
	"time"
)

// GetMulti retrieves multiple items from the cache in a single operation.
// Keys are broadcast to all servers from PickServer, and responses are validated
// to ensure keys come from a valid server (by hash). This handles cases where
// Set wrote to alternative servers when primary was unavailable. Only keys from
// valid servers (according to PickServer) are accepted to avoid stale data.
// Returns a map of found items (keys that don't exist are not included).
// This is more efficient than multiple Get() calls, especially when
// keys are on different servers.
func (c *Client) GetMulti(ctx context.Context, keys []string, o ...MgOption) (_ map[string]*Item, retErr error) {
	if len(keys) == 0 {
		return make(map[string]*Item), nil
	}

	var (
		// All unique servers
		allServers = make(map[string]bool)
		// Original keys for restoration
		allKeys = make([]string, len(keys))
		// Map: encoded key -> opaque value
		keyNum = make(map[string]int, len(keys))
		// Map: encoded key -> set of valid servers (from PickServer)
		keyToValidServers = make(map[string]map[string]bool, len(keys))
		// Map: original key -> encoded key (for fast lookup)
		originalToEncoded = make(map[string]string, len(keys))
		// Map: server -> all encoded keys (for broadcasting)
		serverToKeys = make(map[string][]string)
	)

	for n, k := range keys {
		key, err := c.encodeKey(k)
		if err != nil {
			return nil, err
		}
		addrs := c.opts.PickServer(key)
		if len(addrs) == 0 {
			return nil, ErrNoServers
		}
		opaque := n + 1
		keyNum[key] = opaque
		originalToEncoded[k] = key
		allKeys[n] = k

		// Build set of valid servers for this key
		validServers := make(map[string]bool, len(addrs))
		for _, addr := range addrs {
			validServers[addr] = true
			serverToKeys[addr] = append(serverToKeys[addr], key)
			allServers[addr] = true
		}
		keyToValidServers[key] = validServers
	}

	var (
		opt mgOpts
		chs []chan *Item
	)
	for _, fn := range o {
		fn(&opt)
	}

	// Use minimum of opt.deadline and context.Deadline() if both are set
	deadline := opt.deadline
	if ctxDeadline, ok := ctx.Deadline(); ok {
		if deadline.IsZero() || ctxDeadline.Before(deadline) {
			deadline = ctxDeadline
		}
	}

	// Build list of all encoded keys for broadcasting (in order)
	allEncodedKeys := make([]string, 0, len(keys))
	for _, k := range keys {
		if encodedKey, exists := originalToEncoded[k]; exists {
			allEncodedKeys = append(allEncodedKeys, encodedKey)
		}
	}

	// Build sorted list of servers for deterministic iteration
	serverList := make([]string, 0, len(allServers))
	for addr := range allServers {
		serverList = append(serverList, addr)
	}
	// Sort for deterministic order (important for Go 1.25+)
	sort.Strings(serverList)

	// Send all keys to each server (broadcast to handle cases where Set wrote to alternative servers)
	for _, addr := range serverList {
		ch := make(chan *Item)
		chs = append(chs, ch)
		go func(addr string, ch chan *Item, keyNumMap map[string]int, allKeys []string, keyToValid map[string]map[string]bool, origToEncoded map[string]string, allEncodedKeys []string) {
			defer close(ch)
			localOpt := opt

			conn, err := c.pool.getConn(addr)
			if err != nil {
				return
			}
			if !deadline.IsZero() {
				conn.nc.SetDeadline(deadline)
			}
			defer func() {
				if !deadline.IsZero() {
					conn.nc.SetDeadline(time.Time{})
				}
				c.pool.condRelease(conn, err)
			}()

			// Check context before operations
			select {
			case <-ctx.Done():
				return
			default:
			}

			// Send all mg commands (broadcast ALL keys to this server)
			// This ensures we find keys even if Set wrote to alternative servers
			for _, key := range allEncodedKeys {
				localOpt.opaque = keyNumMap[key]
				cmd := c.makeGetCmd(key, localOpt)
				conn.buff.Write(cmd)
				conn.buff.Write(crlf)
			}

			// Send mn command to signal end of batch
			conn.buff.WriteString("mn\r\n")
			if err := conn.buff.Flush(); err != nil {
				return
			}

			// Read responses until we get "MN" (end of multi-get)
			var item *Item
			for {
				if item, err = parseGetResponse(ctx, c, conn.buff); err == nil {
					// Restore original key using opaque value
					if item.opaque > 0 && item.opaque <= len(allKeys) {
						originalKey := allKeys[item.opaque-1]
						// Get encoded key from precomputed map
						if encodedKey, exists := origToEncoded[originalKey]; exists {
							// Only accept key if it came from a valid server (from PickServer for this key)
							if validServers, exists := keyToValid[encodedKey]; exists && validServers[addr] {
								item.Key = originalKey
								ch <- item
							}
							// If key came from invalid server, ignore it (might be stale or wrong)
						}
					}
					continue
				}
				// Check for end of multi-get (MN response)
				if errors.Is(err, errMnDone) {
					break
				}
				// ErrCacheMiss is expected for non-existent keys
				if errors.Is(err, ErrCacheMiss) {
					continue
				}
				// Any other error is unexpected, stop reading
				return
			}
		}(addr, ch, keyNum, allKeys, keyToValidServers, originalToEncoded, allEncodedKeys)
	}

	items := make(map[string]*Item, len(keys))
	for _, ch := range chs {
		for item := range ch {
			items[item.Key] = item
		}
	}
	return items, nil
}
