# cache

`pkg/cache` 是项目内通用多级缓存封装。它提供 L1 本地内存缓存和 L2 Redis 远端缓存组合能力，业务代码通过 `cache.Client[T]` 使用泛型值缓存，不直接依赖 Redis client、序列化库或本地缓存实现类型。

## 技术选型

- L1 使用 `github.com/patrickmn/go-cache`，提供进程内 TTL 缓存，适合降低热点 key 的 Redis 访问频率。
- L2 通过 `github.com/redis/go-redis/v9` 接入 Redis，但 go-redis 类型只出现在 `pkg/cache/redisstore` 适配子包。
- 缓存值使用 `github.com/vmihailenco/msgpack/v5` 序列化为字节后写入 Redis，业务代码不需要手写 JSON 或 msgpack 编解码。
- 本包不使用 gocache chain。当前实现由项目层同步控制 L1/L2 写入、回填、删除和 tag 失效；L1 清理 goroutine 由 Client 创建并在 `Close` 时等待退出。

## 设计目标

- 显式依赖：独立使用时由调用方创建 Redis client 和 cache client；Kernel 模式通过稳定 Cache Access 构造 typed Client，再通过构造函数注入业务组件。
- 泛型契约：业务通过 `Client[T]` 读写具体类型，不接触 `[]byte` 或 Redis 命令。
- TTL 必填：`Config.DefaultTTL` 或 `WithTTL` 必须提供大于 0 的有效期，避免隐式永不过期。
- 可替换：根包只依赖 `RemoteStore` 字节级契约；未来替换 Redis 适配时不影响业务 API。
- 可失效：支持按 key 删除和按 tags 批量失效，不提供危险的全量清空入口。

## 目录结构

```text
pkg/cache/
├── cache.go             # Client 实现和 L1/L2 协调逻辑
├── config.go            # Config 配置类型
├── defaults.go          # 默认 tags TTL 和清理间隔
├── errors.go            # 项目缓存错误哨兵
├── options.go           # SetOption、WithTTL、WithTags
├── types.go             # Client 和 RemoteStore 契约
├── redisstore/          # Redis 远端存储适配
└── README.md            # 使用文档
```

## 配置项说明

| 字段 | 说明 | 默认值 |
| --- | --- | --- |
| `DefaultTTL` | 默认缓存项有效期。为 0 时，每次 `Set` 必须传入 `WithTTL` | `0` |
| `DefaultTagsTTL` | tag 索引有效期。实际写入 Redis 时使用 `max(itemTTL, tagsTTL)` | `720h` |
| `KeyPrefix` | key 和 tag 的命名空间前缀，用于隔离不同应用或环境 | 空 |
| `CleanupInterval` | L1 本地缓存过期清理间隔 | `1m` |

`DefaultTTL` 有意不提供开箱即用值。缓存有效期通常属于业务语义，脚手架不替业务选择“5 分钟”或“永不过期”。

## 基础使用示例

```go
package main

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/rin721/go-scaffold2/pkg/cache"
	"github.com/rin721/go-scaffold2/pkg/cache/redisstore"
)

type Profile struct {
	ID   int
	Name string
}

func main() {
	ctx := context.Background()

	redisClient := redis.NewUniversalClient(&redis.UniversalOptions{
		Addrs: []string{"127.0.0.1:6379"},
	})
	defer redisClient.Close()

	remote, err := redisstore.New(redisClient, nil)
	if err != nil {
		panic(err)
	}

	cfg := cache.DefaultConfig()
	cfg.DefaultTTL = 10 * time.Minute
	cfg.KeyPrefix = "user-api:"

	profiles, err := cache.New[Profile](remote, &cfg)
	if err != nil {
		panic(err)
	}
	defer profiles.Close()

	err = profiles.Set(ctx, "profile:1", Profile{ID: 1, Name: "Rin"}, cache.WithTags("profile"))
	if err != nil {
		panic(err)
	}

	profile, err := profiles.Get(ctx, "profile:1")
	if err != nil {
		panic(err)
	}
	_ = profile
}
```

## 单次写入 TTL 示例

```go
err := profiles.Set(
	ctx,
	"profile:2",
	Profile{ID: 2, Name: "Lin"},
	cache.WithTTL(30*time.Second),
	cache.WithTags("profile", "active-user"),
)
```

当 `Config.DefaultTTL` 为 0 且 `Set` 未传入 `WithTTL` 时，方法会返回 `cache.ErrInvalidTTL`。

## Tags 失效

```go
if err := profiles.InvalidateTags(ctx, "profile"); err != nil {
	panic(err)
}
```

`InvalidateTags` 会删除 L1 中已知的 tag 关联 key，并通过 Redis tag 索引删除远端 key。L2 命中回填到 L1 的 key 即使没有本地 tag 索引，也会在 Redis 返回失效 key 后同步从 L1 删除。

本包不提供 `Clear` 或 `FlushAll`。全量清空 Redis 影响范围过大，必须由应用或运维层按明确环境和权限单独处理。

## Redis 连接所有权

`redisstore.New` 接收外部传入的 `redis.UniversalClient`。缓存包不会创建、关闭或重配 Redis 连接；独立使用时由应用入口拥有连接池，Kernel 组合模式则由 Cache App 创建并关闭连接池。

这样做会让 `redisstore` 构造入口接触 go-redis 类型，但第三方类型不会进入业务常用的 `cache.Client[T]` 接口。

## 错误语义

- `ErrNotFound`：两层缓存都未命中。
- `ErrNilContext`：传入了 nil context。
- `ErrEmptyKey`：key 为空或只包含空白字符。
- `ErrInvalidTTL`：写入时没有有效 TTL，或 TTL 为负。
- `ErrNilRemoteStore`：构造 cache client 或 Redis store 时缺少远端存储。
- `ErrInvalidCachedValue`：缓存值序列化或反序列化失败。
- `ErrClientClosed`：typed Client 已关闭，不能继续使用。
- `ErrDisabled`：Kernel 的共享 Cache 后端被明确禁用。

调用方应使用 `errors.Is` 判断上述错误，不依赖错误字符串。

## 在业务代码中的推荐使用方式

推荐在 composition 完成后通过 `cacheapp.NewClient[T](capabilities.Cache, cfg)` 创建 `cache.Client[T]`，再通过构造函数注入业务组件。typed Client 拥有自己的 L1 状态和清理 goroutine，创建方必须调用 `Close`；它不拥有 Kernel 的 Redis 连接。业务组件不要自行创建 Redis client，也不要绕过 `pkg/cache` 直接散写 Redis key、tag 索引或序列化格式。

Cache App 默认 `disabled`，启用 Redis 后会在启动 Ready 阶段 Ping。Redis 配置变化采用 `RestartRequired`，不会在运行期替换后端；这避免已有 typed Client 的 L1 状态与远端命名空间在单个进程中跨代混用。
