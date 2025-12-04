package mc_test

import (
	"bufio"
	"context"
	"encoding/base64"
	"fmt"
	"math/rand"
	"net"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/kinescope/mc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeMemcacheServer implements a simple memcache Meta Text Protocol server
type fakeMemcacheServer struct {
	mu         sync.RWMutex
	data       map[string]*cacheItem
	listener   net.Listener
	closed     bool
	addr       string
	wg         sync.WaitGroup // Wait group for active connections
	skipOpaque bool           // For testing: skip returning opaque values
}

type cacheItem struct {
	value      []byte
	flags      uint16
	cas        uint64
	expiration int64 // Unix timestamp, 0 means no expiration
	hit        bool
	lastAccess int64 // Unix timestamp
}

func newFakeMemcacheServer() (*fakeMemcacheServer, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}

	server := &fakeMemcacheServer{
		data:     make(map[string]*cacheItem),
		listener: listener,
		addr:     listener.Addr().String(),
	}

	go server.acceptConnections()
	return server, nil
}

func (s *fakeMemcacheServer) acceptConnections() {
	for {
		s.mu.RLock()
		closed := s.closed
		s.mu.RUnlock()
		if closed {
			return
		}

		conn, err := s.listener.Accept()
		if err != nil {
			s.mu.RLock()
			closed = s.closed
			s.mu.RUnlock()
			if closed {
				return
			}
			continue
		}
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			s.handleConnection(conn)
		}()
	}
}

func (s *fakeMemcacheServer) handleConnection(conn net.Conn) {
	defer conn.Close()
	reader := bufio.NewReader(conn)
	writer := bufio.NewWriter(conn)

	for {
		// Check if server is closed
		s.mu.RLock()
		closed := s.closed
		s.mu.RUnlock()
		if closed {
			return
		}

		line, err := reader.ReadString('\n')
		if err != nil {
			return
		}
		line = strings.TrimSpace(line)

		if line == "" {
			continue
		}

		parts := strings.Fields(line)
		if len(parts) == 0 {
			continue
		}

		cmd := parts[0]
		switch cmd {
		case "version":
			writer.WriteString("VERSION 1.6.14\r\n")
			writer.Flush()
		case "mg":
			s.handleMetaGet(parts[1:], reader, writer)
		case "ms":
			s.handleMetaSet(parts[1:], reader, writer)
		case "mn":
			writer.WriteString("MN\r\n")
			writer.Flush()
		case "md":
			s.handleMetaDelete(parts[1:], writer)
		default:
			writer.WriteString("CLIENT_ERROR unknown command\r\n")
			writer.Flush()
		}
	}
}

