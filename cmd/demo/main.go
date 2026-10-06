// Command demo is a minimal walkthrough of the lrucache package using
// a few built-in values: Put, Get reordering, capacity eviction, Peek,
// Resize and Snapshot.
package main

import (
	"fmt"
	"log"

	"lrucache"
)

func must(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

func printSnapshot(c *lrucache.Cache) {
	fmt.Print("  Snapshot (MRU -> LRU):")
	for _, e := range c.Snapshot() {
		fmt.Printf(" %s=%s", e.Key, e.Value)
	}
	fmt.Println()
}

func main() {
	cache, err := lrucache.New(2)
	must(err)

	fmt.Println("== Put a=1, b=2 (capacity 2) ==")
	must(cache.Put("a", "1"))
	must(cache.Put("b", "2"))
	printSnapshot(cache)

	fmt.Println("== Get a -> a becomes most recently used ==")
	if v, ok := cache.Get("a"); ok {
		fmt.Printf("  Get(a) = %s\n", v)
	}
	printSnapshot(cache)

	fmt.Println("== Put c=3 -> b is evicted (least recently used) ==")
	must(cache.Put("c", "3"))
	printSnapshot(cache)

	fmt.Println("== Peek c -> order unchanged ==")
	if v, ok := cache.Peek("c"); ok {
		fmt.Printf("  Peek(c) = %s\n", v)
	}
	printSnapshot(cache)

	fmt.Println("== Resize to 3, then Put d=4 (existing entries kept) ==")
	must(cache.Resize(3))
	must(cache.Put("d", "4"))
	printSnapshot(cache)

	fmt.Println("== Resize to 1 -> evicts down to the most recent entry ==")
	must(cache.Resize(1))
	printSnapshot(cache)

	fmt.Printf("== Len=%d Capacity=%d ==\n", cache.Len(), cache.Capacity())
}
