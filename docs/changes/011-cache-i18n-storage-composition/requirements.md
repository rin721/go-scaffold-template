# 产品需求：Cache、I18n 与 Storage 装配

## 1. 背景与当前事实

### 1.1 Kernel 当前装配面

- `internal/kernel/composition.Compose` 当前按 Logger、Clock、ID Generator、Validator、Database 的顺序建立并冻结 Plan。
- `composition.Capabilities` 当前只交付上述能力、默认配置管理器和可选 CLI。
- Configured Managed 组件必须通过稳定 Lease facade 输出；Kernel 已实现候选构造、Ready、反向排空、提交、回滚、旧代关闭和 `RestartRequired` 预检。
- 当前默认配置和 `config.example.yaml` 只有 Logger、Database 两段。

### 1.2 三项底层库现状

- `pkg/cache` 提供泛型 `Client[T]`，其 L2 只依赖项目 `RemoteStore`；`pkg/cache/redisstore` 已隔离 go-redis 操作，但 Redis client 仍由调用方创建和关闭。`Client[T]` 当前使用带清理 goroutine 的 L1，却没有显式 Close/Wait 边界。
- `pkg/i18n` 提供不泄漏 go-i18n 类型的 `Translator`。构造时加载消息文件，构造后只执行内存翻译；当前仓库没有该包的外部调用方。
- `pkg/storage` 同时包含文件工具 `Storage` 与对象存储 `StorageManager`。Manager 拥有 local/S3-compatible client 的关闭权，但 `StorageClient` 本身也暴露 `Close`。当前仅 `pkg/foundation_test.go` 直接使用文件工具；没有业务对象依赖对象存储 Manager。
- 三项包的定向测试在方案轮通过，但这只证明底层库现状，不代表 Kernel 装配已经存在。

## 2. 目标

- 为 Cache、I18n、对象 Storage 建立唯一的 `internal/kernel/app/<name>` Definition，并由当前 composition 显式选择。
- 向上层交付身份稳定、项目自有、无第三方类型和无共享资源关闭权的 Capabilities。
- 配置解码、默认值、构造、Ready、重载和 Stop 责任完整进入 Kernel 生命周期。
- 保持本地默认启动不依赖 Redis、S3 或 MinIO，不把空凭据、假服务或静默回退伪装成可用远端能力。
- 维持严格有序 Plan、配置事务和反向释放，不引入自动扫描、Service Locator、全局单例或第二套配置读取路径。

## 3. 功能需求

### 3.1 Cache

- Kernel Cache App 只拥有共享后端资源，不预先猜测业务值类型、TTL 或 key namespace。
- 后端模式只允许显式 `disabled` 或 `redis`；默认是 `disabled`。选择 Redis 时必须完整校验地址、超时、连接池和认证字段，再创建 go-redis client、`redisstore` 与 Ready Ping。
- Redis client 的创建、Ping 和 Close 由 Kernel 私有实例拥有；普通调用方不得取得 go-redis 类型、原始 client 或 Close 权。
- `cacheapp.NewClient[T]` 根据稳定 Cache Access 和业务自己的 `pkg/cache.Config` 构造 typed Client。业务 TTL、KeyPrefix 和 L1 清理周期不进入全局 Kernel 配置。
- `pkg/cache.Client[T]` 必须获得显式 Close 语义；L1 清理 goroutine 由 Client 自己停止并等待，不能继续依赖 GC finalizer 回收。
- Cache 配置变化必须报告 `RestartRequired`。现阶段不承诺在 Redis 后端、认证、tag prefix 变化时迁移或清空既有 typed Client 的 L1 状态。
- Disabled 是明确配置状态；typed Client 使用后端时返回可识别的 disabled error，不静默退化为本地缓存或成功空操作。

### 3.2 I18n

- I18n App 从 `i18n` 配置段解码默认语言、消息文件列表和缺失消息策略，构造 `pkg/i18n.Translator`。
- 输出实现 `pkg/i18n.Translator`，且 facade 身份在 Kernel 换代前后不变；业务不接触 Bundle、Localizer 或 Lease。
- 当前 Kernel 配置只支持相对进程工作目录的 JSON/YAML 文件。`embed.FS` 仍可通过 `pkg/i18n.New` 直接使用，但不在本次 composition 配置面中伪造文件系统序列化。
- I18n 配置变化使用 `KernelInstanceSwap`：新资源全部加载成功后才发布；加载失败保留旧 Translator。
- 只修改消息文件内容而不改变配置快照不会触发 Kernel Reload；消息资源独立 Watch 不属于本任务。