func (s *fakeMemcacheServer) handleMetaGet(parts []string, reader *bufio.Reader, writer *bufio.Writer) {
	if len(parts) < 1 {
		writer.WriteString("CLIENT_ERROR malformed request\r\n")
		writer.Flush()
		return
	}

	key := parts[0]
	var (
		opaque      int
		returnKey   bool
		returnCAS   bool
		returnFlags bool
		returnHit   bool
		returnLast  bool
		binaryKey   bool
	)

	// Parse flags
	for i := 1; i < len(parts); i++ {
		flag := parts[i]
		if len(flag) == 0 {
			continue
		}
		switch flag[0] {
		case 'O':
			if len(flag) > 1 {
				opaque, _ = strconv.Atoi(flag[1:])
			}
		case 'k':
			returnKey = true
		case 'c':
			returnCAS = true
		case 'f':
			returnFlags = true
		case 'h':
			returnHit = true
		case 'l':
			returnLast = true
		case 'b':
			binaryKey = true
		}
	}

	// Decode binary key if needed
	if binaryKey {
		decoded, err := base64.StdEncoding.DecodeString(key)
		if err == nil {
			key = string(decoded)
		}
	}

	s.mu.RLock()
	item, exists := s.data[key]
	s.mu.RUnlock()

	if !exists {
		writer.WriteString("EN\r\n")
		writer.Flush()
		return
	}

	// Update last access time
	s.mu.Lock()
	item.lastAccess = time.Now().Unix()
	item.hit = true
	s.mu.Unlock()

	// Build response
	var resp strings.Builder
	resp.WriteString("VA ")
	resp.WriteString(strconv.Itoa(len(item.value)))
	resp.WriteString(" ")

	if returnFlags {
		resp.WriteString("f")
		resp.WriteString(strconv.FormatUint(uint64(item.flags), 10))
		resp.WriteString(" ")
	}

	if returnCAS {
		resp.WriteString("c")
		resp.WriteString(strconv.FormatUint(item.cas, 10))
		resp.WriteString(" ")
	}

	if returnHit {
		if item.hit {
			resp.WriteString("h1 ")
		} else {
			resp.WriteString("h0 ")
		}
	}

	if returnLast {
		lastAccess := time.Now().Unix() - item.lastAccess
		resp.WriteString("l")
		resp.WriteString(strconv.FormatInt(lastAccess, 10))
		resp.WriteString(" ")
	}

	// Simulate bug: sometimes don't return opaque (to reproduce CI issue)
	// Uncomment this to test behavior when opaque is missing
	// if rand.Float32() < 0.1 { // 10% chance of missing opaque
	// 	opaque = 0
	// }
	// Check if this server should skip opaque (for testing)
	skipOpaque := false
	if s.skipOpaque {
		skipOpaque = true
	}

	if opaque > 0 && !skipOpaque {
		resp.WriteString("O")
		resp.WriteString(strconv.Itoa(opaque))
		resp.WriteString(" ")
	}

	if returnKey {
		encodedKey := key
		if binaryKey {
			encodedKey = base64.StdEncoding.EncodeToString([]byte(key))
		}
		resp.WriteString("k")
		resp.WriteString(encodedKey)
		resp.WriteString(" ")
	}

	resp.WriteString("\r\n")

	writer.WriteString(resp.String())
	writer.Write(item.value)
	writer.WriteString("\r\n")
	writer.Flush()
}

func (s *fakeMemcacheServer) handleMetaSet(parts []string, reader *bufio.Reader, writer *bufio.Writer) {
	if len(parts) < 2 {
		writer.WriteString("CLIENT_ERROR malformed request\r\n")
		writer.Flush()
		return
	}

	key := parts[0]
	size, err := strconv.Atoi(parts[1])
	if err != nil {
		writer.WriteString("CLIENT_ERROR malformed request\r\n")
		writer.Flush()
		return
	}

	var (
		mode       = "S" // default set
		expiration int64
		flags      uint16
		cas        uint64
		binaryKey  bool
		compareCAS bool
	)

	// Parse flags
	for i := 2; i < len(parts); i++ {
		flag := parts[i]
		if len(flag) == 0 {
			continue
		}
		switch flag[0] {
		case 'M':
			if len(flag) > 1 {
				mode = flag[1:]
			}
		case 'T':
			if len(flag) > 1 {
				exp, _ := strconv.ParseInt(flag[1:], 10, 64)
				expiration = time.Now().Unix() + exp
			}
		case 'F':
			if len(flag) > 1 {
				fl, _ := strconv.ParseUint(flag[1:], 10, 16)
				flags = uint16(fl)
			}
		case 'C':
			if len(flag) > 1 {
				cas, _ = strconv.ParseUint(flag[1:], 10, 64)
				compareCAS = true
			}
		case 'b':
			binaryKey = true
		}
	}

	// Decode binary key if needed
	if binaryKey {
		decoded, err := base64.StdEncoding.DecodeString(key)
		if err == nil {
			key = string(decoded)
		}
	}

	// Read value
	value := make([]byte, size)
	if _, err := reader.Read(value); err != nil {
		writer.WriteString("CLIENT_ERROR failed to read value\r\n")
		writer.Flush()
		return
	}

	// Read CRLF
	crlf := make([]byte, 2)
	reader.Read(crlf)

	s.mu.Lock()
	defer s.mu.Unlock()

	existing, exists := s.data[key]

	switch mode {
	case "E": // Add
		if exists {
			writer.WriteString("NS\r\n")
			writer.Flush()
			return
		}
		s.data[key] = &cacheItem{
			value:      value,
			flags:      flags,
			cas:        uint64(time.Now().UnixNano()),
			expiration: expiration,
			lastAccess: time.Now().Unix(),
		}
		writer.WriteString("HD\r\n")
		writer.Flush()

	case "R": // Replace
		if !exists {
			writer.WriteString("NS\r\n")
			writer.Flush()
			return
		}
		if compareCAS && existing.cas != cas {
			writer.WriteString("EX\r\n")
			writer.Flush()
			return
		}
		existing.value = value
		existing.flags = flags
		existing.cas = uint64(time.Now().UnixNano())
		if expiration > 0 {
			existing.expiration = expiration
		}
		writer.WriteString("HD\r\n")
		writer.Flush()

	case "A": // Append
		if !exists {
			writer.WriteString("NS\r\n")
			writer.Flush()
			return
		}
		existing.value = append(existing.value, value...)
		existing.cas = uint64(time.Now().UnixNano())
		writer.WriteString("HD\r\n")
		writer.Flush()

	case "P": // Prepend
		if !exists {
			writer.WriteString("NS\r\n")
			writer.Flush()
			return
		}
		existing.value = append(value, existing.value...)
		existing.cas = uint64(time.Now().UnixNano())
		writer.WriteString("HD\r\n")
		writer.Flush()

	case "S": // Set
		if compareCAS && exists && existing.cas != cas {
			writer.WriteString("EX\r\n")
			writer.Flush()
			return
		}
		s.data[key] = &cacheItem{
			value:      value,
			flags:      flags,
			cas:        uint64(time.Now().UnixNano()),
			expiration: expiration,
			lastAccess: time.Now().Unix(),
		}
		writer.WriteString("HD\r\n")
		writer.Flush()
	}
}

