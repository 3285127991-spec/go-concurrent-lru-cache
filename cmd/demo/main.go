// Command demo shows the lrucache API with a few built-in items.
package main

import (
	"fmt"
	"log"

	"lrucache"
)

func main() {
	cache, err := lrucache.New(2)
	if err != nil {
		log.Fatal(err)
	}
	put := func(key, value string) {
		if err := cache.Put(key, value); err != nil {
			log.Fatal(err)
		}
	}

	fmt.Println("== Put a=1, b=2 (capacity 2) ==")
	put("a", "1")
	put("b", "2")
	printSnapshot(cache)

	fmt.Println("== Get a (moves a to most recently used) ==")
	if v, ok := cache.Get("a"); ok {
		fmt.Printf("Get(a) = %s\n", v)
	}
	printSnapshot(cache)

	fmt.Println("== Put c=3 (evicts least recently used b) ==")
	put("c", "3")
	printSnapshot(cache)

	fmt.Println("== Peek a (order unchanged) ==")
	if v, ok := cache.Peek("a"); ok {
		fmt.Printf("Peek(a) = %s\n", v)
	}
	printSnapshot(cache)

	fmt.Println("== Resize to 1 (evicts down to the newest entry) ==")
	if err := cache.Resize(1); err != nil {
		log.Fatal(err)
	}
	printSnapshot(cache)

	fmt.Println("== Resize to 3, Put x=24, y=25 (existing content kept) ==")
	if err := cache.Resize(3); err != nil {
		log.Fatal(err)
	}
	put("x", "24")
	put("y", "25")
	printSnapshot(cache)

	fmt.Printf("Len=%d Capacity=%d\n", cache.Len(), cache.Capacity())
}

func printSnapshot(c *lrucache.Cache) {
	fmt.Print("Snapshot (MRU -> LRU): ")
	for i, item := range c.Snapshot() {
		if i > 0 {
			fmt.Print(" -> ")
		}
		fmt.Printf("%s=%s", item.Key, item.Value)
	}
	fmt.Println()
}
