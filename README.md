# Memcache Client

[![CI](https://github.com/kinescope/mc/workflows/run-tests/badge.svg)]()
[![Go Report Card](https://goreportcard.com/badge/github.com/kinescope/mc)](https://goreportcard.com/report/github.com/kinescope/mc)
[![godoc](https://img.shields.io/badge/docs-GoDoc-green.svg)](https://godoc.org/github.com/kinescope/mc)


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
		KeyHashFunc:         mc.XXXKeyHashFunc,
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
	}, mc.WithExpiration(1_800, 60) /*mc.WithMinUses(10),*/, mc.WithNamespace("video_id:xxx"))
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
}
```