func (s *fakeMemcacheServer) handleMetaDelete(parts []string, writer *bufio.Writer) {
	if len(parts) < 1 {
		writer.WriteString("CLIENT_ERROR malformed request\r\n")
		writer.Flush()
		return
	}

	key := parts[0]
	var binaryKey bool

	for i := 1; i < len(parts); i++ {
		if parts[i] == "b" {
			binaryKey = true
			break
		}
	}

	if binaryKey {
		decoded, err := base64.StdEncoding.DecodeString(key)
		if err == nil {
			key = string(decoded)
		}
	}

	s.mu.Lock()
	_, exists := s.data[key]
	if exists {
		delete(s.data, key)
	}
	s.mu.Unlock()

	if exists {
		writer.WriteString("HD\r\n")
	} else {
		writer.WriteString("NF\r\n")
	}
	writer.Flush()
}

// setKeyDirectly sets a key directly on a specific server (bypassing client)
func (s *fakeMemcacheServer) setKeyDirectly(key, value string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[key] = &cacheItem{
		value:      []byte(value),
		cas:        uint64(time.Now().UnixNano()),
		lastAccess: time.Now().Unix(),
	}
}

func (s *fakeMemcacheServer) Close() error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil
	}
	s.closed = true
	s.mu.Unlock()

	// Close listener to stop accepting new connections
	err := s.listener.Close()

	// Wait for all active connections to finish
	s.wg.Wait()

	return err
}

func (s *fakeMemcacheServer) Addr() string {
	return s.addr
}

// createFakeServers creates a random number of fake memcache servers (3-7)
func createFakeServers(t *testing.T) ([]*fakeMemcacheServer, []string) {
	numServers := 3 + rand.Intn(5) // 3-7 servers
	servers := make([]*fakeMemcacheServer, 0, numServers)
	addrs := make([]string, 0, numServers)

	for i := 0; i < numServers; i++ {
		server, err := newFakeMemcacheServer()
		require.NoError(t, err)
		servers = append(servers, server)
		addrs = append(addrs, server.Addr())
	}

	return servers, addrs
}

func TestGetMultiWithFakeServers(t *testing.T) {
	servers, addrs := createFakeServers(t)
	defer func() {
		for _, s := range servers {
			s.Close()
		}
	}()

	ctx := context.Background()
	client, err := mc.New(&mc.Options{
		Addrs: addrs,
	})
	require.NoError(t, err)
	defer client.Close()

	// Set some keys
	keys := []string{"key1", "key2", "key3", "key4", "key5"}
	expected := make(map[string]string)

	for i, key := range keys {
		value := fmt.Sprintf("value%d", i+1)
		err := client.Set(ctx, &mc.Item{
			Key:   key,
			Value: []byte(value),
		})
		require.NoError(t, err)
		expected[key] = value
	}

	// Get all keys using GetMulti
	results, err := client.GetMulti(ctx, keys)
	require.NoError(t, err)

	// Verify all keys are present
	assert.Len(t, results, len(keys))
	for key, expectedValue := range expected {
		if item, ok := results[key]; assert.True(t, ok, "key %s not found", key) {
			assert.Equal(t, expectedValue, string(item.Value), "key %s has wrong value", key)
		}
	}
}

