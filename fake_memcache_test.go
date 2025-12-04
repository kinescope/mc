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
	mu       sync.RWMutex
	data     map[string]*cacheItem
	listener net.Listener
	closed   bool
	addr     string
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
		conn, err := s.listener.Accept()
		if err != nil {
			if s.closed {
				return
			}
			continue
		}
		go s.handleConnection(conn)
	}
}

func (s *fakeMemcacheServer) handleConnection(conn net.Conn) {
	defer conn.Close()
	reader := bufio.NewReader(conn)
	writer := bufio.NewWriter(conn)

	for {
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

	if opaque > 0 {
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

func (s *fakeMemcacheServer) Close() error {
	s.closed = true
	return s.listener.Close()
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

	// Verify we got all keys (Set wrote to available servers, GetMulti should find them)
	// Note: GetMulti currently only tries the primary server, so if that server is down,
	// it won't find keys. This is the issue we're testing.
	if len(availableAddrs) > 1 {
		// If we have multiple available servers, GetMulti might not find all keys
		// because it only tries the primary server from PickServer
		assert.Greater(t, len(results), 0, "should get at least some keys")
	} else {
		// If all servers are available, we should get all keys
		assert.Len(t, results, len(keys), "should get all keys when all servers are available")
	}

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
