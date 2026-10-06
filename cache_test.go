package lrucache_test

import (
	"fmt"
	"reflect"
	"sync"
	"testing"

	"lrucache"
)

func mustNew(t *testing.T, capacity int) *lrucache.Cache {
	t.Helper()
	c, err := lrucache.New(capacity)
	if err != nil {
		t.Fatalf("New(%d): %v", capacity, err)
	}
	return c
}

func mustPut(t *testing.T, c *lrucache.Cache, key, value string) {
	t.Helper()
	if err := c.Put(key, value); err != nil {
		t.Fatalf("Put(%q, %q): %v", key, value, err)
	}
}

func snapshotKeys(c *lrucache.Cache) []string {
	items := c.Snapshot()
	keys := make([]string, len(items))
	for i, item := range items {
		keys[i] = item.Key
	}
	return keys
}

func TestPutAndGet(t *testing.T) {
	c := mustNew(t, 2)
	mustPut(t, c, "a", "1")

	if v, ok := c.Get("a"); !ok || v != "1" {
		t.Fatalf("Get(a) = %q, %v; want %q, true", v, ok, "1")
	}
	if _, ok := c.Get("missing"); ok {
		t.Fatal("Get(missing) hit; want miss")
	}
	if got := c.Len(); got != 1 {
		t.Fatalf("Len() = %d; want 1", got)
	}
	if got := c.Capacity(); got != 2 {
		t.Fatalf("Capacity() = %d; want 2", got)
	}
}

func TestPutUpdatesExistingKey(t *testing.T) {
	c := mustNew(t, 2)
	mustPut(t, c, "a", "1")
	mustPut(t, c, "b", "2")
	mustPut(t, c, "a", "3")

	if v, ok := c.Get("a"); !ok || v != "3" {
		t.Fatalf("Get(a) = %q, %v; want %q, true", v, ok, "3")
	}
	if got := c.Len(); got != 2 {
		t.Fatalf("Len() = %d; want 2 (update must not add an entry)", got)
	}
	if got, want := snapshotKeys(c), []string{"a", "b"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("order = %v; want %v (updated key moves to front)", got, want)
	}
}

func TestEvictsLeastRecentlyUsed(t *testing.T) {
	c := mustNew(t, 2)
	mustPut(t, c, "a", "1")
	mustPut(t, c, "b", "2")
	mustPut(t, c, "c", "3")

	if _, ok := c.Get("a"); ok {
		t.Fatal("Get(a) hit; want a evicted")
	}
	if got := c.Len(); got != 2 {
		t.Fatalf("Len() = %d; want 2", got)
	}
	if got, want := snapshotKeys(c), []string{"c", "b"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("order = %v; want %v", got, want)
	}
}

func TestGetMovesToFront(t *testing.T) {
	c := mustNew(t, 2)
	mustPut(t, c, "a", "1")
	mustPut(t, c, "b", "2")
	if _, ok := c.Get("a"); !ok {
		t.Fatal("Get(a) missed")
	}
	mustPut(t, c, "c", "3")

	if _, ok := c.Get("b"); ok {
		t.Fatal("Get(b) hit; want b evicted after a was refreshed")
	}
	if got, want := snapshotKeys(c), []string{"c", "a"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("order = %v; want %v", got, want)
	}
}

func TestPeekDoesNotChangeOrder(t *testing.T) {
	c := mustNew(t, 2)
	mustPut(t, c, "a", "1")
	mustPut(t, c, "b", "2")

	if v, ok := c.Peek("a"); !ok || v != "1" {
		t.Fatalf("Peek(a) = %q, %v; want %q, true", v, ok, "1")
	}
	if _, ok := c.Peek("missing"); ok {
		t.Fatal("Peek(missing) hit; want miss")
	}
	mustPut(t, c, "c", "3")

	if _, ok := c.Get("a"); ok {
		t.Fatal("Get(a) hit; Peek must not protect a from eviction")
	}
	if got, want := snapshotKeys(c), []string{"c", "b"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("order = %v; want %v", got, want)
	}
}

func TestDelete(t *testing.T) {
	c := mustNew(t, 2)
	mustPut(t, c, "a", "1")
	mustPut(t, c, "b", "2")

	if !c.Delete("a") {
		t.Fatal("Delete(a) = false; want true")
	}
	if _, ok := c.Get("a"); ok {
		t.Fatal("Get(a) hit after Delete")
	}
	if c.Delete("a") {
		t.Fatal("Delete(a) = true for already deleted key; want false")
	}
	if got := c.Len(); got != 1 {
		t.Fatalf("Len() = %d; want 1", got)
	}
}

