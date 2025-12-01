package mc_test

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/kinescope/mc"
)

func ExampleNew() {
	memcache, err := mc.New(&mc.Options{
		Addrs: []string{"127.0.0.1:11211"},
	})
	if err != nil {
		log.Fatal(err)
	}
	ctx := context.Background()
	err = memcache.Set(ctx, &mc.Item{
		Key:   "key",
		Value: []byte("value"),
		Flags: 42,
	}, mc.WithExpiration(5))
	if err != nil {
		log.Fatal(err)
	}

	i, err := memcache.Get(ctx, "key")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("key=%q, value=%q\n", i.Key, i.Value)
	// Output: key="key", value="value"
}

func ExampleClient_PurgeNamespace() {
	memcache, err := mc.New(&mc.Options{
		Addrs: []string{"127.0.0.1:11211"},
	})
	if err != nil {
		log.Fatal(err)
	}
	ctx := context.Background()
	err = memcache.Set(ctx, &mc.Item{
		Key:   "key",
		Value: []byte("value"),
		Flags: 42,
	}, mc.WithExpiration(5), mc.WithNamespace("namespace"))
	if err != nil {
		log.Fatal(err)
	}

	i, err := memcache.Get(ctx, "key")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("key=%q, value=%q\n", i.Key, i.Value)

	memcache.PurgeNamespace(ctx, "namespace")

	if _, err = memcache.Get(ctx, "key"); err == nil {
		log.Fatal("ns bug")
	}
	fmt.Println(err)
	// Output:
	// key="key", value="value"
	// memcache: cache miss
}

func ExampleClient_GetMulti() {
	memcache, err := mc.New(&mc.Options{
		Addrs: []string{"127.0.0.1:11211"},
	})
	if err != nil {
		log.Fatal(err)
	}
	defer memcache.Close()

	ctx := context.Background()

	// Set multiple keys
	keys := []string{"key1", "key2", "key3"}
	for i, key := range keys {
		err = memcache.Set(ctx, &mc.Item{
			Key:   key,
			Value: []byte(fmt.Sprintf("value%d", i+1)),
		}, mc.WithExpiration(60))
		if err != nil {
			log.Fatal(err)
		}
	}

	// Get multiple keys at once
	items, err := memcache.GetMulti(ctx, keys)
	if err != nil {
		log.Fatal(err)
	}

	for _, key := range keys {
		if item, ok := items[key]; ok {
			fmt.Printf("%s: %s\n", key, item.Value)
		}
	}
	// Output:
	// key1: value1
	// key2: value2
	// key3: value3
}

func ExampleClient_Add() {
	memcache, err := mc.New(&mc.Options{
		Addrs: []string{"127.0.0.1:11211"},
	})
	if err != nil {
		log.Fatal(err)
	}
	defer memcache.Close()

	ctx := context.Background()

	// Add only works if key doesn't exist
	err = memcache.Add(ctx, &mc.Item{
		Key:   "newkey",
		Value: []byte("value"),
	}, mc.WithExpiration(60))
	if err != nil {
		log.Fatal(err)
	}

	// Try to add again - will fail because key exists
	err = memcache.Add(ctx, &mc.Item{
		Key:   "newkey",
		Value: []byte("another value"),
	})
	if err == mc.ErrNotStored {
		fmt.Println("Key already exists, Add failed")
	}
	// Output: Key already exists, Add failed
}

func ExampleClient_CompareAndSwap() {
	memcache, err := mc.New(&mc.Options{
		Addrs: []string{"127.0.0.1:11211"},
	})
	if err != nil {
		log.Fatal(err)
	}
	defer memcache.Close()

	ctx := context.Background()

	// Set initial value
	err = memcache.Set(ctx, &mc.Item{
		Key:   "counter",
		Value: []byte("100"),
	}, mc.WithExpiration(60))
	if err != nil {
		log.Fatal(err)
	}

	// Get with CAS value
	item, err := memcache.Get(ctx, "counter", mc.WithCAS())
	if err != nil {
		log.Fatal(err)
	}

	// Modify and update using CAS
	item.Value = []byte("101")
	err = memcache.CompareAndSwap(ctx, item)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Updated successfully using CAS")
	// Output: Updated successfully using CAS
}

