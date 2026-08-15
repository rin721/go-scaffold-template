# 设计：第三方封装与分轨装配

## 1. 设计结论

第三方依赖按业务归属和装配层级分两轨，不能用一个“全部放模块”或“全部下沉”规则替代判断：

```text
业务能力：internal/module/<name> 收口完整纵向切片
          Module port <- Module Adapter(third party) <- module.go

双条件底层资源：pkg contract <- Kernel App(third party)
                    <- kernel composition <- application composition/module
```

两条轨道共同遵守：第三方类型、错误、配置对象、Option、Client 和 Close 权不得越过实现边界。

## 2. 业务模块专属 Adapter

模块目录允许按真实需要包含：

```text
internal/module/<name>/
├── model/
├── service/              # 调用方定义的窄 port
├── repo/                 # 业务持久化 Adapter；真实需要时建立
├── adapter/<technology>/ # 第三方实现，只服务本模块
├── binding/              # config、HTTP/CLI Handler、migration 等入口完成品
└── module.go             # 依据模块配置选择私有 Adapter
```

Model、repo、service、Handler、Adapter、binding、配置/迁移/运行单元与 contribution 都属于“先收口”的职责集合，不要求每项都建立独立目录。没有真实职责时不制造空层；Handler 可以位于 `binding/http` 或 `binding/cli`，contribution 可以由 `module.go` 输出，但都不能外溢到顶层通用包。

约束：

- Model/Service 不直接导入第三方 module；
- Adapter 可以导入第三方，但 exported 声明只能出现标准库、同模块契约或允许的项目自有契约；
- 模块根可以导入自己的 Adapter 并构造它，但 `Dependencies`、`Module` 和 Contribution 不能暴露具体 Adapter；
- composition 只导入模块根，不穿透到 `adapter/*`；
- 第三方错误在 Adapter 内保留原因链并转换为模块可识别语义，敏感值脱敏；
- Client/goroutine 的 Start/Ready/Stop/Wait 通过项目生命周期契约交付，调用方没有任意关闭权。

Auth 保持：

```text
auth/service.CredentialVerifier <- auth/adapter/jwt.Verifier(jwx)
auth/service.AuditSink           <- auth/adapter/audit
auth.NewHTTP                     -> auth.Module(project-owned outputs only)
```

不把 Auth/JWT 搬入通用 `pkg/auth`，除非未来证据证明它成为跨业务底层 Capability。

## 3. 底层 Capability 双条件门禁

进入完整底层链必须同时满足：

1. **跨业务复用**：已有至少两个独立业务 owner/应用入口消费同一个稳定能力语义，或资源本身明确不属于任一业务模块；“以后可能复用”不算证据。
2. **进程统一选择**：实现、配置、资源 identity、生命周期或替换策略必须由进程 composition 统一决定，业务模块不能各自安全构造。

SDK、Client、cache、连接、goroutine、registry、exporter 或终结动作只会触发生命周期审计，不会单独满足任何升级条件。分类矩阵固定为：

| 跨业务复用 | 进程统一选择 | 设计结果 |
| --- | --- | --- |
| 否 | 任意 | 留在 `internal/module/<name>`；第三方进该模块 `adapter/<technology>`，运行资源由模块 contribution/Participant 管理 |
| 是 | 否 | 可以形成普通 `pkg` 复用库；不进入 Kernel App，不虚构进程资源 |
| 是 | 是 | 进入完整底层 Capability 链 |

目标路径：

```text
pkg/<capability>                    项目自有调用契约
internal/kernel/app/<capability>    具体实现、配置和生命周期 Definition
internal/kernel/composition         app.Add/Replace 与 Capabilities 输出
internal/composition                连接到模块、Router、Server
```

`pkg` 不允许导入 `internal`，也不暴露第三方；Kernel App 不把私有实例或 Close 权交给消费者；application composition 不自行创建第二套 Client/provider。

每个新 Kernel App 组件的研究必须给出真实消费者、统一选择位置、配置 owner、资源 owner 和生命周期证据，并引用在任务设计中。缺少任一条件时保持模块内或普通 `pkg` 形态，不能用技术复杂度替代边界证据。

