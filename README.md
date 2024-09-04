# Memcache Client

[![CI](https://github.com/kinescope/mc/workflows/run-tests/badge.svg)]()
[![Go Report Card](https://goreportcard.com/badge/github.com/kinescope/mc)](https://goreportcard.com/report/github.com/kinescope/mc)
[![godoc](https://img.shields.io/badge/docs-GoDoc-green.svg)](https://godoc.org/github.com/kinescope/mc)

This is a memcache client library for the Go programming language, which uses memcache's binary protocol and supports namespacing out of the box.

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

func main() {
	cache, err := mc.New(&mc.Options{
		Addrs:               []string{"127.0.0.1:11211"},
		DialTimeout:         500 * time.Millisecond,
		KeyHashFunc:         mc.XXKeyHashFunc, // or mc.DefaultKeyHashFunc. Hash function to use for namespaces
		ConnMaxLifetime:     15 * time.Minute,
		MaxIdleConnsPerAddr: 20,
	})
	if err != nil {
		log.Fatal(err)
	}

	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(time.Second))
	defer cancel()

	type example struct {
		A string
		B string
	}

	var value mc.Value
	err = value.Marshal(example{
		A: "A",
		B: "B",
	})
	if err != nil {
		log.Fatal(err)
	}

	err = cache.Set(ctx, &mc.Item{
		Key:   "cache_key",
		Value: value,
	}, mc.WithExpiration(1_800, 60) /*mc.WithMinUses(10),*/, mc.WithNamespace("my_namespace"))
	if err != nil {
		log.Fatal(err)
	}

	i, err := cache.Get(ctx, "cache_key")
	if err != nil {
		log.Fatal(err)
	}
	var v example
	if err := i.Value.Unmarshal(&v); err != nil {
		log.Fatal(err)
	}
	log.Println(v)
```
#### Namespacing
Let's say you have a user with some user_id like `123`. Given a user and all his related keys, you want a one-stop switch to invalidate all of their cache entries at the same time.
With namespacing you need to add a `mc.WithNamespace` option when setting any user related key.
```go
// set user name for id
var userName mc.Value
err = userName.Marshal("John")
err = cache.Set(ctx, &mc.Item{
	Key:   "123",
	Value: userName,
}, mc.WithNamespace("user_123"))

// set user email for name
var userEmail mc.Value
err = userEmail.Marshal("example@gmail.com")
err = cache.Set(ctx, &mc.Item{
	Key:   "John",
	Value: userEmail,
}, mc.WithNamespace("user_123"))
```
Then invalidating all of the keys for a user with id `123` would be as easy as:
```go
cache.PurgeNamespace(ctx, "user_123") // both 123:John and John:example@gmail.com entries will be deleted
```
Namespaces are hashed by default to prevent collisions. There are two available hash functions - `mc.XXKeyHashFunc`, witch uses xx-hash, and `mc.DefaultKeyHashFunc` - witch doesn't hash keys at all.

For more info on namespaces see [memcache wiki](https://github.com/memcached/memcached/wiki/ProgrammingTricks#namespacing).

#### Other options
- `mc.WithExpiration(exp, scale uint32)` -  after expiration time passes, first request for an item will get a cache miss, any other request will get a hit in a time window of `scale` seconds. See [memcache wiki](https://github.com/memcached/memcached/wiki/ProgrammingTricks#scaling-expiration).
- `mc.WithMinUses(number uint32)` - if an item under the key has been set less than `number` of times, requesting an item will result in a cache miss. See [tests](https://github.com/kinescope/mc/blob/main/client_extend_test.go) for clarity.

## Contributing

For contributing see [`CONTRIBUTING.md`](https://github.com/kinescope/mc/CONTRIBUTING.md)

## Licensing
The code in this project is licensed under MIT license.