func ExampleClient_Inc() {
	memcache, err := mc.New(&mc.Options{
		Addrs: []string{"127.0.0.1:11211"},
	})
	if err != nil {
		log.Fatal(err)
	}
	defer memcache.Close()

	ctx := context.Background()

	// Use a unique key to avoid conflicts with other tests
	key := "example_counter_inc"

	// Delete key first to ensure clean state
	memcache.Del(ctx, key)

	// Increment counter, create with initial value 1 if doesn't exist
	// WithInitialValue(1) creates the key with value 1 if it doesn't exist
	newValue, err := memcache.Inc(ctx, key, 1, 3600, mc.WithInitialValue(1))
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Counter after first increment: %d\n", newValue)

	// Increment again
	newValue, err = memcache.Inc(ctx, key, 5, 3600)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Counter after increment by 5: %d\n", newValue)
	// Output:
	// Counter after first increment: 1
	// Counter after increment by 5: 6
}

func ExampleClient_Dec() {
	memcache, err := mc.New(&mc.Options{
		Addrs: []string{"127.0.0.1:11211"},
	})
	if err != nil {
		log.Fatal(err)
	}
	defer memcache.Close()

	ctx := context.Background()

	// Set initial value
	err = memcache.Set(ctx, &mc.Item{
		Key:   "stock",
		Value: []byte("100"),
	}, mc.WithExpiration(3600))
	if err != nil {
		log.Fatal(err)
	}

	// Decrement
	newValue, err := memcache.Dec(ctx, "stock", 10, 3600)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Stock after decrement: %d\n", newValue)
	// Output: Stock after decrement: 90
}

func ExampleClient_Del() {
	memcache, err := mc.New(&mc.Options{
		Addrs: []string{"127.0.0.1:11211"},
	})
	if err != nil {
		log.Fatal(err)
	}
	defer memcache.Close()

	ctx := context.Background()

	// Set a value
	err = memcache.Set(ctx, &mc.Item{
		Key:   "temp",
		Value: []byte("data"),
	}, mc.WithExpiration(60))
	if err != nil {
		log.Fatal(err)
	}

	// Delete the key
	err = memcache.Del(ctx, "temp")
	if err != nil {
		log.Fatal(err)
	}

	// Try to get deleted key
	_, err = memcache.Get(ctx, "temp")
	if err == mc.ErrCacheMiss {
		fmt.Println("Key successfully deleted")
	}
	// Output: Key successfully deleted
}

func ExampleWithCompression() {
	memcache, err := mc.New(&mc.Options{
		Addrs: []string{"127.0.0.1:11211"},
	})
	if err != nil {
		log.Fatal(err)
	}
	defer memcache.Close()

	ctx := context.Background()

	// Create large value (will be compressed if > 1024 bytes)
	largeData := make([]byte, 2048)
	for i := range largeData {
		largeData[i] = byte(i % 256)
	}

	err = memcache.Set(ctx, &mc.Item{
		Key:   "large_data",
		Value: largeData,
	}, mc.WithCompression(1024), mc.WithExpiration(60))
	if err != nil {
		log.Fatal(err)
	}

	// Get and verify
	item, err := memcache.Get(ctx, "large_data")
	if err != nil {
		log.Fatal(err)
	}

	if len(item.Value) == len(largeData) {
		fmt.Println("Data retrieved and decompressed successfully")
	}
	// Output: Data retrieved and decompressed successfully
}

