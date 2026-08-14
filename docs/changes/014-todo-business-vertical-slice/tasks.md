# 实施任务：Todo 业务垂直切片

## 1. 当前门禁

- 研究门禁：**已通过**，证据见 [R001](research/R001-current-todo-vertical-slice-feasibility/report.md)。
- 计划状态：**已确认**。
- 实施状态：**已完成实现与本地验证**。
- 实施授权：用户已在计划报告后的独立消息中明确要求“执行014方案”，授权当前 `BUS/DB/CFG/HTTP/CLI/CMP/GOV/VER` 任务。
- 计划阶段不暂存、不提交、不启动服务；现有 `.agents/skills` 与 `tmp/` 内容不属于 014。

## 2. 任务依赖

```text
RES-001 -> PLAN-001..004 -> 用户确认
用户确认 -> BUS-001
BUS-001 -> DB-001, CFG-001
DB-001 + CFG-001 -> HTTP-001, CLI-001
HTTP-001 + CLI-001 -> CMP-001
CMP-001 -> GOV-001 -> VER-001
```

## 3. 研究与计划任务

| ID | 工作量 | 任务 | 完成条件 | 状态 |
| --- | ---: | --- | --- | --- |
| `RES-001` | M | 核验当前 Todo 垂直切片可行性 | 当前入口、Config、Database、HTTP、CLI、Supervisor、治理与既有研究完成复核 | 已完成（文档） |
| `PLAN-001` | M | 冻结业务与协议需求 | Model、规则、HTTP、CLI、配置、Schema、非目标和验收无未决项 | 已完成（文档） |
| `PLAN-002` | L | 完成实现设计 | 目录、接口、数据流、生命周期、错误、迁移和验证决策完整 | 已完成（文档） |
| `PLAN-003` | S | 建立稳定实施任务 | 任务 ID、依赖、范围、完成条件和确认状态明确 | 已完成（文档） |
| `PLAN-004` | S | 验证并提交计划报告 | metadata、链接、文本、Diff、Git 范围通过；无实现或 Git 副作用 | 已完成（文档） |

## 4. 已确认实施任务

### BUS-001：Todo Model 与 Service

- 状态：已完成（实现）。
- 工作量：L。
- 修改：建立 `internal/module/todo/model`、`service` 和模块 README。
- 完成条件：
  - Todo/Status/title/complete 不变量与 UTC 时间语义落实；
  - UseCases、Command/Query/Result 和 caller-owned Repository port 无基础设施类型；
  - Service 显式注入 Repository/Clock/ID/Config；
  - 成功、边界、幂等、冲突、取消、ID/Clock/Repository 失败测试通过。

### DB-001：Repo、Model binding 与 SQLite migration

- 状态：已完成（实现）。
- 工作量：L。
- 依赖：BUS-001。
- 修改：建立 `repo`、`binding/model` 与 migration Participant。
- 完成条件：
  - Record/Model 显式转换，Schema 与字段只定义一次；
  - Create/Get/List/Save 全部在 Access 租约内，borrowed Client/Repository/Tx 不逃逸；
  - additive migration 在 HTTP/operation 前完成；
  - 临时 SQLite 覆盖 not-found、version conflict、取消和错误转换。

### CFG-001：Todo Config binding

- 状态：已完成（实现）。
- 工作量：M。
- 依赖：BUS-001。
- 修改：建立 `binding/config`，扩展 Bootstrap application binding 参数，更新默认配置和 `config.example.yaml`。
- 完成条件：
  - `titleMaxRunes/defaultListLimit/maxListLimit` defaults、strict decode、语义校验同源；
  - `config init` 生成 Todo section；
  - unknown/type/zero/bounds/default>max 均在资源副作用前失败；
  - Todo section reload 变化返回 RestartRequired。

### HTTP-001：Handler、Route binding 与 Presenter

- 状态：已完成（实现）。
- 工作量：L。
- 依赖：BUS-001、CFG-001。
- 修改：建立 `handler`、`binding/http`、业务 contribution 与 router validator。
- 完成条件：
  - 四条 API 与 JSON schema 完全符合 requirements；
  - Handler 只依赖 UseCases/I18n，不访问 Repository/Database；
  - fault 到 HTTP/reason/I18n 安全映射完整；
  - ID、route canonicalization、重复项和 middleware 顺序测试通过。

### CLI-001：CLI command binding 与 one-shot Supervisor

