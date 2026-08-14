# 开发设计：Cache、I18n 与 Storage 装配

## 1. 总体结构

011 复用当前单一装配链：

```text
pkg capability / adapter
  -> internal/kernel/app/<name> Definition
  -> internal/kernel/composition 显式 Add
  -> FrozenPlan + DefaultManager
  -> Kernel Install / Start / Reload / Stop
  -> composition.Capabilities 稳定入口
```

不增加 Registry、Resolver、反射扫描、`init` 注册或运行期按类型查找。三个组件都没有业务层依赖，按固定顺序追加到 Database 之后。

| 组件 | ID / ConfigPath | 私有实例 | 稳定输出 | 重载策略 |
| --- | --- | --- | --- | --- |
| Cache | `cache` / `cache` | disabled resource 或 Redis client + `RemoteStore` | `cacheapp.Access` + `NewClient[T]` | `RestartRequired` |
| I18n | `i18n` / `i18n` | `pkg/i18n.Translator` | 实现同一 Translator 契约的 facade | `KernelInstanceSwap` |
| Storage | `storage` / `storage` | `*pkg/storage.StorageManager` | `storageapp.Access` | `KernelInstanceSwap` |

## 2. Cache 设计

### 2.1 后端与 typed Client 分离

Kernel 不可能在没有业务类型的情况下预建 `cache.Client[T]`。因此 Cache App 输出一个封闭的稳定 Access，并由同包泛型函数建立业务 Client：

```go
type Access interface {
	Ping(context.Context) error
	use(context.Context, func(pkgcache.RemoteStore) error) error
}

func NewClient[T any](access Access, cfg *pkgcache.Config) (pkgcache.Client[T], error)
```

`use` 是包内方法，外部不能实现或取得原始 Store。`NewClient[T]` 注入一个私有 remote facade；每次 L2 操作都在当前 Kernel Lease 内完成。业务仍只使用 `pkg/cache.Client[T]`，不接触字节协议或 go-redis。

### 2.2 资源所有权

Redis 模式的 Kernel 私有实例保存 go-redis client 和 `pkg/cache.RemoteStore`：

```text
build -> validate typed config -> create Redis client -> redisstore.New
ready -> Ping with configured timeout
stop  -> Redis client Close
```

构造失败时关闭已经创建的资源；Stop 错误保留上下文。Access、typed Client 和业务代码均没有 Redis Close 权。

`pkg/cache.Client[T]` 增加幂等 Close。底层 go-cache 以 `cleanupInterval=0` 创建，由项目 Client 自己按配置启动可取消清理循环，Close 负责 cancel + wait；不依赖第三方私有 janitor 或 GC finalizer。

### 2.3 配置

```yaml
cache:
  driver: disabled # disabled 或 redis
  redis:
    address: 127.0.0.1:6379
    username: ""
    password: ""
    database: 0
    dialTimeout: 5s
    readTimeout: 3s
    writeTimeout: 3s
    poolSize: 0
    minIdleConns: 0
    tagPrefix: "cache:tag:"
    pingTimeout: 5s
```

`poolSize=0` 明确表示采用 go-redis 已记录的默认计算；负数非法。Password 在生成配置中为空，示例要求使用 `APP_CACHE__REDIS__PASSWORD`。校验错误不得回显密码。

`DefaultTTL`、`DefaultTagsTTL`、`KeyPrefix` 和 `CleanupInterval` 仍属于每个业务 typed Client，不放进全局 `cache` 段。

### 2.4 RestartRequired 原因

连接、认证或 tag prefix 变化时，既有 typed Client 内仍持有 L1 值、tag 索引和业务 KeyPrefix。当前 Kernel 只管理共享后端，无法对所有泛型 Client 做原子枚举和迁移。因此 Cache section 摘要变化只进入 `RestartRequired` 预检，不创建 Redis 候选，也不让同轮其他组件部分提交。

## 3. I18n 设计

### 3.1 Config 与 facade

