# 产品需求：Todo 业务垂直切片

## 1. 目标与依据

依据 [R001](research/R001-current-todo-vertical-slice-feasibility/report.md)，当前 Kernel、Database、HTTP、Config、CLI 与生命周期底座可以承载真实业务，但尚无业务对象和 Application CLI 运行路径。

本任务把 Todo 作为首个真实业务能力，不是空目录或占位 CRUD。它必须同时证明业务分层、SQLite 持久化、HTTP 路由、配置 binding、CLI command binding、one-shot 生命周期和架构治理能够闭环。

## 2. 用户决策

- 业务能力：`Todo`。
- 目录风格：模块内分层，使用正确术语 `handler`，不使用拼写错误的 `handle` 或带连字符的 Go 包名。
- 演示深度：HTTP、SQLite、真实配置、真实 CLI 全部实现，并复用同一 Service。
- 当前阶段：计划已由用户在后续消息中确认并完成实施；本文件描述 014 当前冻结的业务与协议契约。

## 3. 业务规则

### 3.1 Todo 模型

Todo 对外包含：

- `id`：由现有 ID Generator 生成的 UUID 字符串，创建后不可修改。
- `title`：去除首尾空白后保存；必须至少 1 个 Unicode 字符，最多 `todo.titleMaxRunes` 个字符。
- `status`：只允许 `pending` 或 `completed`，创建时固定为 `pending`。
- `createdAt`、`updatedAt`：由注入的 Clock 生成并规范化为 UTC。
- `completedAt`：未完成时缺失，完成时记录 UTC 时间。
- `version`：持久化乐观锁字段，不在 HTTP/CLI 公开输出中暴露。

完成操作具有业务语义：`pending -> completed`。串行重复完成返回当前已完成对象，不再次改变完成时间；并发更新使用 Version 检测，冲突返回稳定 conflict，不静默覆盖。

### 3.2 查询

- 按 ID 查询不存在时返回 Todo not-found。
- 列表允许可选 `status`、`offset` 和 `limit`。
- `offset` 必须大于等于 0；`limit` 缺失时使用配置默认值，必须在 `1..maxListLimit` 内。
- 列表按 `createdAt DESC, id ASC` 稳定排序，并返回 `items/offset/limit/total`。

## 4. HTTP 契约

统一前缀为 `/api/v1/todos`，JSON 字段使用 lowerCamelCase。

| 方法与路径 | 输入 | 成功响应 |
| --- | --- | --- |
| `POST /api/v1/todos` | Body `{ "title": string }` | `201` + Todo DTO |
| `GET /api/v1/todos/{id}` | 非空 path `id` | `200` + Todo DTO |
| `GET /api/v1/todos` | Query `status? offset? limit?` | `200` + 列表结果 |
| `PATCH /api/v1/todos/{id}/complete` | 非空 path `id`，无 Body | `200` + 完成后的 Todo DTO |

Todo DTO 为：

```json
{
  "id": "uuid",
  "title": "学习 Go",
  "status": "pending",
  "createdAt": "2026-08-15T10:00:00.000Z",
  "updatedAt": "2026-08-15T10:00:00.000Z",
  "completedAt": null
}
```

列表响应为：

```json
{
  "items": [],
  "offset": 0,
  "limit": 20,
  "total": 0
}
```

HTTP 错误保持现有 `pkg/httpx` envelope：`{"error":"<reason>","message":"<safe message>"}`。公开映射固定为：

| 分类 | HTTP | reason |
| --- | ---: | --- |
| 非法 title、ID、status 或分页 | 400 | `todo_invalid_argument` |
| Todo 不存在 | 404 | `todo_not_found` |
| 乐观锁或状态冲突 | 409 | `todo_conflict` |
| 数据库暂不可用 | 503 | `todo_unavailable` |
| 截止时间超时 | 504 | `todo_timeout` |
| 未知错误 | 500 | `internal_server_error`，不回显内部原因 |

Handler 从 `Accept-Language` 读取语言偏好，通过现有 I18n Translator 与 namespaced message ID 提供安全消息；消息缺失时使用代码内中文默认文案。客户端断开导致的 cancellation 保留 context 原因，不重复记录或伪装成功。

## 5. CLI 契约

所有命令注册为 `CommandModeApplication`，解析和参数校验完成后才启动 Kernel；不启动 HTTP listener 或 config watcher。