## 4. Observability 项目契约

Observability 由 Auth/Todo 业务 HTTP 与 Ops management/diagnostics 等不同模块消费者共同使用，且 registry/provider/exporter 由进程统一选择与治理，因此满足双条件。新增 `pkg/observability` 只表达当前真实消费者需要，不包装整套 OTel/Prometheus API。候选职责应拆成窄契约：

- `HTTPObserver`：按稳定 operation inventory 包装 `http.Handler`；输入/输出只使用标准库和项目 operation view；
- `MetricsEndpoint`：提供只连接项目 registry 的 `http.Handler`；
- `Diagnostics`：返回项目自有低敏 snapshot；
- 资源输出采用 facade/Access，生命周期由 Kernel owner 保留，消费者没有 `Shutdown`/`Close`。

不公开 `trace.Tracer`、`metric.Meter`、`prometheus.Registry`、collector、OTel Provider/Exporter/SpanProcessor 或第三方 Option/Config。

若一个接口同时承担 observation、endpoint、diagnostics、lifecycle 四种职责，应拆分；不得用 `ObservabilityManager` 巨型对象掩盖依赖。

## 5. Observability 底层组件

当前寿命存在两个不变量：

1. Prometheus registry identity 在进程期稳定，不能每次 generation 重复注册或丢失累计值。
2. OTel provider/exporter 随配置候选构造，候选失败不得影响旧代，提交后旧代按预算 flush/stop。

目标设计默认拆成两个组件，避免用一个假统一生命周期破坏语义：

### 5.1 Metrics 组件

- 项目契约：`pkg/observability` 的 metrics recorder/endpoint 窄输出；
- 形态：无运行期配置且进程期稳定时使用 `app.ManagedFixed`，有真实终结动作才声明 finalizer；
- owner：Kernel/process，单实例 identity；
- 输出：稳定 facade/Access，不暴露 Prometheus registry。

### 5.2 Telemetry 组件

- 项目契约：HTTP observation 与低敏 diagnostics；
- 形态：`app.ManagedConfigured` 候选，依赖 Metrics typed Binding；
- Reload：只有能证明新旧 provider 并存、请求 admission 和 exporter flush 时才能使用 `KernelInstanceSwap`；否则 `RestartRequired` 或继续由完整 Application Generation 持有；
- owner：构造 provider/exporter 的底层组件，负责 Start/Ready/terminal flush；
- 输出：稳定 Lease facade，调用期借用不允许第三方对象逃逸。

实施前必须用当前 Kernel App/Generation 证明该形态可表达完整 HTTP generation 切换。若 Lease 无法包围整个 request 或中间件捕获旧实例导致逃逸，停止实施并回到研究，不把裸 provider 交给 application composition。

## 6. Ops 收口

Ops 继续拥有 management、probe、build、diagnostics 用例，不再拥有具体观测技术：

```text
Observability Capability outputs
  -> application Router HTTP observation
  -> Ops metrics endpoint
  -> Ops typed telemetry diagnostics

Ops Module outputs
  -> management HTTP
  -> project-owned management config/completion
```

删除以下导出耦合：

- `ops.Dependencies.Metrics *prometheusadapter.Registry`；
- `ops.Module.Telemetry *oteladapter.Provider`；
- `middleware.HTTP(trace.Tracer, *prometheusadapter.Registry, ...)`；
- `Provider.Tracer() trace.Tracer` 跨包返回。

替换后的 Ops 输入使用 `pkg/observability` 窄契约或已经完成的标准库 `http.Handler`/middleware，不知道 Prometheus、OTel 或底层组件 ID。

## 7. Composition 职责

`internal/kernel/composition`：

- 选择 Metrics/Telemetry Definition；
- 用 typed Binding 声明底层依赖；
- 输出项目自有 Observability Capability；
- 不知道 Auth/Todo/Ops 业务语义。

`internal/composition`：

- 从底层 Capabilities 取得项目自有输出；
- 连接 application Router 与 Ops；
- 不导入 Prometheus/OTel，不持有 registry/provider 具体类型；
- 不在模块之外重新实现 operation label、诊断或错误语义。

