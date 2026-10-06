// Package lrucache provides a concurrency-safe, fixed-capacity, in-memory
// LRU cache with string keys and string values.
//
// The cache is implemented with a hash table plus a doubly linked list:
// the map gives O(1) lookups and the list tracks usage order from most
// recently used (head) to least recently used (tail). A single mutex
// guards all state, so the map, the list and evictions always stay
// consistent under concurrent use.
package lrucache

import (
	"errors"
	"sync"
)

// Sentinel errors returned for invalid arguments.
var (
	ErrInvalidCapacity = errors.New("lrucache: capacity must be a positive integer")
	ErrEmptyKey        = errors.New("lrucache: key must not be empty")
)

// Entry is a single key/value pair as returned by Snapshot.
type Entry struct {
	Key   string
	Value string
}

// node is one element of the doubly linked list.
type node struct {
	key   string
	value string
	prev  *node // towards the most recently used end
	next  *node // towards the least recently used end
}

// Cache is a concurrency-safe LRU cache. Create one with New.
type Cache struct {
	mu       sync.Mutex
	capacity int
	items    map[string]*node
	head     *node // most recently used
	tail     *node // least recently used
}

// New creates a Cache that holds at most capacity entries.
// capacity must be a positive integer.
func New(capacity int) (*Cache, error) {
	if capacity <= 0 {
		return nil, ErrInvalidCapacity
	}
	return &Cache{
		capacity: capacity,
		items:    make(map[string]*node, capacity),
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

	if n, ok := c.items[key]; ok {
		n.value = value
		c.moveToFront(n)
		return nil
	}
	n := &node{key: key, value: value}
	c.items[key] = n
	c.pushFront(n)
	if len(c.items) > c.capacity {
		c.evictOldest()
	}
	return nil
}

// Get returns the value for key and marks it most recently used.
// The second return value reports whether the key was present.
func (c *Cache) Get(key string) (string, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	n, ok := c.items[key]
	if !ok {
		return "", false
	}
	c.moveToFront(n)
	return n.value, true
}

// Peek returns the value for key without changing the LRU order.
func (c *Cache) Peek(key string) (string, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	n, ok := c.items[key]
	if !ok {
		return "", false
	}
	return n.value, true
}

// Delete removes key from the cache and reports whether it was present.
func (c *Cache) Delete(key string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	n, ok := c.items[key]
	if !ok {
		return false
	}
	c.remove(n)
	delete(c.items, key)
	return true
}

// Resize changes the capacity, which must be a positive integer.
// Shrinking evicts least recently used entries until the cache fits;
// growing keeps all existing entries.
func (c *Cache) Resize(capacity int) error {
	if capacity <= 0 {
		return ErrInvalidCapacity
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	c.capacity = capacity
	for len(c.items) > c.capacity {
		c.evictOldest()
	}
	return nil
}

// Snapshot returns an independent copy of the cache contents ordered
// from most recently used to least recently used.
func (c *Cache) Snapshot() []Entry {
	c.mu.Lock()
	defer c.mu.Unlock()

	entries := make([]Entry, 0, len(c.items))
	for n := c.head; n != nil; n = n.next {
		entries = append(entries, Entry{Key: n.key, Value: n.value})
	}
	return entries
}

// Len returns the number of entries currently stored.
func (c *Cache) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.items)
}

// Capacity returns the current maximum number of entries.
func (c *Cache) Capacity() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.capacity
}

// pushFront inserts n at the most recently used end of the list.
// Callers must hold c.mu and have already stored n in c.items.
func (c *Cache) pushFront(n *node) {
	n.prev = nil
	n.next = c.head
	if c.head != nil {
		c.head.prev = n
	}
	c.head = n
	if c.tail == nil {
		c.tail = n
	}
}

// remove unlinks n from the list. Callers must hold c.mu.
func (c *Cache) remove(n *node) {
	if n.prev != nil {
		n.prev.next = n.next
	} else {
		c.head = n.next
	}
	if n.next != nil {
		n.next.prev = n.prev
	} else {
		c.tail = n.prev
	}
	n.prev = nil
	n.next = nil
}

// moveToFront marks n as most recently used. Callers must hold c.mu.
func (c *Cache) moveToFront(n *node) {
	if c.head == n {
		return
	}
	c.remove(n)
	c.pushFront(n)
}

// evictOldest removes the least recently used entry from both the list
// and the map. Callers must hold c.mu.
func (c *Cache) evictOldest() {
	oldest := c.tail
	if oldest == nil {
		return
	}
	c.remove(oldest)
	delete(c.items, oldest.key)
}
