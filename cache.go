// Package lrucache provides a concurrency-safe in-memory LRU cache
// with string keys and string values.
package lrucache

import (
	"container/list"
	"errors"
	"sync"
)

var (
	// ErrInvalidCapacity is returned when a capacity is not a positive integer.
	ErrInvalidCapacity = errors.New("lrucache: capacity must be a positive integer")
	// ErrEmptyKey is returned when an empty key is supplied.
	ErrEmptyKey = errors.New("lrucache: key must not be empty")
)

// Item is a single key/value pair as returned by Snapshot.
type Item struct {
	Key   string
	Value string
}

type entry struct {
	key   string
	value string
}

// Cache is a concurrency-safe LRU cache backed by a hash map and a
// doubly linked list. The front of the list is the most recently used
// entry, the back is the least recently used one. All methods are safe
// for concurrent use by multiple goroutines.
type Cache struct {
	mu    sync.RWMutex
	cap   int
	ll    *list.List
	items map[string]*list.Element
}

// New creates a Cache with the given positive capacity.
func New(capacity int) (*Cache, error) {
	if capacity <= 0 {
		return nil, ErrInvalidCapacity
	}
	return &Cache{
		cap:   capacity,
		ll:    list.New(),
		items: make(map[string]*list.Element),
	}, nil
}

// Put inserts or updates key with value and marks it most recently used.
// When the cache is full, the least recently used entry is evicted
// atomically as part of the same locked operation.
func (c *Cache) Put(key, value string) error {
	if key == "" {
		return ErrEmptyKey
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if el, ok := c.items[key]; ok {
		el.Value.(*entry).value = value
		c.ll.MoveToFront(el)
		return nil
	}
	c.items[key] = c.ll.PushFront(&entry{key: key, value: value})
	if c.ll.Len() > c.cap {
		c.evictOldest()
	}
	return nil
}

// Get returns the value for key and marks it most recently used.
func (c *Cache) Get(key string) (string, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	el, ok := c.items[key]
	if !ok {
		return "", false
	}
	c.ll.MoveToFront(el)
	return el.Value.(*entry).value, true
}

// Peek returns the value for key without changing the LRU order.
func (c *Cache) Peek(key string) (string, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	el, ok := c.items[key]
	if !ok {
		return "", false
	}
	return el.Value.(*entry).value, true
}

// Delete removes key and reports whether it was present.
func (c *Cache) Delete(key string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	el, ok := c.items[key]
	if !ok {
		return false
	}
	c.removeElement(el)
	return true
}

// Resize sets a new positive capacity. Shrinking evicts least recently
// used entries immediately; growing keeps all existing entries.
func (c *Cache) Resize(capacity int) error {
	if capacity <= 0 {
		return ErrInvalidCapacity
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cap = capacity
	for c.ll.Len() > c.cap {
		c.evictOldest()
	}
	return nil
}

// Snapshot returns an independent copy of the cache contents ordered
// from most recently used to least recently used.
func (c *Cache) Snapshot() []Item {
	c.mu.RLock()
	defer c.mu.RUnlock()
	items := make([]Item, 0, c.ll.Len())
	for el := c.ll.Front(); el != nil; el = el.Next() {
		e := el.Value.(*entry)
		items = append(items, Item{Key: e.key, Value: e.value})
	}
	return items
}

// Len returns the current number of entries.
func (c *Cache) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.ll.Len()
}

// Capacity returns the current capacity.
func (c *Cache) Capacity() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.cap
}

// evictOldest drops the least recently used entry.
// The caller must hold the write lock.
func (c *Cache) evictOldest() {
	if el := c.ll.Back(); el != nil {
		c.removeElement(el)
	}
}

// removeElement unlinks el and deletes its key from the map.
// The caller must hold the write lock.
func (c *Cache) removeElement(el *list.Element) {
	c.ll.Remove(el)
	delete(c.items, el.Value.(*entry).key)
}
