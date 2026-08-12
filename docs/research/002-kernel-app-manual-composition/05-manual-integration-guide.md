# 手动接入指南

> 本章使用 `<pkg-name1>` 和 `<pkg-app-name1>` 表达目标开发流程。示例 API 是目标伪代码，当前仓库不能直接编译；当前等价实现仍位于 `internal/kernel/capability` 和 `internal/kernel/composition`。

## 1. 先回答三个问题

不要从复制 Database Definition 开始。先写清：

1. **业务需要什么能力**：调用方真正需要哪些方法，哪些第三方 API 不应暴露？
2. **谁拥有资源**：谁创建、Start、Stop、Close，派生对象能否逃逸？
3. **配置怎样生效**：底层库能否原子重载，新旧实例能否并存，是否存在排他资源？

只有能力需要进程级治理时，才继续建立 `kernel/app` 组件。纯函数或无资源能力到 `pkg` 和普通构造函数注入即结束。

## 2. 第一步：封装 `pkg/<pkg-name1>`

推荐最小文件：

```text
pkg/<pkg-name1>/
├── <pkg-name1>.go       # 项目自有业务能力接口
├── config.go            # 与第三方类型无关的配置
├── errors.go            # 可识别的项目错误
├── builder.go           # New/Build 和资源所有权
├── <pkg-name1>_test.go  # 契约与实现测试
└── README.md            # 直接使用方式和边界
```

示意契约：

```go
// 伪代码：具体名称由真实能力语义决定。
type Client interface {
	Do(context.Context, Request) (Result, error)
}

type Resource interface {
	Client
	Close() error
}

type Config struct {
	Endpoint string
	Timeout  time.Duration
}

func ValidateConfig(*Config) error
func New(context.Context, *Config) (Resource, error)
```

约束：

- `Request`、`Result`、`Config` 和错误都是项目类型；
- `New` 不读取全局配置，不自动注册 Kernel；
- `Resource` 的 Close 只交给所有者，业务常用接口不暴露 Close；
- I/O 接收 Context，超时和取消保持可识别；
- 第三方错误经过项目语义包装但保留原始原因；
- 单元测试覆盖配置、构造失败、正常调用、取消、重复 Close 和资源清理；
- 如果计划未来替换第三方库，建立同一套契约测试，而不是依赖实现私有测试。

完成这一步后，`pkg` 已经可以脱离 Kernel 单独使用，也已经实现了最重要的“第三方库可替换”。

## 3. 第二步：判断是否需要 Kernel 组件

使用下表：

| 问题 | 否时处理 |
| --- | --- |
| 是否拥有共享进程级资源？ | 在普通 composition root 直接构造 |
| 是否需要统一 Start/Stop 或长期 Task？ | 由实际所有者管理生命周期 |
| 是否需要配置监听、健康或实例重载？ | 不建立 `kernel/app` |
| 业务是否必须避免取得 Close 权？ | 可直接注入窄 `pkg` 接口 |

Clock、ID Generator、Validator、Codec 等通常无需进入 Kernel。不要为了目录整齐机械包装。

## 4. 第三步：建立 `kernel/app/<pkg-app-name1>`

目标最小文件：

```text
internal/kernel/app/<pkg-app-name1>/
├── component.go       # 唯一 Definition 入口、ID、Dependencies
├── config.go          # typed 解码、校验和默认值契约
├── lifecycle.go       # Build/Start/Ready/Stop，仅保留实际需要的职责
├── access.go          # 必要时收敛稳定租约 Access
├── reload.go          # Reload Policy 及策略专用实现
└── component_test.go  # 组件级契约和失败语义
```

文件可以在能力较小时合并；重要的是职责和唯一入口，不是机械目录数量。

### 4.1 定义身份和依赖

```go
// 目标伪代码，尚未实现。
const ID kernel.ID = "<pkg-app-name1>"
const ConfigPath = "<pkg-app-name1>"

type Dependencies struct {
	// 只列构建该组件确实需要的固定依赖。
}

func Definition(deps Dependencies) kernelapp.Definition[Config, Instance, Access] {
	// 在一个入口组合小契约和 Reload Policy。
}
```

组件不持有 Kernel，不调用 Register，不 import 其他 composition 文件。依赖由手动装配者传入。

### 4.2 typed 配置和初始配置

- 从完整 Snapshot 只读取自己的 ConfigPath；
- Decode 预填由 `pkg` 提供的安全默认值；
- Validate 不打开网络、文件或 goroutine；
- Defaults 只生成自身配置段；
- 密码、Token、DSN 等默认保持安全空值，通过环境变量等受控来源提供；
- 有组件专用启动前命令时贡献 CLI Contract，不自行构造 CLI App。

### 4.3 构建与生命周期

```text
Build: Config -> 未发布 Instance
Start: 启动实例拥有的后台活动或连接
Ready: 确认候选可以接管业务
Stop: 停止活动并释放该代全部资源
Health: 运行期间返回结构化健康结果
```

没有 Start 需要就不声明 Starter；没有持续 Health 就不提供空检查。Stop 必须幂等或由 Kernel 严格保证只调用一次，并保留主要错误与所有清理错误。

### 4.4 选择 Reload Policy

在组件包内记录证据：

