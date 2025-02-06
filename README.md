# Memcache Client

[![CI](https://github.com/kinescope/mc/workflows/run-tests/badge.svg)]()
[![Go Report Card](https://goreportcard.com/badge/github.com/kinescope/mc)](https://goreportcard.com/report/github.com/kinescope/mc)
[![godoc](https://img.shields.io/badge/docs-GoDoc-green.svg)](https://godoc.org/github.com/kinescope/mc)

This is a memcache client library for the Go programming language, which uses memcache's [Meta Text Protocol](https://docs.memcached.org/protocols/meta) and supports namespacing out of the box.

## Installing
To add this libraty to your project, just run:
```bash
go get github.com/kinescope/mc
```

## Example
```go
package main

import (
	"context"
	"log"
	"time"

	"github.com/kinescope/mc"
)

func ExampleBase() {
	memcache, err := mc.New(&mc.Options{
		Addrs: []string{"127.0.0.1:11211"},
	})
	if err != nil {
		log.Fatal(err)
	}
	err = memcache.Set(&mc.Item{
		Key:   "key",
		Value: []byte("value"),
		Flags: 42,
	}, mc.WithExpiration(5))
	if err != nil {
		log.Fatal(err)
	}

	i, err := memcache.Get("key")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("key=%q, value=%q\n", i.Key, i.Value)
	// Output: key="key", value="value"
}

```
#### Namespacing
Let's say you have a user with some user_id like `123`. Given a user and all his related keys, you want a one-stop switch to invalidate all of their cache entries at the same time.
With namespacing you need to add a `mc.WithNamespace` option when setting any user related key.
```go

func ExampleNamespace() {
	memcache, err := mc.New(&mc.Options{
		Addrs: []string{"127.0.0.1:11211"},
	})
	if err != nil {
		log.Fatal(err)
	}
	err = memcache.Set(&mc.Item{
		Key:   "key",
		Value: []byte("value"),
		Flags: 42,
	}, mc.WithExpiration(5), mc.WithNamespace("namespace"))
	if err != nil {
		log.Fatal(err)
	}

	i, err := memcache.Get("key")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("key=%q, value=%q\n", i.Key, i.Value)

	memcache.PurgeNamespace("namespace")

	if _, err = memcache.Get("key"); err == nil {
		log.Fatal("ns bug")
	}
	fmt.Println(err)
	// Output:
	// key="key", value="value"
	// memcache: cache miss
}
```
Then invalidating all of the keys for a user with id `123` would be as easy as:
```go
cache.PurgeNamespace(ctx, "user_123") // both 123:John and John:example@gmail.com entries will be deleted
```

For more info on namespaces see [memcache wiki](https://github.com/memcached/memcached/wiki/ProgrammingTricks#namespacing).

#### Other options
- `mc.WithMinUses(number uint32)` - if an item under the key has been set less than `number` of times, requesting an item will result in a cache miss. See [tests](https://github.com/kinescope/mc/blob/main/client_extend_test.go) for clarity.
- `mc.WithEarlyRecache(seconds int)` - https://docs.memcached.org/protocols/meta/#early-recache
- `mc.WithLastAccess` - returns the time in seconds since last access.
- ... - see `mc.With*`

## Contributing

For contributing see [`CONTRIBUTING.md`](https://github.com/kinescope/mc/blob/main/CONTRIBUTING.md)

## Licensing
The code in this project is licensed under MIT license.