func ExampleContext() {
	memcache, err := mc.New(&mc.Options{
		Addrs: []string{"127.0.0.1:11211"},
	})
	if err != nil {
		log.Fatal(err)
	}
	defer memcache.Close()

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err = memcache.Set(ctx, &mc.Item{
		Key:   "key",
		Value: []byte("value"),
	}, mc.WithExpiration(60))
	if err != nil {
		if err == context.DeadlineExceeded {
			fmt.Println("Operation timed out")
		} else {
			log.Fatal(err)
		}
	} else {
		fmt.Println("Operation completed")
	}
	// Output: Operation completed
}

func ExampleWithCAS() {
	memcache, err := mc.New(&mc.Options{
		Addrs: []string{"127.0.0.1:11211"},
	})
	if err != nil {
		log.Fatal(err)
	}
	defer memcache.Close()

	ctx := context.Background()

	// Set initial value
	err = memcache.Set(ctx, &mc.Item{
		Key:   "data",
		Value: []byte("initial"),
	}, mc.WithExpiration(60))
	if err != nil {
		log.Fatal(err)
	}

	// Get with CAS to retrieve CAS value for optimistic locking
	item, err := memcache.Get(ctx, "data", mc.WithCAS())
	if err != nil {
		log.Fatal(err)
	}

	// Use CAS value for CompareAndSwap operation
	item.Value = []byte("updated")
	err = memcache.CompareAndSwap(ctx, item)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Updated using CAS")
	// Output: Updated using CAS
}

func ExampleWithMinUses() {
	memcache, err := mc.New(&mc.Options{
		Addrs: []string{"127.0.0.1:11211"},
	})
	if err != nil {
		log.Fatal(err)
	}
	defer memcache.Close()

	ctx := context.Background()

	// Set with min uses - item will only be stored if accessed at least N times
	err = memcache.Set(ctx, &mc.Item{
		Key:   "popular",
		Value: []byte("data"),
	}, mc.WithExpiration(60), mc.WithMinUses(3))
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Item set with min uses requirement")
	// Output: Item set with min uses requirement
}

func ExampleWithEarlyRecache() {
	memcache, err := mc.New(&mc.Options{
		Addrs: []string{"127.0.0.1:11211"},
	})
	if err != nil {
		log.Fatal(err)
	}
	defer memcache.Close()

	ctx := context.Background()

	// Set a value
	err = memcache.Set(ctx, &mc.Item{
		Key:   "cacheable",
		Value: []byte("data"),
	}, mc.WithExpiration(3600))
	if err != nil {
		log.Fatal(err)
	}

	// Get with early recache - will return stale data if within recache window
	item, err := memcache.Get(ctx, "cacheable", mc.WithEarlyRecache(300))
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Value: %s\n", item.Value)
	// Output: Value: data
}

func ExampleWithLastAccess() {
	memcache, err := mc.New(&mc.Options{
		Addrs: []string{"127.0.0.1:11211"},
	})
	if err != nil {
		log.Fatal(err)
	}
	defer memcache.Close()

	ctx := context.Background()

	// Set a value
	err = memcache.Set(ctx, &mc.Item{
		Key:   "tracked",
		Value: []byte("data"),
	}, mc.WithExpiration(3600))
	if err != nil {
		log.Fatal(err)
	}

	// Get with last access time
	item, err := memcache.Get(ctx, "tracked", mc.WithLastAccess())
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Retrieved: %s\n", item.Value)
	// Output: Retrieved: data
}

func ExampleClient_Close() {
	memcache, err := mc.New(&mc.Options{
		Addrs: []string{"127.0.0.1:11211"},
	})
	if err != nil {
		log.Fatal(err)
	}

	// Always close the client when done
	defer memcache.Close()

	ctx := context.Background()
	err = memcache.Set(ctx, &mc.Item{
		Key:   "key",
		Value: []byte("value"),
	}, mc.WithExpiration(60))
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Client operations completed")
	// Output: Client operations completed
}