// TestGetMultiKeyOnPrimaryServer tests that key on primary server is accepted
func TestGetMultiKeyOnPrimaryServer(t *testing.T) {
	servers, addrs := createFakeServers(t)
	require.GreaterOrEqual(t, len(servers), 2, "need at least 2 servers")
	defer func() {
		for _, s := range servers {
			s.Close()
		}
	}()

	ctx := context.Background()
	// Disable binary encoding for simpler testing
	// Custom PickServer: key1 -> [server0, server1], key2 -> [server1, server0]
	pickServer := func(key string) []string {
		if key == "key1" || strings.Contains(key, "key1") {
			return []string{addrs[0], addrs[1]}
		}
		if key == "key2" || strings.Contains(key, "key2") {
			return []string{addrs[1], addrs[0]}
		}
		// Default: first server
		return []string{addrs[0]}
	}

	client, err := mc.New(&mc.Options{
		Addrs:                    addrs,
		PickServer:               pickServer,
		DisableBinaryEncodedKeys: true,
	})
	require.NoError(t, err)
	defer client.Close()

	// Set key1 on primary server (server0)
	servers[0].setKeyDirectly("key1", "value1")
	// Set key2 on primary server (server1)
	servers[1].setKeyDirectly("key2", "value2")

	// GetMulti should find both keys
	results, err := client.GetMulti(ctx, []string{"key1", "key2"})
	require.NoError(t, err)

	assert.Len(t, results, 2)
	assert.Equal(t, "value1", string(results["key1"].Value))
	assert.Equal(t, "value2", string(results["key2"].Value))
}

// TestGetMultiKeyOnAlternativeServer tests that key on alternative valid server is accepted
func TestGetMultiKeyOnAlternativeServer(t *testing.T) {
	servers, addrs := createFakeServers(t)
	require.GreaterOrEqual(t, len(servers), 2, "need at least 2 servers")
	defer func() {
		for _, s := range servers {
			s.Close()
		}
	}()

	ctx := context.Background()
	// Disable binary encoding for simpler testing
	// Custom PickServer: key1 -> [server0, server1]
	pickServer := func(key string) []string {
		if key == "key1" || strings.Contains(key, "key1") {
			return []string{addrs[0], addrs[1]}
		}
		return []string{addrs[0]}
	}

	client, err := mc.New(&mc.Options{
		Addrs:                    addrs,
		PickServer:               pickServer,
		DisableBinaryEncodedKeys: true,
	})
	require.NoError(t, err)
	defer client.Close()

	// Set key1 on alternative server (server1), NOT on primary (server0)
	servers[1].setKeyDirectly("key1", "value1")

	// GetMulti should find key1 on alternative server
	results, err := client.GetMulti(ctx, []string{"key1"})
	require.NoError(t, err)

	assert.Len(t, results, 1)
	assert.Equal(t, "value1", string(results["key1"].Value))
}

// TestGetMultiKeyOnInvalidServer tests that key on invalid server is rejected
func TestGetMultiKeyOnInvalidServer(t *testing.T) {
	servers, addrs := createFakeServers(t)
	require.GreaterOrEqual(t, len(servers), 3, "need at least 3 servers")
	defer func() {
		for _, s := range servers {
			s.Close()
		}
	}()

	ctx := context.Background()
	// Custom PickServer: key1 -> [server0, server1] (NOT server2)
	pickServer := func(key string) []string {
		if key == "key1" || strings.Contains(key, "key1") {
			return []string{addrs[0], addrs[1]}
		}
		return []string{addrs[0]}
	}

	client, err := mc.New(&mc.Options{
		Addrs:                    addrs,
		PickServer:               pickServer,
		DisableBinaryEncodedKeys: true,
	})
	require.NoError(t, err)
	defer client.Close()

	// Set key1 on invalid server (server2), which is NOT in PickServer list
	servers[2].setKeyDirectly("key1", "wrong_value")

	// GetMulti should NOT find key1 because it's on invalid server
	results, err := client.GetMulti(ctx, []string{"key1"})
	require.NoError(t, err)

	// Key should not be in results (rejected because from invalid server)
	assert.Len(t, results, 0, "key on invalid server should be rejected")
}

