# Memcache Client for Go

[![CI](https://github.com/kinescope/mc/workflows/run-tests/badge.svg)](https://github.com/kinescope/mc/actions)
[![Go Report Card](https://goreportcard.com/badge/github.com/kinescope/mc)](https://goreportcard.com/report/github.com/kinescope/mc)
[![GoDoc](https://img.shields.io/badge/docs-GoDoc-blue.svg)](https://godoc.org/github.com/kinescope/mc)

A high-performance memcache client library for Go that uses memcache's [Meta Text Protocol](https://docs.memcached.org/protocols/meta) and supports namespacing, compression, and context cancellation out of the box.

## Features

- ✅ **Meta Text Protocol** - Full support for memcache's modern protocol
- ✅ **Context Support** - All operations support `context.Context` for cancellation and timeouts
- ✅ **Namespacing** - Built-in namespace support for cache invalidation
- ✅ **Compression** - Optional compression with configurable thresholds
- ✅ **Connection Pooling** - Efficient connection pooling with automatic management
- ✅ **Binary Key Encoding** - Optional binary key encoding for better performance
- ✅ **Type Safety** - Strong typing with comprehensive error handling

## Installation

```bash
go get github.com/kinescope/mc
```

## Quick Start

```go
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/kinescope/mc"
)

func main() {
	// Create a new client
	client, err := mc.New(&mc.Options{
		Addrs: []string{"127.0.0.1:11211"},
	})
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()

	ctx := context.Background()

	// Set a value
	err = client.Set(ctx, &mc.Item{
		Key:   "user:123",
		Value: []byte("John Doe"),
		Flags: 0,
	}, mc.WithExpiration(3600))
	if err != nil {
		log.Fatal(err)
	}

	// Get a value
	item, err := client.Get(ctx, "user:123")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Value: %s\n", item.Value)
}
```

## Basic Operations

### Set, Get, Delete

```go
ctx := context.Background()

// Set
err := client.Set(ctx, &mc.Item{
	Key:   "key",
	Value: []byte("value"),
}, mc.WithExpiration(60))

// Get
item, err := client.Get(ctx, "key")
if err != nil {
	if err == mc.ErrCacheMiss {
		// Key not found
	}
}

// Delete
err = client.Del(ctx, "key")
```

### Add and Compare-and-Swap

```go
// Add (only if key doesn't exist)
err := client.Add(ctx, &mc.Item{
	Key:   "key",
	Value: []byte("value"),
})

// Compare-and-Swap (optimistic locking)
item, err := client.Get(ctx, "key", mc.WithCAS())
if err == nil {
	item.Value = []byte("new value")
	err = client.CompareAndSwap(ctx, item)
}
```

### Increment and Decrement

```go
// Increment
newValue, err := client.Inc(ctx, "counter", 1, 3600, mc.WithInitialValue(0))

// Decrement
newValue, err := client.Dec(ctx, "counter", 1, 3600)
```

### Get Multiple Keys

```go
keys := []string{"key1", "key2", "key3"}
items, err := client.GetMulti(ctx, keys)
for key, item := range items {
	fmt.Printf("%s: %s\n", key, item.Value)
}
```

## Advanced Features

### Namespacing

Namespacing allows you to invalidate all cache entries for a namespace with a single operation. This is useful for cache invalidation by user, tenant, or any logical grouping.

```go
ctx := context.Background()
userID := "123"

// Store items with namespace
err := client.Set(ctx, &mc.Item{
	Key:   "name",
	Value: []byte("John"),
}, mc.WithNamespace("user:"+userID))

err = client.Set(ctx, &mc.Item{
	Key:   "email",
	Value: []byte("john@example.com"),
}, mc.WithNamespace("user:"+userID))

// Invalidate all items for this user
err = client.PurgeNamespace(ctx, "user:"+userID)
// Both "name" and "email" keys are now invalidated
```

For more information, see the [memcache namespacing documentation](https://github.com/memcached/memcached/wiki/ProgrammingTricks#namespacing).

### Compression

Enable compression for values larger than a specified threshold:

```go
err := client.Set(ctx, &mc.Item{
	Key:   "large_data",
	Value: largeData,
}, mc.WithCompression(1024)) // Compress if value > 1KB
```

You can also provide custom compression functions:

```go
client, err := mc.New(&mc.Options{
	Addrs: []string{"127.0.0.1:11211"},
	Compression: struct {
		Compress   func([]byte) ([]byte, error)
		Decompress func([]byte) ([]byte, error)
	}{
		Compress:   myCompress,
		Decompress: myDecompress,
	},
})
```

### Context and Timeouts

All operations support `context.Context` for cancellation and timeouts:

```go
// With timeout
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()

item, err := client.Get(ctx, "key")
if err != nil {
	if err == context.DeadlineExceeded {
		// Operation timed out
	}
}

// With cancellation
ctx, cancel := context.WithCancel(context.Background())
go func() {
	time.Sleep(1 * time.Second)
	cancel() // Cancel the operation
}()
item, err := client.Get(ctx, "key")
```

### Min Uses

Require an item to be set a minimum number of times before it can be retrieved:

```go
// Set with min uses
err := client.Set(ctx, &mc.Item{
	Key:   "key",
	Value: []byte("value"),
}, mc.WithMinUses(3))

// First 2 Get calls will return ErrCacheMiss
// After 3rd Set, Get will succeed
```

### Early Recache

Enable early recaching to refresh items before they expire:

```go
item, err := client.Get(ctx, "key", mc.WithEarlyRecache(60))
if item.Won() {
	// This client "won" the recache, should refresh the value
}
```

### Last Access Time

Get the time since last access:

```go
item, err := client.Get(ctx, "key", mc.WithLastAccess())
fmt.Printf("Last accessed %d seconds ago\n", item.LastAccess())
```

## Configuration

### Client Options

```go
client, err := mc.New(&mc.Options{
	// Server addresses
	Addrs: []string{"127.0.0.1:11211", "127.0.0.1:11212"},

	// Connection settings
	DialTimeout:         200 * time.Millisecond,
	ConnMaxLifetime:     time.Minute,
	MaxIdleConnsPerAddr: 10,

	// Disable binary key encoding (use plain text keys)
	DisableBinaryEncodedKeys: false,

	// Custom server selection function
	PickServer: func(key string) []string {
		// Custom logic to select servers
		return []string{"127.0.0.1:11211"}
	},

	// Custom compression
	Compression: struct {
		Compress   func([]byte) ([]byte, error)
		Decompress func([]byte) ([]byte, error)
	}{
		Compress:   gzip.Compress,
		Decompress: gzip.Decompress,
	},
})
```

## Error Handling

The library provides typed errors for common scenarios:

```go
item, err := client.Get(ctx, "key")
switch err {
case mc.ErrCacheMiss:
	// Key not found
case mc.ErrNotStored:
	// Item not stored (e.g., Add failed because key exists)
case mc.ErrCASConflict:
	// Compare-and-swap conflict
case mc.ErrMalformedKey:
	// Invalid key format
case mc.ErrNoServers:
	// No servers available
case nil:
	// Success
default:
	// Other error
}
```

## Graceful Shutdown

Always close the client when done to release resources:

```go
client, err := mc.New(&mc.Options{
	Addrs: []string{"127.0.0.1:11211"},
})
defer client.Close() // Closes all connections in the pool
```

## Development

### Running Tests

```bash
# Run all tests
make test

# Run tests with race detector
make test-race

# Run tests with coverage
make test-coverage

# Run benchmarks
make test-bench
```

### Code Quality

```bash
# Format code
make fmt

# Run linter
make lint

# Run all checks
make check
```

### Generating Protobuf Code

```bash
make proto
```

## Examples

See the [`example_test.go`](example_test.go) file for more examples.

## Contributing

Contributions are welcome! Please see [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## References

- [Memcache Meta Text Protocol](https://docs.memcached.org/protocols/meta)
- [Memcache Namespacing](https://github.com/memcached/memcached/wiki/ProgrammingTricks#namespacing)
- [Go Documentation](https://godoc.org/github.com/kinescope/mc)
