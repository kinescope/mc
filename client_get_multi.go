package mc

import (
	"context"
	"errors"
	"fmt"
	"os"
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
// Set MC_DEBUG=1 environment variable to enable detailed debug logging.
func (c *Client) GetMulti(ctx context.Context, keys []string, o ...MgOption) (_ map[string]*Item, retErr error) {
	debug := os.Getenv("MC_DEBUG") == "1"
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

	if debug {
		fmt.Fprintf(os.Stderr, "[GetMulti DEBUG] Requested %d keys, broadcasting to %d servers: %v\n", len(keys), len(serverList), serverList)
		fmt.Fprintf(os.Stderr, "[GetMulti DEBUG] Encoded keys (%d): %v\n", len(allEncodedKeys), allEncodedKeys)
		for n, k := range keys {
			if enc, exists := originalToEncoded[k]; exists {
				fmt.Fprintf(os.Stderr, "[GetMulti DEBUG] Key[%d]: orig=%s, encoded=%s, opaque=%d, validServers=%v\n",
					n, k, enc, keyNum[enc], func() []string {
						if vs, ok := keyToValidServers[enc]; ok {
							var addrs []string
							for addr := range vs {
								addrs = append(addrs, addr)
							}
							sort.Strings(addrs)
							return addrs
						}
						return nil
					}())
			}
		}
	}

	// Send all keys to each server (broadcast to handle cases where Set wrote to alternative servers)
	for _, addr := range serverList {
		ch := make(chan *Item)
		chs = append(chs, ch)
		go func(addr string, ch chan *Item, keyNumMap map[string]int, allKeys []string, keyToValid map[string]map[string]bool, origToEncoded map[string]string, allEncodedKeys []string) {
			defer close(ch)
			localOpt := opt
			localOpt.returnKey = true // Request key in response as fallback (critical for CI)

			if debug {
				fmt.Fprintf(os.Stderr, "[GetMulti DEBUG] Server %s: connecting, sending %d keys\n", addr, len(allEncodedKeys))
			}

			conn, err := c.pool.getConn(addr)
			if err != nil {
				if debug {
					fmt.Fprintf(os.Stderr, "[GetMulti DEBUG] Server %s: connection failed: %v\n", addr, err)
				}
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
			var (
				item          *Item
				responseCount int
				acceptedCount int
				rejectedCount int
			)
			for {
				if item, err = parseGetResponse(ctx, c, conn.buff); err == nil {
					responseCount++
					var originalKey string
					var encodedKey string
					restoreMethod := "none"

					// Try to restore key using opaque value first (preferred method)
					if item.opaque > 0 && item.opaque <= len(allKeys) {
						originalKey = allKeys[item.opaque-1]
						if encKey, exists := origToEncoded[originalKey]; exists {
							encodedKey = encKey
							restoreMethod = "opaque"
						}
					}

					// Fallback: use key from response if opaque failed or missing (critical for CI)
					if encodedKey == "" && item.Key != "" {
						responseKey := item.Key
						// Key in response is in encoded form (base64 if binary, original if not)
						// Find original key by matching encoded key
						for orig, enc := range origToEncoded {
							if enc == responseKey {
								originalKey = orig
								encodedKey = enc
								restoreMethod = "key_from_response"
								break
							}
						}
						// If not found, try direct match (for non-binary keys)
						if encodedKey == "" {
							if orig, exists := origToEncoded[responseKey]; exists {
								originalKey = responseKey
								encodedKey = orig
								restoreMethod = "key_direct_match"
							} else {
								// Try direct match for non-binary encoded keys
								originalKey = responseKey
								encodedKey = responseKey
								restoreMethod = "key_fallback"
							}
						}
					}

					if debug {
						fmt.Fprintf(os.Stderr, "[GetMulti DEBUG] Server %s: response[%d] opaque=%d, keyFromResponse=%q, restoreMethod=%s, originalKey=%q, encodedKey=%q\n",
							addr, responseCount, item.opaque, item.Key, restoreMethod, originalKey, encodedKey)
					}

					// Only accept key if we found it and it came from a valid server
					if encodedKey != "" {
						if validServers, exists := keyToValid[encodedKey]; exists && validServers[addr] {
							item.Key = originalKey
							acceptedCount++
							if debug {
								fmt.Fprintf(os.Stderr, "[GetMulti DEBUG] Server %s: ACCEPTED key %q (encoded: %q)\n", addr, originalKey, encodedKey)
							}
							ch <- item
						} else {
							rejectedCount++
							if debug {
								fmt.Fprintf(os.Stderr, "[GetMulti DEBUG] Server %s: REJECTED key %q (encoded: %q) - invalid server (valid: %v)\n",
									addr, originalKey, encodedKey, func() bool {
										if vs, ok := keyToValid[encodedKey]; ok {
											return vs[addr]
										}
										return false
									}())
							}
							// If key came from invalid server, ignore it (might be stale or wrong)
						}
					} else {
						rejectedCount++
						if debug {
							fmt.Fprintf(os.Stderr, "[GetMulti DEBUG] Server %s: REJECTED response[%d] - could not restore key (opaque=%d, keyFromResponse=%q)\n",
								addr, responseCount, item.opaque, item.Key)
						}
					}
					continue
				}
				// Check for end of multi-get (MN response)
				if errors.Is(err, errMnDone) {
					if debug {
						fmt.Fprintf(os.Stderr, "[GetMulti DEBUG] Server %s: finished (responses=%d, accepted=%d, rejected=%d)\n",
							addr, responseCount, acceptedCount, rejectedCount)
					}
					break
				}
				// ErrCacheMiss is expected for non-existent keys
				if errors.Is(err, ErrCacheMiss) {
					if debug {
						fmt.Fprintf(os.Stderr, "[GetMulti DEBUG] Server %s: cache miss (expected)\n", addr)
					}
					continue
				}
				// Any other error is unexpected, stop reading
				if debug {
					fmt.Fprintf(os.Stderr, "[GetMulti DEBUG] Server %s: ERROR - %v (responses=%d, accepted=%d, rejected=%d)\n",
						addr, err, responseCount, acceptedCount, rejectedCount)
				}
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

	if debug {
		fmt.Fprintf(os.Stderr, "[GetMulti DEBUG] FINAL RESULT: got %d items out of %d requested keys\n", len(items), len(keys))
		if len(items) < len(keys) {
			var missing []string
			for _, k := range keys {
				if _, found := items[k]; !found {
					missing = append(missing, k)
				}
			}
			fmt.Fprintf(os.Stderr, "[GetMulti DEBUG] MISSING KEYS (%d): %v\n", len(missing), missing)
			var found []string
			for k := range items {
				found = append(found, k)
			}
			sort.Strings(found)
			fmt.Fprintf(os.Stderr, "[GetMulti DEBUG] FOUND KEYS (%d): %v\n", len(found), found)
		}
	}

	return items, nil
}