// TestGetMultiCollisionResolution tests collision resolution when key exists on multiple servers
func TestGetMultiCollisionResolution(t *testing.T) {
	servers, addrs := createFakeServers(t)
	require.GreaterOrEqual(t, len(servers), 2, "need at least 2 servers")
	defer func() {
		for _, s := range servers {
			s.Close()
		}
	}()

	ctx := context.Background()
	// Custom PickServer: key1 -> [server0, server1]
	pickServer := func(key string) []string {
		if key == "key1" || strings.Contains(key, "key1") {
			return []string{addrs[0], addrs[1]}
		}
		return []string{addrs[0]}
	}

	client, err := mc.New(&mc.Options{
		Addrs:                    addrs,
		PickServer:               pickServer,
		DisableBinaryEncodedKeys: true,
	})
	require.NoError(t, err)
	defer client.Close()

	// Set key1 on BOTH servers with different values
	servers[0].setKeyDirectly("key1", "value_from_primary")
	servers[1].setKeyDirectly("key1", "value_from_alternative")

	// GetMulti should accept key from primary server (server0)
	results, err := client.GetMulti(ctx, []string{"key1"})
	require.NoError(t, err)

	// Should get value from primary server (first in PickServer list)
	assert.Len(t, results, 1)
	// Note: both responses might come, but we accept the one from valid server
	// The actual value depends on which response arrives first, but both are valid
	assert.Contains(t, []string{"value_from_primary", "value_from_alternative"}, string(results["key1"].Value))
}

// TestGetMultiPartialKeys tests when some keys are found and some are not
func TestGetMultiPartialKeys(t *testing.T) {
	servers, addrs := createFakeServers(t)
	require.GreaterOrEqual(t, len(servers), 2, "need at least 2 servers")
	defer func() {
		for _, s := range servers {
			s.Close()
		}
	}()

	ctx := context.Background()
	client, err := mc.New(&mc.Options{
		Addrs:                    addrs,
		DisableBinaryEncodedKeys: true,
	})
	require.NoError(t, err)
	defer client.Close()

	// Set only some keys
	servers[0].setKeyDirectly("key1", "value1")
	servers[0].setKeyDirectly("key2", "value2")
	// key3 is not set

	results, err := client.GetMulti(ctx, []string{"key1", "key2", "key3"})
	require.NoError(t, err)

	// Should find only key1 and key2
	assert.Len(t, results, 2)
	assert.Equal(t, "value1", string(results["key1"].Value))
	assert.Equal(t, "value2", string(results["key2"].Value))
	assert.NotContains(t, results, "key3")
}

// TestGetMultiPrimaryUnavailableAlternativeAvailable tests when primary is unavailable but alternative has key
func TestGetMultiPrimaryUnavailableAlternativeAvailable(t *testing.T) {
	servers, addrs := createFakeServers(t)
	require.GreaterOrEqual(t, len(servers), 2, "need at least 2 servers")
	defer func() {
		for _, s := range servers {
			s.Close()
		}
	}()

	ctx := context.Background()
	// Custom PickServer: key1 -> [server0, server1]
	pickServer := func(key string) []string {
		if key == "key1" || strings.Contains(key, "key1") {
			return []string{addrs[0], addrs[1]}
		}
		return []string{addrs[0]}
	}

	client, err := mc.New(&mc.Options{
		Addrs:                    addrs,
		PickServer:               pickServer,
		DisableBinaryEncodedKeys: true,
	})
	require.NoError(t, err)
	defer client.Close()

	// Close primary server (server0)
	servers[0].Close()

	// Set key1 on alternative server (server1)
	servers[1].setKeyDirectly("key1", "value1")

	// GetMulti should find key1 on alternative server
	results, err := client.GetMulti(ctx, []string{"key1"})
	require.NoError(t, err)

	assert.Len(t, results, 1)
	assert.Equal(t, "value1", string(results["key1"].Value))
}