### 3.3 Storage

- Kernel Storage App 只治理 `StorageManager` 的对象存储能力，支持当前 `disabled`、`local`、`s3`、`minio`、`local+s3`、`local+minio` 模式。
- 稳定 Access 使用专用 `Route` 选择 Primary、Local 或 Object，并在租约回调内交付不含 Close 的窄 Client。逃逸回调的 Client 必须失效，且不能通过类型断言恢复 Manager 或底层具体 client。
- 带文件监听、Excel、图片辅助和 `RemoveAll` 的文件工具不进入全局 Capabilities；调用方使用 `storage.New` 时继续自行拥有和关闭。
- Storage 配置使用单一 Kernel Snapshot；不调用 `Config.OverrideConfig`，避免 `APP_` 配置源之外再读取 `STORAGE_*` 环境变量。
- 默认对象存储为 local，根目录为 `.data/storage`。远端密钥默认留空，只允许通过受控配置源提供；示例文档不得填入真实凭据。
- 候选 Ready 检查覆盖当前实际创建的 local/object client；local 探针只能在受控根目录写入唯一临时对象并清理，S3-compatible 探针使用 bucket metadata 请求。
- Storage 配置变化使用 `KernelInstanceSwap`。所有新调用在提交后进入新 Manager，旧租约排空后反向关闭旧 client；构造或 Ready 失败保留旧 Manager。

### 3.4 Composition、配置与入口

- 固定装配顺序扩展为 Logger、Clock、ID Generator、Validator、Database、Cache、I18n、Storage；原有 Logger、Database 默认段顺序不改变。
- `composition.Capabilities` 新增 Cache、I18n、Storage 稳定入口；任一 Definition、Defaults、CLI、Freeze、配置管理器或 Install 失败时返回零值且不部分安装。
- `config init` 和 `config.example.yaml` 同步增加三段配置。生成默认配置不包含真实密码、Token 或私有地址。
- `cmd/app` 使用同一 Compose 路径，不单独创建 Redis、Translator 或 Storage Manager，不新增第二套 client。
- CLI 模式只构造 Plan 与默认配置管理器，不启动 Managed 实例；无参数 Host 模式才打开资源。

## 4. 验收标准

- 三个新 App Definition 的 ID、ConfigPath、Defaults、Exposure、ReloadPolicy 和生命周期契约均有单元测试。
- Cache 证明 disabled 不伪装成功、Redis Ping/Close 所有权正确、typed Client 可关闭、配置变化整轮 `RestartRequired` 且无候选副作用。
- I18n 证明 facade 身份稳定、成功换代、非法语言/缺失文件保旧、消息文件内容和缺失策略按配置生效。
- Storage 证明 route 选择、窄 Client、逃逸失效、local 真读写、远端 HTTP 合约、Ready、换代、排空和多关闭错误聚合。
- 完整 composition 与 Host 测试证明默认启动无需 Redis/S3/MinIO，三项 Capabilities 可注入，Stop 反向释放，跨组件失败不部分提交。
- `config init`、`config.example.yaml`、根 README、Kernel/App/pkg 权威文档与实现一致，且明确文件工具未进入全局装配。
- 实施后执行 `gofmt`、`go mod tidy -diff`、`go build ./cmd/app`、`go test ./...`、`go test -race ./...`、`go vet ./...`、无 CGO 构建、Markdown 相对链接、边界/旧符号搜索和 `git diff --check`。

## 5. 非目标

- 不装配 HTTP、业务 service/repository/model、消息、任务、鉴权或健康 HTTP endpoint。
- 不把 `tmp/` 下任何代码复制、迁移或视为当前架构依据。
- 不为 Cache 新增本地降级、双写迁移、cluster/sentinel 管理或运行期 L1 跨代迁移。
- 不监听 i18n 消息文件内容变化，不引入远端翻译平台。
- 不把 `storage.New` 文件工具变成进程全局共享对象，不设计业务目录、上传 API、权限或清理策略。
- 不在本任务中启动真实 Redis/S3/MinIO、部署应用、写入外部系统或 push 远端。

## 6. 需要确认的关键决策

1. Cache 默认显式禁用；启用 Redis 后配置变化需要重启。
2. `pkg/cache.Client[T]` 增加 Close，属于公开接口收紧；仓库当前没有该接口的外部实现或业务调用方需要迁移。
3. I18n 可热换配置，但消息文件内容本身不单独监听。
4. Storage 只装配对象存储 Manager；文件工具继续由具体调用方拥有。
