# go-concurrent-lru-cache

A concurrency-safe, in-memory LRU cache written in Go. Keys and values are
strings. Backend library only, plus a minimal CLI demo and automated tests.
No third-party dependencies.

## Features

- O(1) `Put` / `Get` / `Delete` via hash map + doubly linked list
- `Get` promotes the entry to most recently used; `Peek` reads without
  changing the order
- Atomic eviction of the least recently used entry when full
- `Resize` shrinks (evicting oldest first) or grows (keeping content)
- `Snapshot` returns an independent copy ordered MRU to LRU
- Safe for concurrent use from multiple goroutines

## Layout

- `cache.go` — the `lrucache` library
- `cache_test.go` — unit tests and a basic concurrency test
- `cmd/demo/main.go` — minimal CLI demo

## API

```go
c, _ := lrucache.New(128)   // positive capacity, else error
_ = c.Put("k", "v")         // error on empty key
v, ok := c.Get("k")         // hit moves k to most recently used
v, ok = c.Peek("k")         // read-only, order unchanged
ok = c.Delete("k")
_ = c.Resize(64)            // positive, else error
items := c.Snapshot()       // []lrucache.Item, MRU -> LRU, independent copy
n := c.Len()
cap := c.Capacity()
```

## Run the demo

```sh
go run ./cmd/demo
```

## Run the tests

```sh
go test -race ./...
```