## 8. 架构门禁

后续实施必须增加通用规则和正反 fixture：

1. `internal/module/*/model|service` 禁止任意第三方 direct import。
2. `internal/module/*/adapter/**` 可以 direct import 第三方，但 exported 声明禁止第三方 selector。
3. 模块根、binding、Contribution 禁止导入/导出第三方或本模块具体 Adapter 类型。
4. `internal/composition` 禁止直接导入 Observability 第三方包或 `internal/module/*/adapter/**`。
5. `pkg/observability` 与全部 `pkg` 一样禁止第三方类型泄漏和 `internal` 反向依赖。
6. Observability 具体实现只允许从 `internal/kernel/app/observability` 被底层 composition 选择；禁止旁路构造第二套 provider/registry。
7. 新增 Kernel App 组件的任务研究必须记录跨业务消费者与进程统一选择证据；静态 architecture test 负责可执行的 import/export/旁路规则，不能伪装成能推断业务语义。
8. architecture test 使用 import origin 与 AST/type 信息，不维护只针对 `otel`、`prometheus` 的脆弱字符串名单。

## 9. 单轨迁移

1. 先定义 `pkg/observability` 窄契约和独立 contract tests。
2. 建立 Metrics 与 Telemetry 底层 Definition，证明 identity、candidate、flush 和错误语义。
3. production kernel composition 输出项目自有能力。
4. application composition 改用该能力，再收口 Ops 输入/输出。
5. 删除 `internal/module/ops/adapter/prometheus`、`adapter/otel` 和直接 OTel middleware；不保留 alias、wrapper 或 fallback。
6. Auth/JWT 路径不迁移，只补第三方导出和 composition 穿透门禁。
7. 同步当前权威文档和任务证据，执行完整验证。

## 10. 文件影响

确认后的预计范围：

- 新增 `pkg/observability` 及测试/README；
- 新增 `internal/kernel/app/observability` 与底层 composition 接入；
- 修改 Kernel/App Capabilities 与 Application Generation 连接；
- 修改 `internal/module/ops` 的 module、middleware、测试和 README；
- 删除 Ops 模块内 Prometheus/OTel 具体实现；
- 修改通用 architecture/boundary tests；
- 同步根 README、模块指南、`pkg`、Kernel App 与 Ops 当前说明。

预计不修改第三方版本、公开 OpenAPI、配置键/default、Auth/JWT 实现、Todo、migration SQL、Database、Cache、Storage 或外部系统。

## 11. 验证矩阵

### 契约与结构

- Auth Adapter 内第三方可用但零导出泄漏；
- Ops/ application composition 无 Prometheus/OTel import；
- `pkg/observability` 导出 API 只含项目/标准库类型；
- 正反 fixture 证明两条轨道不会互相绕过。

### 生命周期与行为

- metrics registry 跨 generation identity 与累计值保持；
- telemetry candidate 失败保留旧代；提交后旧代 flush/stop；
- queue drop/export/diagnostics、metrics endpoint、trace propagation 与 HTTP 标签行为不变；
- Auth/JWKS、Todo、management 和 reload tests 保持通过。

### 完整门禁

```powershell
gofmt -l .
go generate ./...
git diff --exit-code -- api/openapi.yaml internal/transport/http/api
go mod tidy -diff
go test ./...
go test -race ./...
go vet ./...
go build ./cmd/app
git diff --check
```

## 12. 重新确认触发器

出现以下任一事实时返回研究并重新确认：

- 必须新增/升级/替换第三方依赖或改变配置 schema/default；
- Kernel Lease 无法包围 HTTP 请求，必须暴露裸 tracer/provider；
- metrics identity 与 telemetry generation 无法用现有 Component/Generation 契约表达；
- 必须改变 Prometheus exposition、OTLP 协议、trace propagation 或 diagnostics 行为；
- Auth/JWT 被证明不是模块专属而需要升级为底层 Capability；
- 需要修改公开 HTTP API、数据、migration、部署或外部系统。