- `NativeAtomicReload`：链接上游原子性说明并提供失败保留旧状态的测试；
- `KernelInstanceSwap`：证明新旧可并存、业务使用可租约化、双实例资源预算可接受；
- `ComponentHandoff`：提供专用交接设计和失败提交点；
- `RestartRequired`：说明排他约束或不可安全回滚原因；
- `Ignore`：说明配置不参与运行期变化。

不要让 composition 根据环境临时改变同一组件的安全语义。若不同实现的重载能力不同，应由各实现对应的组件定义显式声明。

### 4.5 收敛业务 Access

若使用 `KernelInstanceSwap`，业务通过 Access 的有界回调使用当前实例：

```go
// 目标示意。
type Access interface {
	Use(context.Context, func(pkgname.Client) error) error
}
```

回调返回代表本次租约结束。Client 派生的 stream、iterator、Rows、transaction 或 session 必须在回调内完整消费和关闭，否则 Kernel 无法知道实例是否仍被使用。

若组件不会运行期换代，不必为了形式统一强制包一层 `Use`；可直接返回稳定的项目能力接口。

## 5. 第四步：在 composition 手动登记

新增唯一能力装配文件：

```text
internal/kernel/composition/<pkg-app-name1>.go
```

目标伪代码：

```go
func composePkgName(
	runtime *kernel.Kernel,
	deps pkgnameapp.Dependencies,
) (pkgnameapp.Access, error) {
	access, err := kernelapp.Register(runtime, pkgnameapp.Definition(deps))
	if err != nil {
		return nil, fmt.Errorf("compose <pkg-app-name1>: %w", err)
	}
	return access, nil
}
```

然后在总 `Compose` 中：

1. 按明确顺序调用 `composePkgName`；
2. 将 Access 放进进程实际需要的组合结果；
3. 让 Kernel 从登记的 Definition 自行聚合 Defaults 和 CLI Contract；
4. 任一登记失败返回零值组合结果，不暴露部分可用对象；
5. 用测试固定启用清单、顺序、重复 ID 和失败行为。

目标设计应消除当前 composition 手工拆出 `registration.Access`、`registration.Defaults` 并为每种契约维护并行切片的样板，但仍保留显式登记这一事实。

## 6. 第五步：注入业务静态对象图

应用组合根从组合结果中取出最小依赖：

```go
// 未来业务示意，不是当前实现。
repo := orderrepo.New(capabilities.Database)
service := orderservice.New(repo, capabilities.Clock)
handler := orderhandler.New(service, capabilities.Logger)
router := orderhttp.NewRouter(handler, middlewares...)
server := httpapp.NewServer(serverConfig, router)
```

禁止：

- 把整个 `Capabilities` 传给每个 service；
- 让业务接收 Kernel、Handle、Container 或 Resolver；
- 在 repository 内绕过 Access 再创建第二套第三方 Client；
- 因为还没有真实业务层就提前发明通用 BaseRepository/BaseService。

## 7. 第六步：交给 Host 运行

受托管底层组件由 Kernel Participant 启动。HTTP Server、Worker 等上层长期服务作为独立 Participant 或 Task 交给 Host：

```text
启动：Kernel components -> business/server Participants -> long-running Tasks
停止：停止 Tasks -> business/server Participants -> Kernel components
```

这样在停止业务入口后，底层 Database、Logger 等资源仍可供在途清理使用。配置 Watch 是 Host 的可选 Task；单次 Reload 失败由回调记录并继续监听，Watcher 自身失败才终止 Task。

## 8. 最小修改集合

一个新受托管能力通常只需要：

1. `pkg/<name>`：能力封装和契约测试；
2. `internal/kernel/app/<name>`：一个组件定义包；
3. `internal/kernel/composition/<name>.go`：一处手动选择和登记；
4. `composition.go`：把该能力加入当前进程清单和结果；
5. 权威文档和必要配置示例；
6. 组件、composition、Kernel 策略语义测试。

若只是无资源通用能力，通常只有第 1 项和应用构造函数调用，不需要第 2 至第 4 项。

## 9. 验收清单

### `pkg` 边界

- [ ] 业务 API 不暴露未经允许的第三方类型。
- [ ] Config、错误、Context、超时和 Close 语义明确。
- [ ] 可脱离 Kernel 独立构造和测试。
- [ ] 第三方实现可由同一契约测试验证。

### 组件边界

- [ ] ID 和 ConfigPath 稳定且无重复。
- [ ] 只有一个 Definition 入口，不自行 Register。
- [ ] 只声明实际存在的小契约。
- [ ] 资源所有者、Ready、Health、Stop 和敏感信息边界明确。
- [ ] Reload Policy 有证据，不靠方法名猜测。

### composition 与业务

- [ ] 启用清单和顺序显式可搜索。
- [ ] 业务只接收最小 typed 依赖。
- [ ] 没有 Kernel/Container/Resolver 穿透。
- [ ] 没有调用方自行创建第二套共享 Client。

### 重载与停止

- [ ] 配置 Decode 失败不影响当前实例。
- [ ] 候选 Build/Start/Ready 失败会完整清理。
- [ ] 租约排空、Context 取消和超时有测试。
- [ ] 观察失败能回切并清理失败新代。
- [ ] 观察成功后才清理上一代，清理失败不伪装成回滚。
- [ ] 连续变化只保留最新配置且不会累积多代资源。
- [ ] 排他资源使用 Handoff 或明确 RestartRequired。
- [ ] Host 先停业务入口，再释放底层资源。
