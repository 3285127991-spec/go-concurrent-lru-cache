# go-concurrent-lru-cache

一个零依赖、并发安全的进程内 LRU 缓存，使用 Go 标准库实现（哈希表 + 双向链表），
Key 和 Value 均为字符串，Key 不能为空。

## 特性

- 创建时指定正整数容量，容量满时 `Put` 原子淘汰最久未使用项
- `Get` 命中会将该项移动到最近使用位置；`Peek` 只查询、不改变顺序
- `Resize` 缩容立即按 LRU 顺序淘汰，扩容保留全部现有内容
- `Snapshot` 按最近使用到最久未使用顺序返回独立副本
- 所有操作由互斥锁保护，map、链表与淘汰结果始终一致，可安全并发调用
- 核心 `Get`/`Put`/`Delete` 均为 O(1)，不做全量重排

## API

| 方法 | 说明 |
| --- | --- |
| `New(capacity int) (*Cache, error)` | 创建缓存，容量必须为正整数 |
| `Put(key, value string) error` | 插入或更新并标记为最近使用，满时原子淘汰 LRU 项 |
| `Get(key string) (string, bool)` | 查询并移动到最近使用位置 |
| `Peek(key string) (string, bool)` | 只查询，不改变 LRU 顺序 |
| `Delete(key string) bool` | 删除指定 Key，返回是否存在 |
| `Resize(capacity int) error` | 调整容量（正整数），缩容立即淘汰 |
| `Snapshot() []Entry` | 返回 MRU → LRU 顺序的独立副本 |
| `Len() int` | 当前条目数 |
| `Capacity() int` | 当前容量 |

非法参数返回哨兵错误 `ErrInvalidCapacity` 与 `ErrEmptyKey`，可用 `errors.Is` 判断。

## 使用示例

```go
cache, err := lrucache.New(2)
if err != nil {
    log.Fatal(err)
}
_ = cache.Put("a", "1")
_ = cache.Put("b", "2")

if v, ok := cache.Get("a"); ok { // a 变为最近使用
    fmt.Println(v)
}
_ = cache.Put("c", "3") // 容量满，淘汰最久未使用的 b
```

## 运行演示

```sh
go run ./cmd/demo
```

演示依次展示 Put、Get 改变顺序、容量淘汰、Peek、Resize 与 Snapshot。

## 运行测试

```sh
go test -race ./...
```

## 项目结构

```
lru.go            缓存实现（哈希表 + 双向链表 + 互斥锁）
lru_test.go       自动化测试（含并发场景，配合 -race 验证）
cmd/demo/main.go  最小 CLI 演示
```
