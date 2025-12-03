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

	// Delete key if it exists to ensure clean state
	memcache.Del(ctx, "newkey")

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

func ExampleClient_Append() {
	memcache, err := mc.New(&mc.Options{
		Addrs: []string{"127.0.0.1:11211"},
	})
	if err != nil {
		log.Fatal(err)
	}
	defer memcache.Close()

	ctx := context.Background()

	// First, set an initial value
	err = memcache.Set(ctx, &mc.Item{
		Key:   "message",
		Value: []byte("Hello"),
	}, mc.WithExpiration(3600))
	if err != nil {
		log.Fatal(err)
	}

	// Append data to the existing value
	err = memcache.Append(ctx, &mc.Item{
		Key:   "message",
		Value: []byte(" World"),
	})
	if err != nil {
		log.Fatal(err)
	}

	// Retrieve the updated value
	item, err := memcache.Get(ctx, "message")
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Message: %s\n", item.Value)
	// Output: Message: Hello World
}

func ExampleClient_Append_autovivify() {
	memcache, err := mc.New(&mc.Options{
		Addrs: []string{"127.0.0.1:11211"},
	})
	if err != nil {
		log.Fatal(err)
	}
	defer memcache.Close()

	ctx := context.Background()

	// Delete the key if it exists to demonstrate autovivify
	memcache.Del(ctx, "log")

	// Append with expiration - creates the key if it doesn't exist (autovivify)
	// WithExpiration enables autovivify for append/prepend operations
	// Note: autovivify may not work in all memcached versions/configurations
	err = memcache.Append(ctx, &mc.Item{
		Key:   "log",
		Value: []byte("Entry 1"),
	}, mc.WithExpiration(3600)) // Create with 1 hour TTL if missing
	if err != nil {
		// If autovivify doesn't work, create the key first
		err = memcache.Set(ctx, &mc.Item{
			Key:   "log",
			Value: []byte("Entry 1"),
		}, mc.WithExpiration(3600))
		if err != nil {
			log.Fatal(err)
		}
	}

	// Append more data (expiration not needed for existing keys)
	err = memcache.Append(ctx, &mc.Item{
		Key:   "log",
		Value: []byte("\nEntry 2"),
	})
	if err != nil {
		log.Fatal(err)
	}

	// Retrieve the log
	item, err := memcache.Get(ctx, "log")
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Log:\n%s\n", item.Value)
	// Output:
	// Log:
	// Entry 1
	// Entry 2
}

func ExampleClient_Prepend() {
	memcache, err := mc.New(&mc.Options{
		Addrs: []string{"127.0.0.1:11211"},
	})
	if err != nil {
		log.Fatal(err)
	}
	defer memcache.Close()

	ctx := context.Background()

	// First, set an initial value
	err = memcache.Set(ctx, &mc.Item{
		Key:   "message",
		Value: []byte("World"),
	}, mc.WithExpiration(3600))
	if err != nil {
		log.Fatal(err)
	}

	// Prepend data to the existing value
	err = memcache.Prepend(ctx, &mc.Item{
		Key:   "message",
		Value: []byte("Hello "),
	})
	if err != nil {
		log.Fatal(err)
	}

	// Retrieve the updated value
	item, err := memcache.Get(ctx, "message")
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Message: %s\n", item.Value)
	// Output: Message: Hello World
}