| 命令 | 参数 | 副作用 |
| --- | --- | --- |
| `todo create --title <text>` | `title` 必填 | `SideEffectExternalWrite` |
| `todo get --id <id>` | `id` 必填 | `SideEffectExternalWrite` |
| `todo list [--status pending|completed] [--offset 0] [--limit N]` | 可选筛选与分页 | `SideEffectExternalWrite` |
| `todo complete --id <id>` | `id` 必填 | `SideEffectExternalWrite` |

四个命令都声明 `SideEffectExternalWrite`：即使 `get/list` 只有读取用例，one-shot 启动阶段也可能首次执行 additive Schema migration，不能把潜在数据库写入隐藏为只读。成功时 stdout 输出与 HTTP 字段一致的单行 JSON；列表输出列表 envelope。参数错误使用 `ExitUsage=2`，配置错误使用 `ExitConfig=3`，取消使用 `ExitInterrupted=130`，业务/依赖错误使用 `ExitError=1`；stderr 由现有进程边界统一输出，不在 Service 重复打印。

## 6. 配置契约

新增 application-owned `todo` section：

```yaml
todo:
  titleMaxRunes: 120
  defaultListLimit: 20
  maxListLimit: 100
```

- 三项都必须为正整数。
- `titleMaxRunes` 最大为数据库 Schema 固定容量 200。
- `defaultListLimit <= maxListLimit`。
- 该 section 由同一个 config binding 提供 defaults、strict decode 和语义校验。
- `config init` 和 `config.example.yaml` 必须包含该 section。
- 运行期变更该 section 返回 `RestartRequired`，不进行动态业务对象替换。

## 7. 持久化契约

- 默认验收数据库使用现有 SQLite 配置 `.data/app.db`。
- 表名 `todos`；字段为 `id/title/status/created_at/updated_at/completed_at/version`。
- `id` 是字符串主键，`title` 固定容量 200，`status` 固定容量 16，`completed_at` 可空，`version` 启用乐观锁。
- 建立 `status + created_at` 普通索引；Schema 名称和字段只在 model binding 中定义一次。
- 启动时由 Todo schema participant 在 Database Access 租约内执行 additive `Migrate`；迁移完成后才允许 CLI operation 或 HTTP listener 就绪。
- Repository 不保存 borrowed Client/Tx；每次操作都在 `Use/WithinTx` 回调内完成并把 database 错误转换为 Todo/`fault` 分类。

## 8. 目录与依赖要求

目标目录：

```text
internal/business/
├── contracts.go
├── README.md
└── todo/
    ├── model/
    ├── service/
    ├── repo/
    ├── handler/
    ├── binding/
    │   ├── model/
    │   ├── config/
    │   ├── http/
    │   └── cli/
    ├── module.go
    └── README.md
```

`model` 不导入 Kernel、HTTP、CLI、Database 或第三方库；`service` 只依赖 model 和自己定义的 Repository port；`repo/handler/binding` 依赖 service；只有 application composition 同时知道 Kernel capabilities 和业务 module。

## 9. 非目标

- 不实现删除、改标题、批量操作、标签、截止日期、优先级、用户归属或认证授权。
- 不实现 Cache、消息、任务调度、OpenAPI、Web UI、管理健康端点或动态路由。
- 不新增第三方依赖，不暴露 GORM、chi 或 Cobra 类型给 model/service。
- 不实现破坏性或版本化数据库迁移系统，不删除用户已有数据。
- 不为其他未知模块预建空目录、万能 Module SDK、自动扫描或运行期 Resolver。

## 10. 验收标准

- 默认 SQLite 下，CLI 创建的数据可被后续 CLI 和 HTTP 进程查询；HTTP 创建的数据也可被 CLI 完成。
- HTTP 四条路由的成功、非法输入、not-found、conflict、取消和依赖失败有边界测试。
- CLI 四个命令的解析、输出、退出码、配置失败、取消和资源反向停止有测试。
- `config init` 生成 Todo 默认节，严格加载拒绝未知字段、错误类型和非法值。
- 服务启动顺序为 Kernel → Todo migration → HTTP；CLI 为 Kernel → Todo migration → operation → 反向停止。
- package graph、module/route/command/schema 冲突和 borrowed resource 不逃逸有可执行门禁。
- `go test ./...`、`go test -race ./...`、`go vet ./...`、`go build ./cmd/app`、`go mod tidy -diff` 和 `git diff --check` 通过；真实进程 smoke 使用临时 SQLite 与临时端口，不污染固定用户数据。