```go
type Config struct {
	DefaultLanguage string                   `mapstructure:"defaultLanguage"`
	MessageFiles    []string                 `mapstructure:"messageFiles"`
	MissingBehavior pkgi18n.MissingBehavior `mapstructure:"missingBehavior"`
}
```

App 把该配置映射为 `pkg/i18n.Config`，`MessageFS` 固定为 `os.DirFS(".")`。Decode 只做语言、策略、路径和扩展名校验；Build 才加载文件。

稳定 facade 实现 `pkg/i18n.Translator`。每次 Translate 在当前 Lease 内调用 Translator；返回值和 error 在释放租约前完成，不泄漏底层对象。`MustTranslate` 保持现有契约，在 Translate 失败时 panic，不偷偷回退消息 ID。

### 3.2 默认与换代

```yaml
i18n:
  defaultLanguage: zh-CN
  messageFiles: []
  missingBehavior: error
```

空资源清单允许组件构造，但不存在的消息仍按 `error` 返回。配置变化时先完整加载候选 Bundle，再排空当前翻译调用并切换 facade；候选失败时旧 Translator 继续服务。

Kernel 只监控主配置文件。消息文件原地内容变化不改变 i18n section digest，需重启或同时修改主配置触发候选；本任务不建立第二套 fsnotify。

## 4. Storage 设计

### 4.1 对象存储边界

Kernel 私有实例保持整个 `StorageManager`，从而在 `local+s3` / `local+minio` 下完整关闭两个 client。稳定 Access 不直接返回 Manager：

```go
type Route string

const (
	RoutePrimary Route = "primary"
	RouteLocal   Route = "local"
	RouteObject  Route = "object"
)

type Client interface {
	Put(context.Context, string, []byte, pkgstorage.PutOptions) error
	Get(context.Context, string) ([]byte, pkgstorage.ObjectInfo, error)
	Delete(context.Context, string) error
	Exists(context.Context, string) (bool, error)
}

type Access interface {
	Use(context.Context, Route, func(Client) error) error
}
```

`Use` 在 Lease 内选择 Manager 的 Primary、Local 或 Object，并交付私有 borrowed wrapper。wrapper 不含 Close，以 `Use` 的根 context 和每次操作 context 共同约束 I/O；回调结束后所有方法返回 `ErrClientUnavailable`。即使逃逸或类型断言，也不能恢复底层 concrete client，调用方也不能用新的 `context.Background()` 绕过原租约取消。

文件工具 `storage.Storage` 不进入该接口。它包含 watcher 注册、`RemoveAll`、Workbook 和 Image 等调用方局部语义，不适合作为所有业务共享的进程级万能存储对象。

### 4.2 配置与默认

Storage App 使用自己的 typed Config，只映射 Manager 实际需要的字段：

```yaml
storage:
  driver: local
  local:
    basePath: .data/storage
    publicUrl: ""
  s3:
    provider: s3
    endpoint: ""
    region: us-east-1
    bucket: ""
    accessKeyId: ""
    secretAccessKey: ""
    usePathStyle: false
    publicBaseUrl: ""
  minio:
    provider: minio
    endpoint: ""
    region: us-east-1
    bucket: ""
    accessKeyId: ""
    secretAccessKey: ""
    usePathStyle: true
    publicBaseUrl: ""
```

文件工具专用的 `fsType`、`enableWatch`、`watchBufferSize` 不进入 Kernel section。远端 driver 只有在被选择时才要求对应 endpoint/bucket/credential；未选择分支的空凭据不是错误。Secret 只在示例中说明环境变量覆盖，不写入真实值。

### 4.3 Ready、换代与关闭

- Local Ready 在 `.data/storage` 或显式根目录内用唯一临时 key 完成 Put/Get/Exists/Delete，并尽力保留主错误与清理错误。
- Object Ready 调用 S3-compatible `HeadBucket`，不创建业务对象。
- 组合模式检查实际创建的 Local 和 Object，避免只检查 Primary。
- 候选全部 Ready 后才提交；旧 Access 使用排空后，`StorageManager.Close` 用 `errors.Join` 关闭所有实例。
- Disabled 生成非 nil 私有实例以满足组件不变量，但任何 `Use` 返回 typed disabled error，不返回 nil Client 或成功空操作。

