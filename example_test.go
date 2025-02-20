package mc_test

import (
	"fmt"
	"log"

	"github.com/kinescope/mc"
)

func ExampleNew() {
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
