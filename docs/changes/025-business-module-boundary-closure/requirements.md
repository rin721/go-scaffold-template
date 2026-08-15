# 需求：业务模块边界收口

## 1. 背景

现有架构规则已经要求业务能力按 `internal/module/<name>` 纵向收口，但 Todo 手写 HTTP Adapter 位于 `internal/transport/http`，application Router 直接构造 Todo Handler，`cmd/app` 也直接使用 Ops 内部 Model。当前 package graph 没有阻止这些外溢，模块开发指南与 Todo 当前说明因此互相冲突。

本需求由 [R001](research/R001-current-business-module-boundary/report.md) 和用户确认的模块/Capability 边界支撑。

## 2. 目标

- 让所有手写 Todo HTTP 语义回到 Todo 模块。
- 让 Todo 不导入 Auth，跨模块身份和授权只经 Todo-owned 窄端口与 composition Adapter 连接。
- 让 application composition root 只构造底层能力、连接模块并安装模块完成品，不实现 Todo transport。
- 让 `cmd/app` 只依赖 `internal/composition`，不直接导入业务模块内部类型。
- 让自动 package graph 门禁阻止同类外溢再次出现。
- 保持全局 OpenAPI authority、生成协议、公开行为、配置、生命周期和依赖不变。

## 3. 范围

### 3.1 包含

- `internal/transport/http/todo.go` 及其测试迁入 `internal/module/todo/binding/http`。
- Todo-owned HTTP request access port 与 composition-owned Auth Adapter。
- Todo HTTP profile 的完成品输出和 Router 安装方式调整。
- `cmd/app` 到 Ops BuildInfo 的边界收口。
- package graph 测试与当前权威文档。
- 旧路径、旧 imports、旧 package 名和失效说明的单轨删除。

### 3.2 不包含

- 修改 `api/openapi.yaml` 的路径、operation、DTO、security 或兼容语义。
- 修改生成器版本、第三方依赖、HTTP 中间件政策、Auth/JWT、Todo 用例、数据库或 migration 行为。
- 新增第二个 HTTP 业务模块或设计通用动态 Router Registry。
- 新增 Kernel Capability、Component、资源、goroutine 或 Reload 策略。
- 为没有真实职责的模块补空目录、接口或占位实现。
- push、tag、Release、部署或外部系统写入。

## 4. 核心需求

### MOD-OWN-001 模块收口

Todo-owned Model、Service、Repository、Adapter、binding、局部装配和 contribution 必须位于 `internal/module/todo`。顶层 `internal/transport/http` 不得保留手写 Todo 业务映射。

### HTTP-001 协议单一权威

`api/openapi.yaml` 继续是 operation、路径、DTO、security 与兼容性的唯一 authority；`internal/transport/http/api` 只保存由它生成的协议代码和 inventory。迁移不得复制或手写第二份路径与 operation metadata。

### HTTP-002 Todo HTTP profile

Todo 模块必须提供显式 HTTP profile，完成 Repository、Service 和 HTTP Handler 的纯内存构造。Handler 依赖 Todo UseCases、I18n 与 Todo-owned request access port，不依赖 Auth module、Kernel、composition 或完整 Capabilities。

### CROSS-001 跨模块窄端口

Todo HTTP binding 自己定义最小 request access 契约，至少能从 context 获得 Todo Actor 并按 operation ID 请求授权。composition Adapter 负责把 Auth Principal/Service 映射到该契约，并保留未认证、拒绝和底层错误的可识别语义。

### COMP-001 组合根职责

`internal/composition` 可以同时知道 Kernel Capability 和多个模块，也可以实现它们之间的窄 Adapter；不得实现 Todo DTO 映射、Todo 错误呈现或 Todo Handler。application Router 只接收并安装已完成的业务 Handler。

### ENTRY-001 入口依赖

`cmd/app` 只能通过 `internal/composition` 提交应用身份、构建信息和启动输入，不得直接导入 `internal/module/*`、`internal/kernel/*` 或第三方业务实现。

### CONTRIB-001 完成品与 contribution

Todo HTTP profile 必须把 Service、HTTP Handler 和 module Contribution 作为明确、非 nil 的完成品返回。Contribution 继续声明模块 ID 和真实 lifecycle Participants；本任务不得把路由、配置、Health 或未来入口塞进无语义的万能 Registry。

### ASYM-001 按需目录

模块只建立真实需要的 Model、Service、Repository、Adapter、Handler、middleware 和 binding。Auth、Ops、Migration 缺少某层时，若没有对应业务职责，不得为了目录对称补空实现。

### CAP-001 Capability 升级门禁

模块专属第三方 SDK、Client、cache、goroutine、migration 或 Adapter 默认留在模块。只有研究证明其跨业务复用、拥有稳定项目契约并由进程统一选择和治理时，才能进入 `pkg/<name> -> internal/kernel/app/<name> -> internal/kernel/composition`。

### GOV-001 可执行导入门禁

package graph 至少强制：

- `cmd/app` 只能导入 application composition root 和标准库；
- composition 之外的包不得直接导入其他 `internal/module/<name>` 内部包；
- 一个业务模块不得导入另一个业务模块、Kernel composition 或 application composition；
- module core 继续不得导入 HTTP、CLI、Database 具体边界；module binding/Adapter 只按真实职责使用项目契约；
- `internal/transport/http` 除纯生成 API 包外不得依赖业务模块。

测试 fixture 必须同时证明合法图通过、Todo transport 外溢和跨模块直连会失败。

### MIG-001 单轨替换

迁移完成后删除旧顶层 Todo transport 文件、旧 imports、旧构造入口和失效文档；不得保留 alias、wrapper、deprecated package 或双路由兼容层。

## 5. 验收标准

1. `rg` 证明 `internal/transport/http` 除生成 `api` 外没有手写 Todo Handler、Todo Model/Service 或 Auth Principal 依赖。
2. Todo HTTP Handler、DTO 映射、错误呈现和协议测试均位于 `internal/module/todo/binding/http`。
3. Todo 模块生产代码不导入 Auth、Kernel 或 composition；HTTP binding 只依赖 Todo-owned port 和项目协议/底层契约。
4. `applicationRouter` 不再构造 Todo transport，只安装 Todo HTTP profile 返回的 Handler。
5. `cmd/app` 不再导入 `internal/module/*`。
6. OpenAPI 文件、生成文件内容和公开 Todo HTTP/CLI 行为不变；生成 clean diff 门禁通过。
7. Auth、Ops、Migration 不出现为了对称新增的空 repo/handler/binding。
8. package graph 正向 fixture 通过，transport 外溢、跨模块直连、cmd 直连模块和模块反向依赖 fixture 均失败。
9. 当前权威文档统一描述模块内 HTTP binding；历史变更只保留事实，不再作为当前边界说明。
10. `gofmt -l .`、`go generate ./...` clean diff、`go test ./...`、`go test -race ./...`、`go vet ./...`、`go build ./cmd/app`、`go mod tidy -diff` 和 `git diff --check` 全部通过。

## 6. 非功能约束

- 保留错误链、取消、超时、401/403 与 RFC 9457 响应语义。
- 不泄露 Token、Principal 原始 claims、DSN 或内部错误文本。
- 不增加运行时扫描、反射、全局 Registry 或隐藏依赖。
- 注释、文档与测试场景以中文为主。
- 本任务不得降低 024 已建立的 OpenAPI、安全、代际、迁移和观测门禁。
