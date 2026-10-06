package lrucache

import (
	"errors"
	"fmt"
	"sync"
	"testing"
)

func mustNew(t *testing.T, capacity int) *Cache {
	t.Helper()
	c, err := New(capacity)
	if err != nil {
		t.Fatalf("New(%d) returned unexpected error: %v", capacity, err)
	}
	return c
}

func mustPut(t *testing.T, c *Cache, key, value string) {
	t.Helper()
	if err := c.Put(key, value); err != nil {
		t.Fatalf("Put(%q, %q) returned unexpected error: %v", key, value, err)
	}
}

func snapshotKeys(c *Cache) []string {
	entries := c.Snapshot()
	keys := make([]string, len(entries))
	for i, e := range entries {
		keys[i] = e.Key
	}
	return keys
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// checkInvariants verifies internal consistency between the map and the
// doubly linked list: no duplicate or cyclic nodes, no capacity overflow
// overflow, and both structures holding exactly the same keys.
func checkInvariants(t *testing.T, c *Cache) {
	t.Helper()
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.capacity <= 0 {
		t.Fatalf("capacity must stay positive, got %d", c.capacity)
	}
	if len(c.items) > c.capacity {
		t.Fatalf("len(items)=%d exceeds capacity=%d", len(c.items), c.capacity)
	}

	count := 0
	seen := make(map[*node]bool, len(c.items))
	var last *node
	for n := c.head; n != nil; n = n.next {
		if seen[n] {
			t.Fatal("duplicate node / cycle detected in list")
		}
		seen[n] = true
		count++
		if n.next != nil && n.next.prev != n {
			t.Fatalf("broken link: node %q is not prev of its next", n.key)
		}
		if got, ok := c.items[n.key]; !ok || got != n {
			t.Fatalf("list node %q missing from map (or map points elsewhere)", n.key)
		}
		last = n
	}
	if count != len(c.items) {
		t.Fatalf("list holds %d nodes but map holds %d entries", count, len(c.items))
	}
	if last != c.tail {
		t.Fatal("tail does not point at the last list node")
	}
	if (c.head == nil) != (c.tail == nil) {
		t.Fatal("head and tail disagree on list emptiness")
	}
}

func TestPutAndGet(t *testing.T) {
	c := mustNew(t, 2)
	mustPut(t, c, "a", "1")

	if v, ok := c.Get("a"); !ok || v != "1" {
		t.Fatalf("Get(a) = %q, %v; want %q, true", v, ok, "1")
	}
	if _, ok := c.Get("missing"); ok {
		t.Fatal("Get(missing) reported a hit")
	}
	if got := c.Len(); got != 1 {
		t.Fatalf("Len() = %d; want 1", got)
	}
	if got := c.Capacity(); got != 2 {
		t.Fatalf("Capacity() = %d; want 2", got)
	}
	checkInvariants(t, c)
}

func TestPutUpdatesExistingKey(t *testing.T) {
	c := mustNew(t, 2)
	mustPut(t, c, "a", "1")
	mustPut(t, c, "b", "2")
	mustPut(t, c, "a", "updated")

	if got := c.Len(); got != 2 {
		t.Fatalf("Len() = %d; want 2 (update must not insert a new entry)", got)
	}
	if v, ok := c.Peek("a"); !ok || v != "updated" {
		t.Fatalf("Peek(a) = %q, %v; want %q, true", v, ok, "updated")
	}
	// The update marks "a" as most recently used.
	if keys := snapshotKeys(c); !equalStrings(keys, []string{"a", "b"}) {
		t.Fatalf("order = %v; want [a b]", keys)
	}
	checkInvariants(t, c)
}

func TestEvictsLeastRecentlyUsed(t *testing.T) {
	c := mustNew(t, 2)
	mustPut(t, c, "a", "1")
	mustPut(t, c, "b", "2")
	mustPut(t, c, "c", "3") // evicts "a"

	if _, ok := c.Peek("a"); ok {
		t.Fatal("expected a to be evicted")
	}
	if _, ok := c.Peek("b"); !ok {
		t.Fatal("expected b to survive")
	}
	if _, ok := c.Peek("c"); !ok {
		t.Fatal("expected c to survive")
	}
	if got := c.Len(); got != 2 {
		t.Fatalf("Len() = %d; want 2", got)
	}
	checkInvariants(t, c)
}

func TestGetMovesToFront(t *testing.T) {
	c := mustNew(t, 2)
	mustPut(t, c, "a", "1")
	mustPut(t, c, "b", "2")

	if _, ok := c.Get("a"); !ok {
		t.Fatal("Get(a) missed")
	}
	mustPut(t, c, "c", "3") // must evict "b", not the recently used "a"

	if _, ok := c.Peek("b"); ok {
		t.Fatal("expected b to be evicted after a was refreshed by Get")
	}
	if _, ok := c.Peek("a"); !ok {
		t.Fatal("expected a to survive")
	}
	if keys := snapshotKeys(c); !equalStrings(keys, []string{"c", "a"}) {
		t.Fatalf("order = %v; want [c a]", keys)
	}
	checkInvariants(t, c)
}

func TestPeekDoesNotChangeOrder(t *testing.T) {
	c := mustNew(t, 2)
	mustPut(t, c, "a", "1")
	mustPut(t, c, "b", "2")

	if v, ok := c.Peek("a"); !ok || v != "1" {
		t.Fatalf("Peek(a) = %q, %v; want %q, true", v, ok, "1")
	}
	if keys := snapshotKeys(c); !equalStrings(keys, []string{"b", "a"}) {
		t.Fatalf("order after Peek = %v; want [b a] (unchanged)", keys)
	}

	mustPut(t, c, "c", "3") // must evict "a": Peek did not refresh it
	if _, ok := c.Peek("a"); ok {
		t.Fatal("expected a to be evicted; Peek must not refresh entries")
	}
	checkInvariants(t, c)
}

func TestDelete(t *testing.T) {
	c := mustNew(t, 3)
	mustPut(t, c, "a", "1")
	mustPut(t, c, "b", "2")
	mustPut(t, c, "c", "3")

	if !c.Delete("b") {
		t.Fatal("Delete(b) = false; want true")
	}
	if c.Delete("b") {
		t.Fatal("Delete(b) again = true; want false")
	}
	if c.Delete("missing") {
		t.Fatal("Delete(missing) = true; want false")
	}
	if _, ok := c.Peek("b"); ok {
		t.Fatal("deleted key b is still visible")
	}
	if got := c.Len(); got != 2 {
		t.Fatalf("Len() = %d; want 2", got)
	}
	if keys := snapshotKeys(c); !equalStrings(keys, []string{"c", "a"}) {
		t.Fatalf("order = %v; want [c a]", keys)
	}
	checkInvariants(t, c)
}

func TestResizeShrink(t *testing.T) {
	c := mustNew(t, 4)
	mustPut(t, c, "a", "1")
	mustPut(t, c, "b", "2")
	mustPut(t, c, "c", "3")
	mustPut(t, c, "d", "4")

	if err := c.Resize(2); err != nil {
		t.Fatalf("Resize(2) returned unexpected error: %v", err)
	}
	if got := c.Capacity(); got != 2 {
		t.Fatalf("Capacity() = %d; want 2", got)
	}
	if got := c.Len(); got != 2 {
		t.Fatalf("Len() = %d; want 2 after shrinking", got)
	}
	// Oldest entries a and b must be gone; newest c and d survive.
	if keys := snapshotKeys(c); !equalStrings(keys, []string{"d", "c"}) {
		t.Fatalf("order = %v; want [d c]", keys)
	}
	checkInvariants(t, c)
}

func TestResizeGrow(t *testing.T) {
	c := mustNew(t, 2)
	mustPut(t, c, "a", "1")
	mustPut(t, c, "b", "2")

	if err := c.Resize(4); err != nil {
		t.Fatalf("Resize(4) returned unexpected error: %v", err)
	}
	if got := c.Capacity(); got != 4 {
		t.Fatalf("Capacity() = %d; want 4", got)
	}
	// Growing keeps every existing entry.
	if keys := snapshotKeys(c); !equalStrings(keys, []string{"b", "a"}) {
		t.Fatalf("order = %v; want [b a]", keys)
	}
	mustPut(t, c, "c", "3")
	mustPut(t, c, "d", "4")
	if got := c.Len(); got != 4 {
		t.Fatalf("Len() = %d; want 4", got)
	}
	checkInvariants(t, c)
}

func TestInvalidArguments(t *testing.T) {
	for _, capacity := range []int{0, -1, -100} {
		if _, err := New(capacity); !errors.Is(err, ErrInvalidCapacity) {
			t.Fatalf("New(%d) error = %v; want ErrInvalidCapacity", capacity, err)
		}
	}

	c := mustNew(t, 1)
	for _, capacity := range []int{0, -5} {
		if err := c.Resize(capacity); !errors.Is(err, ErrInvalidCapacity) {
			t.Fatalf("Resize(%d) error = %v; want ErrInvalidCapacity", capacity, err)
		}
	}
	if got := c.Capacity(); got != 1 {
		t.Fatalf("Capacity() = %d; failed Resize must not change it", got)
	}

	if err := c.Put("", "v"); !errors.Is(err, ErrEmptyKey) {
		t.Fatalf("Put with empty key error = %v; want ErrEmptyKey", err)
	}
	if got := c.Len(); got != 0 {
		t.Fatalf("Len() = %d; rejected Put must not store anything", got)
	}
	if _, ok := c.Get(""); ok {
		t.Fatal("Get with empty key reported a hit")
	}
	if _, ok := c.Peek(""); ok {
		t.Fatal("Peek with empty key reported a hit")
	}
	if c.Delete("") {
		t.Fatal("Delete with empty key returned true")
	}
	checkInvariants(t, c)
}

func TestSnapshotOrderAndIsolation(t *testing.T) {
	c := mustNew(t, 3)
	mustPut(t, c, "a", "1")
	mustPut(t, c, "b", "2")
	mustPut(t, c, "c", "3")
	if _, ok := c.Get("a"); !ok {
		t.Fatal("Get(a) missed")
	}

	snap := c.Snapshot()
	want := []Entry{{Key: "a", Value: "1"}, {Key: "c", Value: "3"}, {Key: "b", Value: "2"}}
	if len(snap) != len(want) {
		t.Fatalf("Snapshot length = %d; want %d", len(snap), len(want))
	}
	for i, e := range want {
		if snap[i] != e {
			t.Fatalf("Snapshot()[%d] = %+v; want %+v", i, snap[i], e)
		}
	}

	// Mutating the returned slice must not affect the cache.
	snap[0].Value = "corrupted"
	snap[1].Key = "corrupted"
	if v, _ := c.Peek("a"); v != "1" {
		t.Fatalf("Peek(a) = %q after mutating snapshot; want %q", v, "1")
	}
	if keys := snapshotKeys(c); !equalStrings(keys, []string{"a", "c", "b"}) {
		t.Fatalf("order after mutating snapshot = %v; want [a c b]", keys)
	}
	checkInvariants(t, c)
}

func TestConcurrentAccess(t *testing.T) {
	c := mustNew(t, 64)

	const goroutines = 8
	const iterations = 200

	var wg sync.WaitGroup
	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				key := fmt.Sprintf("key-%d", (id*31+i)%100)
				switch i % 6 {
				case 0:
					if err := c.Put(key, "value"); err != nil {
						t.Errorf("Put(%q) error: %v", key, err)
					}
				case 1:
					c.Get(key)
				case 2:
					c.Peek(key)
				case 3:
					c.Delete(key)
				case 4:
					if err := c.Resize(32 + (i % 64)); err != nil {
						t.Errorf("Resize error: %v", err)
					}
				case 5:
					c.Snapshot()
					c.Len()
					c.Capacity()
				}
			}
		}(g)
	}
	wg.Wait()

	checkInvariants(t, c)
}