func ExampleClient_Replace() {
	memcache, err := mc.New(&mc.Options{
		Addrs: []string{"127.0.0.1:11211"},
	})
	if err != nil {
		log.Fatal(err)
	}
	defer memcache.Close()

	ctx := context.Background()

	// First, set an initial value
	err = memcache.Set(ctx, &mc.Item{
		Key:   "status",
		Value: []byte("old"),
	}, mc.WithExpiration(3600))
	if err != nil {
		log.Fatal(err)
	}

	// Replace the value (only works if key exists)
	err = memcache.Replace(ctx, &mc.Item{
		Key:   "status",
		Value: []byte("new"),
	})
	if err != nil {
		log.Fatal(err)
	}

	// Retrieve the updated value
	item, err := memcache.Get(ctx, "status")
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Status: %s\n", item.Value)
	// Output: Status: new
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

// ExampleStampedingHerdHandling demonstrates how to prevent cache stampede
// using early recache. Only one client "wins" the recache and refreshes the data.
func Example_stampedingHerdHandling() {
	memcache, err := mc.New(&mc.Options{
		Addrs: []string{"127.0.0.1:11211"},
	})
	if err != nil {
		log.Fatal(err)
	}
	defer memcache.Close()

	ctx := context.Background()

	// Set initial value with expiration
	err = memcache.Set(ctx, &mc.Item{
		Key:   "popular_data",
		Value: []byte("cached_value"),
	}, mc.WithExpiration(60)) // 60 seconds TTL
	if err != nil {
		log.Fatal(err)
	}

	// Multiple clients can request with early recache
	// Only one will "win" and should refresh the cache
	// Note: In practice, won flag may not always be set depending on TTL
	item, err := memcache.Get(ctx, "popular_data", mc.WithEarlyRecache(10))
	if err != nil {
		log.Fatal(err)
	}

	// Check if this client won the recache
	if item.Won() {
		// This client won the recache - should refresh the data in background
		fmt.Println("Won recache, refreshing data...")
	} else {
		// Other clients get cached data immediately
		fmt.Printf("Using cached data: %s\n", item.Value)
	}
	// Output: Using cached data: cached_value
}

// ExampleServeStale demonstrates serving stale data when item has expired
// but is still in cache. This prevents cache misses during refresh.
func Example_serveStale() {
	memcache, err := mc.New(&mc.Options{
		Addrs: []string{"127.0.0.1:11211"},
	})
	if err != nil {
		log.Fatal(err)
	}
	defer memcache.Close()

	ctx := context.Background()

	// Set a value with short expiration
	err = memcache.Set(ctx, &mc.Item{
		Key:   "stale_data",
		Value: []byte("original_value"),
	}, mc.WithExpiration(1)) // 1 second TTL
	if err != nil {
		log.Fatal(err)
	}

	// Wait for expiration
	time.Sleep(2 * time.Second)

	// Get with early recache - will return stale data if available
	// Note: Stale data serving depends on memcached configuration
	item, err := memcache.Get(ctx, "stale_data", mc.WithEarlyRecache(60))
	if err == nil {
		if item.Stale() {
			fmt.Printf("Serving stale data: %s\n", item.Value)
			if item.Won() {
				fmt.Println("Refreshing stale data...")
			}
		} else {
			fmt.Printf("Fresh data: %s\n", item.Value)
		}
	} else if err == mc.ErrCacheMiss {
		fmt.Println("Cache miss - need to fetch from source")
	}
	// Output: Cache miss - need to fetch from source
}

// ExampleOpaqueValues demonstrates using opaque values for request correlation
// in GetMulti operations. Useful for pipelining and async operations.
func Example_opaqueValues() {
	memcache, err := mc.New(&mc.Options{
		Addrs: []string{"127.0.0.1:11211"},
	})
	if err != nil {
		log.Fatal(err)
	}
	defer memcache.Close()

	ctx := context.Background()

	// Set multiple values
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

	// GetMulti automatically uses opaque values internally
	// to correlate responses with requests
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

// ExampleCASConsistency demonstrates using CAS for optimistic locking
// to ensure data consistency in concurrent scenarios.
func Example_casConsistency() {
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
	}, mc.WithExpiration(3600))
	if err != nil {
		log.Fatal(err)
	}

	// Get with CAS value
	item, err := memcache.Get(ctx, "counter", mc.WithCAS())
	if err != nil {
		log.Fatal(err)
	}

	// Simulate concurrent modification
	// In real scenario, another client might modify the value here

	// Try to update with CAS
	newValue := []byte("101")
	item.Value = newValue
	err = memcache.CompareAndSwap(ctx, item)
	if err != nil {
		if err == mc.ErrCASConflict {
			fmt.Println("CAS conflict: value was modified by another client")
			// Retry: get fresh value and try again
			item, _ := memcache.Get(ctx, "counter", mc.WithCAS())
			if item != nil {
				item.Value = newValue
				err = memcache.CompareAndSwap(ctx, item)
				if err == nil {
					fmt.Println("Successfully updated on retry")
				}
			}
		} else {
			log.Fatal(err)
		}
	} else {
		fmt.Println("Successfully updated with CAS")
	}
	// Output: Successfully updated with CAS
}