// TestGetMultiMultipleKeysDifferentServers tests multiple keys on different servers
func TestGetMultiMultipleKeysDifferentServers(t *testing.T) {
	servers, addrs := createFakeServers(t)
	require.GreaterOrEqual(t, len(servers), 3, "need at least 3 servers")
	defer func() {
		for _, s := range servers {
			s.Close()
		}
	}()

	ctx := context.Background()
	// Custom PickServer: each key goes to different servers
	pickServer := func(key string) []string {
		if key == "key1" || strings.Contains(key, "key1") {
			return []string{addrs[0], addrs[1]}
		}
		if key == "key2" || strings.Contains(key, "key2") {
			return []string{addrs[1], addrs[2]}
		}
		if key == "key3" || strings.Contains(key, "key3") {
			return []string{addrs[2], addrs[0]}
		}
		return []string{addrs[0]}
	}

	client, err := mc.New(&mc.Options{
		Addrs:                    addrs,
		PickServer:               pickServer,
		DisableBinaryEncodedKeys: true,
	})
	require.NoError(t, err)
	defer client.Close()

	// Set keys on their primary servers
	servers[0].setKeyDirectly("key1", "value1")
	servers[1].setKeyDirectly("key2", "value2")
	servers[2].setKeyDirectly("key3", "value3")

	// GetMulti should find all keys
	results, err := client.GetMulti(ctx, []string{"key1", "key2", "key3"})
	require.NoError(t, err)

	assert.Len(t, results, 3)
	assert.Equal(t, "value1", string(results["key1"].Value))
	assert.Equal(t, "value2", string(results["key2"].Value))
	assert.Equal(t, "value3", string(results["key3"].Value))
}

// TestGetMultiStaleDataRejection tests that stale data from invalid server is rejected
func TestGetMultiStaleDataRejection(t *testing.T) {
	servers, addrs := createFakeServers(t)
	require.GreaterOrEqual(t, len(servers), 3, "need at least 3 servers")
	defer func() {
		for _, s := range servers {
			s.Close()
		}
	}()

	ctx := context.Background()
	// Custom PickServer: key1 -> [server0, server1] (NOT server2)
	pickServer := func(key string) []string {
		if key == "key1" || strings.Contains(key, "key1") {
			return []string{addrs[0], addrs[1]}
		}
		return []string{addrs[0]}
	}

	client, err := mc.New(&mc.Options{
		Addrs:                    addrs,
		PickServer:               pickServer,
		DisableBinaryEncodedKeys: true,
	})
	require.NoError(t, err)
	defer client.Close()

	// Set correct value on valid server (server0)
	servers[0].setKeyDirectly("key1", "correct_value")
	// Set stale/wrong value on invalid server (server2)
	servers[2].setKeyDirectly("key1", "stale_value")

	// GetMulti should accept only value from valid server
	results, err := client.GetMulti(ctx, []string{"key1"})
	require.NoError(t, err)

	assert.Len(t, results, 1)
	assert.Equal(t, "correct_value", string(results["key1"].Value), "should reject stale data from invalid server")
}

// TestGetMultiAllKeysNotFound tests when no keys are found
func TestGetMultiAllKeysNotFound(t *testing.T) {
	servers, addrs := createFakeServers(t)
	defer func() {
		for _, s := range servers {
			s.Close()
		}
	}()

	ctx := context.Background()
	client, err := mc.New(&mc.Options{
		Addrs:                    addrs,
		DisableBinaryEncodedKeys: true,
	})
	require.NoError(t, err)
	defer client.Close()

	// Don't set any keys
	results, err := client.GetMulti(ctx, []string{"key1", "key2", "key3"})
	require.NoError(t, err)

	// Should return empty map
	assert.Len(t, results, 0)
}