- 状态：已完成（实现）。
- 工作量：L。
- 依赖：BUS-001、DB-001、CFG-001。
- 修改：建立 `binding/cli`，为 Supervisor 增加 `RunOperation`，扩展 Bootstrap command 聚合。
- 完成条件：
  - `todo create/get/list/complete` 为真实 Application commands，调用同一 UseCases；
  - 解析完成前不创建 Kernel/数据库，运行时不启动 HTTP/Watcher；
  - stdout 单行 JSON、stderr/退出码、取消和 cleanup 错误符合契约；
  - RunOperation 覆盖 success/op error/start error/stop error/cancel/timeout/duplicate-run。

### CMP-001：Application composition 与进程接入

- 状态：已完成（实现）。
- 工作量：L。
- 依赖：DB-001、HTTP-001、CLI-001。
- 修改：建立 `internal/composition`，收敛 `cmd/app`，替换 NotFound-only HTTP 装配。
- 完成条件：
  - Service 顺序为 Coordinator → Todo Migrator → Application → HTTP Server；
  - CLI 顺序为 Coordinator → Todo Migrator → operation → reverse Stop；
  - HTTP 与 CLI 共享 Todo module/service 构造，未创建第二个底层客户端或生命周期容器；
  - 服务未匹配路由仍 404，四条业务路由真实访问 SQLite。

### GOV-001：单轨治理与当前文档

- 状态：已完成（实现）。
- 工作量：M。
- 依赖：CMP-001。
- 修改：扩展 package graph/冲突门禁，更新根 README、business README、012/014 状态与使用说明。
- 完成条件：
  - model/service 到 Kernel/协议/第三方的禁用 import 可执行；
  - 只有 application composition 可连接 Kernel composition 与业务模块；
  - 旧 `http.NotFoundHandler` 默认装配、旧 Bootstrap 签名和失效“无业务路由”说明零残留；
  - 没有空目录、兼容双轨或未使用依赖。

### VER-001：真实垂直切片验收

- 状态：已完成（本地验证）。
- 工作量：L。
- 依赖：GOV-001。
- 完成条件：
  - 临时 SQLite 上连续执行 CLI create/list/complete，并由 HTTP get/list 读取同一数据；
  - 独立进程 HTTP 覆盖 create/get/list/complete、非法输入、not-found、取消和 graceful stop；
  - `go test ./...`、`go test -race ./...`、`go vet ./...`、`go build ./cmd/app`、`go mod tidy -diff`、`git diff --check` 通过；
  - 完整 Diff、敏感信息、配置/数据文件和本任务范围审阅通过；
  - 未执行的远程数据库、Docker 或部署门禁明确记录，不能用 SQLite 替代声明。

## 5. 实施提交与回退

- 用户确认后，014 计划文档、实现、测试和当前权威文档作为同一任务提交；计划阶段不单独 commit。
- 只显式暂存 014 文件，排除既有 `.agents/skills` 迁移、`tmp/`、本地 `config.yaml`、SQLite 数据和构建产物。
- Schema 只有 additive 变更，不提供自动降级。实施失败时回退代码提交；测试使用临时数据库，不删除或修改用户已有 `.data/app.db`。
- 如果实现发现必须新增依赖、改变公开 API/config/schema、加入认证或执行破坏性迁移，立即回到计划待确认，不顺手扩大范围。

## 6. 执行记录

| 轮次 | 日期 | 完成任务 | 证据 | Commit | 剩余风险 |
| --- | --- | --- | --- | --- | --- |
| 1 | 2026-08-15 | RES-001、PLAN-001..004 | R001；当前 HEAD；全量与定向 Go 基线测试；8 个 scoped 文件的 metadata、UTF-8、行数、空白、末尾换行、非代码链接、敏感赋值、Diff 与 Git 范围检查 | 未提交（计划门禁要求） | 全部实现任务待用户确认 |
| 2 | 2026-08-15 | BUS-001、DB-001、CFG-001、HTTP-001、CLI-001、CMP-001、GOV-001、VER-001 | Todo 单元/SQLite Adapter/HTTP/CLI/配置重启/跨入口进程测试；`go test ./... -count=1`、`go test -race ./... -count=1`、`go vet ./...`、`go build -o NUL ./cmd/app`、`go mod tidy -diff`、架构门禁、文档链接与 `git diff --check` | 本任务 Conventional Commit（见 Git 历史） | PostgreSQL/MySQL、Docker、部署未执行；不属于 014 本地 SQLite 验收范围 |