func TestResize(t *testing.T) {
	c := mustNew(t, 3)
	mustPut(t, c, "a", "1")
	mustPut(t, c, "b", "2")
	mustPut(t, c, "c", "3")

	if err := c.Resize(1); err != nil {
		t.Fatalf("Resize(1): %v", err)
	}
	if got := c.Len(); got != 1 {
		t.Fatalf("Len() = %d; want 1 after shrinking", got)
	}
	if got, want := snapshotKeys(c), []string{"c"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("order = %v; want %v (oldest evicted first)", got, want)
	}

	if err := c.Resize(3); err != nil {
		t.Fatalf("Resize(3): %v", err)
	}
	if got := c.Capacity(); got != 3 {
		t.Fatalf("Capacity() = %d; want 3", got)
	}
	mustPut(t, c, "d", "4")
	mustPut(t, c, "e", "5")
	if got, want := snapshotKeys(c), []string{"e", "d", "c"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("order = %v; want %v (growing keeps existing entries)", got, want)
	}
}

func TestInvalidParams(t *testing.T) {
	if _, err := lrucache.New(0); err == nil {
		t.Fatal("New(0) error = nil; want error")
	}
	if _, err := lrucache.New(-3); err == nil {
		t.Fatal("New(-3) error = nil; want error")
	}

	c := mustNew(t, 1)
	if err := c.Put("", "v"); err == nil {
		t.Fatal("Put with empty key: error = nil; want error")
	}
	if err := c.Resize(0); err == nil {
		t.Fatal("Resize(0) error = nil; want error")
	}
	if err := c.Resize(-1); err == nil {
		t.Fatal("Resize(-1) error = nil; want error")
	}
	if got := c.Capacity(); got != 1 {
		t.Fatalf("Capacity() = %d; want 1 (failed Resize must not change state)", got)
	}
}

func TestSnapshotOrderAndIsolation(t *testing.T) {
	c := mustNew(t, 3)
	mustPut(t, c, "a", "1")
	mustPut(t, c, "b", "2")
	mustPut(t, c, "c", "3")

	items := c.Snapshot()
	want := []lrucache.Item{
		{Key: "c", Value: "3"},
		{Key: "b", Value: "2"},
		{Key: "a", Value: "1"},
	}
	if !reflect.DeepEqual(items, want) {
		t.Fatalf("Snapshot() = %v; want %v (MRU to LRU)", items, want)
	}

	items[0].Key = "mutated"
	items[1].Value = "mutated"
	fresh := c.Snapshot()
	if !reflect.DeepEqual(fresh, want) {
		t.Fatalf("Snapshot() after mutating first copy = %v; want %v", fresh, want)
	}
}

func TestConcurrentAccess(t *testing.T) {
	c := mustNew(t, 64)
	const workers = 8
	const ops = 200
	const keySpace = 32

	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for i := 0; i < ops; i++ {
				key := fmt.Sprintf("k%d", (id*ops+i)%keySpace)
				switch i % 4 {
				case 0:
					_ = c.Put(key, "v")
				case 1:
					_, _ = c.Get(key)
				case 2:
					_, _ = c.Peek(key)
				case 3:
					_ = c.Delete(key)
				}
			}
		}(w)
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 20; i++ {
			if i%2 == 0 {
				_ = c.Resize(16)
			} else {
				_ = c.Resize(64)
			}
			_ = c.Snapshot()
			_ = c.Len()
			_ = c.Capacity()
		}
	}()
	wg.Wait()

	if got, cap := c.Len(), c.Capacity(); got > cap {
		t.Fatalf("Len() = %d exceeds Capacity() = %d", got, cap)
	}
	snap := c.Snapshot()
	if len(snap) != c.Len() {
		t.Fatalf("Snapshot length = %d; want Len() = %d", len(snap), c.Len())
	}
	seen := make(map[string]bool, len(snap))
	for _, item := range snap {
		if seen[item.Key] {
			t.Fatalf("duplicate key %q in Snapshot", item.Key)
		}
		seen[item.Key] = true
	}
}