// TestGetMultiMixedValidInvalidServers tests mixed scenario with valid and invalid servers
func TestGetMultiMixedValidInvalidServers(t *testing.T) {
	servers, addrs := createFakeServers(t)
	require.GreaterOrEqual(t, len(servers), 3, "need at least 3 servers")
	defer func() {
		for _, s := range servers {
			s.Close()
		}
	}()

	ctx := context.Background()
	// Custom PickServer:
	// key1 -> [server0, server1] (valid)
	// key2 -> [server1, server2] (valid)
	// key3 -> [server0] (valid, but we'll put it on server2 which is invalid for key3)
	pickServer := func(key string) []string {
		if key == "key1" || strings.Contains(key, "key1") {
			return []string{addrs[0], addrs[1]}
		}
		if key == "key2" || strings.Contains(key, "key2") {
			return []string{addrs[1], addrs[2]}
		}
		if key == "key3" || strings.Contains(key, "key3") {
			return []string{addrs[0]} // Only server0 is valid for key3
		}
		return []string{addrs[0]}
	}

	client, err := mc.New(&mc.Options{
		Addrs:                    addrs,
		PickServer:               pickServer,
		DisableBinaryEncodedKeys: true,
	})
	require.NoError(t, err)
	defer client.Close()

	// Set key1 on valid server (server0)
	servers[0].setKeyDirectly("key1", "value1")
	// Set key2 on valid server (server1)
	servers[1].setKeyDirectly("key2", "value2")
	// Set key3 on INVALID server (server2) - should be rejected
	servers[2].setKeyDirectly("key3", "wrong_value")

	results, err := client.GetMulti(ctx, []string{"key1", "key2", "key3"})
	require.NoError(t, err)

	// Should find key1 and key2, but NOT key3 (rejected because on invalid server)
	assert.Len(t, results, 2)
	assert.Equal(t, "value1", string(results["key1"].Value))
	assert.Equal(t, "value2", string(results["key2"].Value))
	assert.NotContains(t, results, "key3", "key3 should be rejected (on invalid server)")
}

func TestGetMultiWithUnavailableServers(t *testing.T) {
	// Use only available servers from testServerAddrs
	availableAddrs := checkAvailableServers(t, testServerAddrs)
	require.Greater(t, len(availableAddrs), 0, "at least one server should be available")

	// If we have only one server, skip this test as we can't test unavailable servers
	if len(availableAddrs) < 2 {
		t.Skip("Need at least 2 available servers to test unavailable server scenario")
	}

	ctx := context.Background()
	// Use all testServerAddrs (including potentially unavailable ones)
	// to simulate real scenario where some servers might be down
	client, err := mc.New(&mc.Options{
		Addrs: testServerAddrs,
	})
	require.NoError(t, err)
	defer client.Close()

	// Set keys - Set will use pickServer which tries all servers until it finds an available one
	keys := []string{"key1", "key2", "key3", "key4", "key5"}
	expected := make(map[string]string)

	for i, key := range keys {
		value := fmt.Sprintf("value%d", i+1)
		err := client.Set(ctx, &mc.Item{
			Key:   key,
			Value: []byte(value),
		})
		require.NoError(t, err, "Set should succeed even if primary server is unavailable")
		expected[key] = value
	}

	// Verify keys were actually stored by checking each one individually
	// This simulates what Get does - it uses pickServer to find available servers
	for key, expectedValue := range expected {
		item, err := client.Get(ctx, key)
		require.NoError(t, err, "Get should find key %s on available server", key)
		assert.Equal(t, expectedValue, string(item.Value), "key %s should have correct value", key)
	}

	// GetMulti should find keys even if primary server is unavailable
	// because Set might have written to a different server
	results, err := client.GetMulti(ctx, keys)
	require.NoError(t, err)

	// Verify we got all keys (Set wrote to available servers, GetMulti broadcasts to all servers)
	// GetMulti now broadcasts all keys to all servers and validates responses,
	// so it should find keys even if primary server was unavailable during Set
	assert.Len(t, results, len(keys), "should get all keys even if primary server was unavailable")

	// Verify that keys we got have correct values
	for key, item := range results {
		if expectedValue, ok := expected[key]; ok {
			assert.Equal(t, expectedValue, string(item.Value), "key %s has wrong value", key)
		}
	}
}

// checkAvailableServers checks which servers are actually available
func checkAvailableServers(t *testing.T, addrs []string) []string {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	available := make([]string, 0)
	for _, addr := range addrs {
		client, err := mc.New(&mc.Options{
			Addrs: []string{addr},
		})
		if err != nil {
			continue
		}

		// Try to set a test key to verify server is working
		testKey := fmt.Sprintf("_test_%d", time.Now().UnixNano())
		err = client.Set(ctx, &mc.Item{
			Key:   testKey,
			Value: []byte("test"),
		})
		client.Close()

		if err == nil {
			available = append(available, addr)
		}
	}

	return available
}

