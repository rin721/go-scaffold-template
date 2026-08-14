# 实施任务：Todo 路由中间件示例

## 1. 当前门禁

- 研究门禁：**已通过**，证据见 [R001](research/R001-current-route-middleware-gap/report.md)。
- 计划状态：**已确认**。
- 实施状态：**已完成实现与本地验证**。
- 实施授权：用户已在计划报告后的后续消息中明确要求“执行中间件方案”，授权当前 `MW-001/BIND-001/VER-001/DOC-001`。
- 既有 `.agents/skills` 与 `tmp/` 不是 015 范围。

## 2. 任务依赖

```text
RES-001 -> PLAN-001 -> 用户确认
用户确认 -> MW-001 -> BIND-001 -> VER-001 -> DOC-001
```

## 3. 任务清单

| ID | 工作量 | 任务 | 完成条件 | 状态 |
| --- | ---: | --- | --- | --- |
| `RES-001` | S | 核对全局/route middleware 与既有研究 | 现有调用链、顺序、缺口、可复用 R005 和非目标可复核 | 已完成（文档） |
| `PLAN-001` | S | 冻结 JSON Content-Type 示例 | 415 契约、目录、绑定、兼容影响、测试和文件范围明确 | 已完成（文档） |
| `MW-001` | M | 实现 `RequireJSONContentType` | 合法参数放行；缺失/非法/非 JSON 短路；cause 与下游错误保留 | 已完成（实现） |
| `BIND-001` | S | 绑定 Todo create route | 只有 POST create 绑定 middleware；Contribution 校验与顺序通过 | 已完成（实现） |
| `VER-001` | M | 增加单元、route 与进程验收 | 415、安全 envelope、合法创建及全量门禁通过 | 已完成（本地验证） |
| `DOC-001` | S | 同步当前使用说明与任务证据 | README、Todo 目录、015 状态和验证结果与实现一致 | 已完成（文档） |

## 4. 实施提交与回退

- 用户确认后，015 计划文档、实现、测试和当前文档作为一个 Conventional Commit 提交。
- 只显式暂存 015 文件和确认过的当前文档，排除 `.agents/skills`、`tmp/`、本地配置、SQLite 数据和构建产物。
- 回退该提交即可恢复旧 HTTP 行为；不涉及数据库、配置或外部状态迁移。
- 若实施需要认证、限流、CORS、请求大小配置、`pkg/httpx` 公共 API 或新依赖，退回研究/计划并重新确认。

## 5. 计划阶段记录

| 轮次 | 日期 | 完成任务 | 证据 | Commit | 剩余风险 |
| --- | --- | --- | --- | --- | --- |
| 1 | 2026-08-15 | RES-001、PLAN-001 | `Route.Middlewares -> applicationRouter -> Router.Handle -> chain`；全局 middleware 组合；R005；Todo route 现状；当前 HEAD `2239f4c` | 未提交（计划门禁要求） | 415 会收紧未声明 Content-Type 的既有客户端行为，等待确认 |
| 2 | 2026-08-15 | MW-001、BIND-001、VER-001、DOC-001 | middleware/route/进程测试；全局 Header 包裹 415；合法创建；`go test ./... -count=1`、`go test -race ./... -count=1`、`go vet ./...`、`go build -o NUL ./cmd/app`、`go mod tidy -diff`、文档链接与 `git diff --check` | 本任务 Conventional Commit（见 Git 历史） | 认证、限流、CORS、请求体限制仍为非目标 |
