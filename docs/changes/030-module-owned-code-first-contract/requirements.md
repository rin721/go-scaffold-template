# 需求：模块自有代码优先 HTTP 契约（030）

## 1. 依据

- `R001`：当前是 spec-first 单轨（openapi.yaml -> 代码），authority 在全局 YAML；模块契约不归模块；transport 直接使用第三方库；ADR-003 已固定该方向。用户要求反转方向、模块化契约、通用能力先行、第三方封装。
- `R002`：typed code-first 工具链可行（invopop/jsonschema + kin-openapi + yaml.v3 已在依赖树）；不需要新增第三方生成框架。
- `R003`：通用契约能力应落在 pkg 层，模块/transport/composition 分层消费；迁移必须通用能力先行、Todo 单轨替换、删除旧链、同步门禁与文档。

## 2. 目标

将 HTTP 契约 authority 从 `api/openapi.yaml` 反转为模块自有的 typed 代码声明，并让契约文件（openapi.yaml）与 operation inventory 由代码生成。改造后：

```text
模块 typed 契约声明（authority）
  -> 生成器渲染 api/openapi.yaml + operation_inventory.gen.go（产物）
  -> transport 从同一份声明做一次路由绑定、校验与错误呈现
  -> composition 聚合模块契约并连接 Auth/Ops
```

模块不得再依赖全局生成包 `internal/transport/http/api`，第三方库不得泄漏到模块与 composition。

## 3. 术语

- **模块自有契约**：`internal/module/<name>/binding/http` 中以项目自有类型声明的 operation 集合（method/path/operationId/policy/security/DTO 引用）。
- **code-first 生成**：以 Go 代码为唯一 authority，由项目自有生成器产出 `api/openapi.yaml` 与 `operation_inventory.gen.go`，不再由 openapi.yaml 生成 Go 代码。
- **通用契约能力**：pkg 层项目自有能力，封装 schema 派生、OpenAPI 渲染与运行期绑定所需的第三方库，对外只暴露项目自有类型。
- **运行期 binder**：transport 中一次性把聚合后的契约声明绑定为路由、validator 与问题呈现的组件。

## 4. 功能要求

### `REQ-001` 模块自有契约

每个业务 HTTP 模块在 `binding/http` 中以项目自有类型声明自己拥有的操作，包括 method/path、operationId、policy、security 与 DTO 类型引用。模块不修改全局 `api/openapi.yaml`，不创建 Router，不加载 OpenAPI，不调用生成 binding。

### `REQ-002` 契约文件由代码生成

`api/openapi.yaml` 与 `operation_inventory.gen.go` 必须由项目自有生成器从模块契约声明生成；`go generate` 后必须 clean diff。不再由 openapi.yaml 生成 DTO、接口或路由代码；删除 oapi-codegen 生成链。

### `REQ-003` 通用能力先行

先落地 pkg 层通用契约能力与生成器，并在生成器输出与当前 `api/openapi.yaml` 等价后，再迁移 Todo。通用能力必须先于任何模块迁移完成并独立验证。

### `REQ-004` 第三方零泄漏

模块、composition、cmd 中不得出现 chi、kin-openapi、nethttp-middleware、jsonschema 类型；第三方只存在于 pkg 通用能力与 transport 内部。架构门禁必须持续阻止这些依赖泄漏。

### `REQ-005` 单一运行期绑定

整份公开契约的 spec 校验、operation policy、strict 边界、404/405 与问题呈现必须只在 transport 装配一次，不按模块重复。模块即使增加，也不复制 Router、validator 或 method/path 表。

### `REQ-006` 行为兼容

以下行为必须保持：

- 四个 Todo 路径、method、DTO JSON 形态、operationId、policy、security 与兼容性（oasdiff 不报 ERR）；
- bearer 认证、401/403、对象隐藏、I18n、审计语义；
- invalid JSON、未知字段、Content-Type、404/405 与 RFC 9457 Problem Details；
- request ID、日志、trace、metrics、rate/overload、timeout、body limit、CORS 与 Application Generation reload/listener/admission/drain。

### `REQ-007` 单轨迁移

迁移完成后不得保留旧生成物、旧依赖或兼容 wrapper：删除 oapi-codegen 工具依赖与配置、`api.gen.go`、`StrictServerInterface` 断言、`HandlerWithOptions` 调用、`nethttp-middleware` 依赖与旧文档入口。禁止双轨并行。

### `REQ-008` 新模块扩展面

新增业务 HTTP 模块的 HTTP 接入只允许触及：模块自有契约声明、模块 Handler 适配器、composition 聚合、对应测试与文档；不得要求修改既有模块、transport 已装配代码或全局 YAML。

## 5. 质量要求

| 标准 | 可验收定义 |
| --- | --- |
| 简单 | 模块契约声明、生成器、transport binder、composition 聚合各只有一处明确 owner |
| 高效 | 每代只构建/校验一份 OpenAPI、只绑定一次路由树；运行期不扫描、不反射发现路由 |
| 通用 | 增加模块只增加声明与聚合点，不复制绑定或路由表 |
| 明确 | authority（代码）与产物（YAML/inventory）可区分；owner 可从 package 与类型看出 |
| 可验证 | 生成 golden 对比、结构门禁、协议测试、进程测试与完整 gate 同时通过 |

## 6. 范围

### 包含

- pkg 层通用契约能力与生成器；
- Todo 模块契约化迁移；
- transport 单一运行期 binder；
- composition 聚合与连接调整；
- 架构门禁、测试、生成物与权威文档同步。

### 不包含

- 新增第二个真实业务模块或假业务示例；
- 修改公开 HTTP 行为、版本或路径；
- 动态 route registry、扫描、插件或 Service Locator；
- 修改 Kernel、Host、listener、配置 schema、migration、Database 或 module Contribution；
- 新增第三方依赖（现有依赖树已足够；允许将 `invopop/jsonschema` 从 indirect 提升为 direct）。
- push、tag、Release、部署或数据库操作。

## 7. 验收标准

1. `go generate ./...` 后 `api/openapi.yaml` 与 `internal/transport/http/api` 无意外 diff。
2. 生成器输出与迁移前 `api/openapi.yaml` 的公开语义等价（oasdiff 无 ERR；结构字段逐一对比）。
3. 模块 HTTP binding 不再 import `internal/transport/http/api`；`api.StrictServerInterface`、`api.HandlerWithOptions`、`api.GetSwagger` 在 production 代码中零引用。
4. 模块、composition、cmd 中 chi/kin-openapi/nethttp-middleware 零引用；架构门禁正反 fixture 通过。
5. transport 仍是唯一 route binding 位置，每代只构造一次；composition 仍是唯一聚合位置。
6. Todo 全部现有 HTTP、Auth、I18n、Ops 与 process tests 保持通过。
7. 完整 Go test/race/vet/build/tidy、Markdown 链接与 `git diff --check` 通过。
8. `api/README.md`、应用模块开发指南、Todo README、`internal/module/README.md` 与根 README 的“API 文档”入口同步为新方向的当前说明。

## 8. 确认要求

这是非纯文档实施计划：将修改源码、生成物、依赖与测试。只有用户在本计划报告之后的后续消息明确确认 030 当前方案，才能开始实施。若实施中发现必须改变公开契约、依赖清单、Kernel/Host、module Contribution、路由生成策略或既有 ADR 结论，必须退回研究并重新确认。