## 5. Composition 与配置数据流

```text
Compose
  -> Add Logger target/replacement
  -> Add Clock -> ID Generator -> Validator -> Database
  -> Add Cache -> I18n -> Storage
  -> Freeze
  -> NewDefaultManager(frozen.Defaults)
  -> optional CLI
  -> Kernel.Install(frozen)
```

`Capabilities` 增加：

```go
Cache   cacheapp.Access
I18n    pkgi18n.Translator
Storage storageapp.Access
```

默认配置段顺序为 Logger、Database、Cache、I18n、Storage。CLI 模式执行 `config init` 时 Managed 实例尚未 Build，因此不会连接 Redis、读取消息文件、创建 Storage 目录或访问远端 bucket。

## 6. 失败与安全语义

- Decode/Validate 不打开连接、不创建目录、不启动 goroutine。
- nil/canceled context、disabled、无效 route、无可用 client、租约停止分别使用可识别的项目 error；取消与超时保持 `errors.Is`。
- Redis、i18n 文件、local filesystem 和 S3 错误增加能力与阶段上下文；任何包含密码、secret、完整凭据或私有 URL 的值不得进入错误、日志或测试输出。
- 敏感第三方错误在能力边界映射为安全项目错误；可安全保留的取消、超时和 typed 分类继续保留。
- Cache typed Client Close 幂等停止本地清理任务；构造回滚或 Storage 多 client Close 同时产生多个错误时用 `errors.Join` 保留所有已知失败，不只返回最后一个错误。
- I18n/Storage 候选失败只丢弃候选；Cache 变更在整轮预检阶段返回 `RestartRequired`，不发生任何候选副作用。

## 7. 文件影响

### 新增

- `internal/kernel/app/cache/**`
- `internal/kernel/app/i18n/**`
- `internal/kernel/app/storage/**`
- `internal/kernel/composition/cache.go`
- `internal/kernel/composition/i18n.go`
- `internal/kernel/composition/storage.go`

### 修改

- `pkg/cache/**`：显式 typed Client Close、受控清理循环、Redis 构造所需项目配置/错误与测试。
- `pkg/storage/**`：纯配置校验、唯一 local readiness key、borrowed client 所需稳定错误与现有文档。
- `internal/kernel/composition/composition.go` 及测试：固定顺序和 Capabilities。
- `cmd/app/**` 测试：CLI 无资源副作用、默认 Host 能力交付。
- `config.example.yaml`、根 `README.md`、`pkg/README.md`、`pkg/cache/README.md`、`pkg/i18n/README.md`、`pkg/storage/README.md`、`internal/kernel/README.md`、`internal/kernel/app/README.md`：同步当前使用方式和边界。
- 必要的架构/文档测试与 CI 配置；不新增生产依赖，除非实施时出现当前依赖无法满足且重新获得确认。

## 8. 验证策略

- Package：Cache typed Client Close/race、miniredis Redis resource、I18n 文件加载、Storage local 与 httptest S3-compatible 合约。
- App：Definition/Defaults/decode、稳定输出、disabled、Ready、Stop、候选失败、租约排空和逃逸失效。
- Composition：固定顺序、完整 Capabilities、默认配置顺序、失败原子性、CLI 零资源副作用。
- Reload：I18n/Storage 成功换代与失败保旧；Cache section 变化使包含其他变更的整轮在副作用前 `RestartRequired`。
- Process：临时目录执行 `config init` 与默认 service smoke；证明无需 Redis/S3/MinIO，Storage 只写受控 `.data/storage`，退出后资源关闭。
- Repository：完整 build/test/race/vet/tidy/no-CGO、Markdown 链接、第三方类型/Close 权/旧直连搜索、`git diff --check` 和独立暂存 Diff 审阅。