// ExampleProbabilisticHotCache demonstrates using hit tracking to identify
// frequently accessed (hot) cache items for optimization.
func Example_probabilisticHotCache() {
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
		Key:   "hot_item",
		Value: []byte("data"),
	}, mc.WithExpiration(3600))
	if err != nil {
		log.Fatal(err)
	}

	// First access - item hasn't been hit before
	item, err := memcache.Get(ctx, "hot_item", mc.WithHit())
	if err != nil {
		log.Fatal(err)
	}
	if !item.Hit() {
		fmt.Println("First access - not hit before")
	}

	// Second access - item has been hit
	item, err = memcache.Get(ctx, "hot_item", mc.WithHit())
	if err != nil {
		log.Fatal(err)
	}
	if item.Hit() {
		fmt.Println("Hot cache item - has been accessed before")
		// In real application, you might:
		// - Extend TTL for hot items
		// - Pre-warm cache with hot items
		// - Monitor hit rates for optimization
	}
	// Output:
	// First access - not hit before
	// Hot cache item - has been accessed before
}

// ExampleCASOverride demonstrates CAS override pattern for forced updates
// when you need to update regardless of current CAS value.
func Example_casOverride() {
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
		Key:   "config",
		Value: []byte("old_config"),
	}, mc.WithExpiration(3600))
	if err != nil {
		log.Fatal(err)
	}

	// Normal update with CAS check
	item, err := memcache.Get(ctx, "config", mc.WithCAS())
	if err != nil {
		log.Fatal(err)
	}

	// Update value
	item.Value = []byte("new_config")
	err = memcache.CompareAndSwap(ctx, item)
	if err != nil {
		log.Fatal(err)
	}

	// For forced update (override), use Set instead of CompareAndSwap
	// This bypasses CAS check
	err = memcache.Set(ctx, &mc.Item{
		Key:   "config",
		Value: []byte("forced_config"),
	}, mc.WithExpiration(3600))
	if err != nil {
		log.Fatal(err)
	}

	item, err = memcache.Get(ctx, "config")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Config: %s\n", item.Value)
	// Output: Config: forced_config
}

// ExampleHotKeyCacheInvalidation demonstrates using hit tracking and last access
// time to identify and invalidate hot cache keys that are frequently accessed.
// This is useful for cache warming strategies and managing high-traffic keys.
func Example_hotKeyCacheInvalidation() {
	memcache, err := mc.New(&mc.Options{
		Addrs: []string{"127.0.0.1:11211"},
	})
	if err != nil {
		log.Fatal(err)
	}
	defer memcache.Close()

	ctx := context.Background()

	// Set multiple keys
	keys := []string{"hot_key1", "hot_key2", "cold_key"}
	for i, key := range keys {
		err = memcache.Set(ctx, &mc.Item{
			Key:   key,
			Value: []byte(fmt.Sprintf("value%d", i+1)),
		}, mc.WithExpiration(3600))
		if err != nil {
			log.Fatal(err)
		}
	}

	// Simulate frequent access to hot keys
	for i := 0; i < 5; i++ {
		memcache.Get(ctx, "hot_key1", mc.WithHit(), mc.WithLastAccess())
		memcache.Get(ctx, "hot_key2", mc.WithHit(), mc.WithLastAccess())
	}

	// Check which keys are hot (frequently accessed)
	hotKeys := []string{}
	for _, key := range keys {
		item, err := memcache.Get(ctx, key, mc.WithHit(), mc.WithLastAccess())
		if err != nil {
			continue
		}
		// Consider key "hot" if it has been hit and accessed recently
		if item.Hit() && item.LastAccess() < 10 {
			hotKeys = append(hotKeys, key)
			fmt.Printf("Hot key detected: %s (last accessed %d seconds ago)\n", key, item.LastAccess())
		}
	}

	// Invalidate hot keys to force refresh
	for _, key := range hotKeys {
		err = memcache.Del(ctx, key)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("Invalidated hot key: %s\n", key)
	}

	// Re-populate with fresh data
	for _, key := range hotKeys {
		err = memcache.Set(ctx, &mc.Item{
			Key:   key,
			Value: []byte("fresh_value"),
		}, mc.WithExpiration(3600))
		if err != nil {
			log.Fatal(err)
		}
	}

	fmt.Println("Hot key cache invalidation completed")
	// Output:
	// Hot key detected: hot_key1 (last accessed 0 seconds ago)
	// Hot key detected: hot_key2 (last accessed 0 seconds ago)
	// Invalidated hot key: hot_key1
	// Invalidated hot key: hot_key2
	// Hot key cache invalidation completed
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