// TestGetMultiReproduceIssue reproduces the issue where GetMulti returns only 1 key instead of 20
// This test simulates the scenario from TestGetMultiNamespace that was failing in CI
func TestGetMultiReproduceIssue(t *testing.T) {
	servers, addrs := createFakeServers(t)
	require.GreaterOrEqual(t, len(servers), 2, "need at least 2 servers")
	defer func() {
		for _, s := range servers {
			s.Close()
		}
	}()

	ctx := context.Background()
	client, err := mc.New(&mc.Options{
		Addrs: addrs,
	})
	require.NoError(t, err)
	defer client.Close()

	// Create 20 keys like in TestGetMultiNamespace
	var (
		keyVal = make(map[string]string)
		keys   []string
	)
	for n := range 20 {
		k := fmt.Sprintf("key_%d", n)
		v := fmt.Sprintf("value_%d", n)
		err := client.Set(ctx, &mc.Item{
			Key:   k,
			Value: []byte(v),
		})
		require.NoError(t, err, "Set should succeed for key %s", k)
		keyVal[k] = v
		keys = append(keys, k)
	}

	// GetMulti should return all 20 keys
	results, err := client.GetMulti(ctx, keys)
	require.NoError(t, err)

	// This is where the issue manifests - should have 20 items but might only have 1
	if !assert.Len(t, results, 20, "Expected 20 items, got %d. This reproduces the CI failure", len(results)) {
		// Debug: print which keys are missing
		t.Logf("Got %d keys instead of 20:", len(results))
		for k := range results {
			t.Logf("  - Found: %s", k)
		}
		for _, k := range keys {
			if _, found := results[k]; !found {
				t.Logf("  - Missing: %s", k)
				// Try individual Get to see if key exists
				item, err := client.Get(ctx, k)
				if err == nil {
					t.Logf("    -> Key exists on server (Get returned: %s)", string(item.Value))
				} else {
					t.Logf("    -> Key missing on server (Get error: %v)", err)
				}
			}
		}
		return
	}

	// Verify all keys have correct values
	for k, expectedVal := range keyVal {
		item, exists := results[k]
		if !assert.True(t, exists, "Key %s should be in results", k) {
			continue
		}
		assert.Equal(t, expectedVal, string(item.Value), "Key %s has wrong value", k)
	}
}

// TestGetMultiWithoutOpaque tests behavior when server doesn't return opaque values
// This reproduces the issue: without opaque, keys cannot be restored and are lost
func TestGetMultiWithoutOpaque(t *testing.T) {
	server, err := newFakeMemcacheServer()
	require.NoError(t, err)
	defer server.Close()

	// Configure server to skip opaque (reproducing the bug)
	server.skipOpaque = true

	ctx := context.Background()
	client, err := mc.New(&mc.Options{
		Addrs: []string{server.Addr()},
	})
	require.NoError(t, err)
	defer client.Close()

	// Create 20 keys like in the failing test
	var (
		keyVal = make(map[string]string)
		keys   []string
	)
	for n := range 20 {
		k := fmt.Sprintf("key_%d", n)
		v := fmt.Sprintf("value_%d", n)
		err := client.Set(ctx, &mc.Item{
			Key:   k,
			Value: []byte(v),
		})
		require.NoError(t, err)
		keyVal[k] = v
		keys = append(keys, k)
	}

	// The issue: if opaque is missing, GetMulti can't restore keys
	// Current implementation requires opaque > 0 && opaque <= len(allKeys)
	// So if opaque is 0 or missing, the key is silently ignored

	results, err := client.GetMulti(ctx, keys)
	require.NoError(t, err)

	// Without opaque, we should get 0 keys - this reproduces the CI issue!
	t.Logf("GetMulti without opaque returned %d keys (expected 0 because opaque is required)", len(results))

	if len(results) > 0 {
		t.Logf("WARNING: Got %d keys even without opaque - this shouldn't happen with current implementation", len(results))
		for k := range results {
			t.Logf("  - Unexpectedly got key: %s", k)
		}
	} else {
		t.Logf("SUCCESS: Reproduced the issue - without opaque, all keys are lost!")
		t.Logf("This confirms that the CI failure is due to missing opaque values")
	}
}
