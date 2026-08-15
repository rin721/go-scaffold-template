# 本地启动指南

本文是本仓库本地启动、首次配置和首次迁移的当前权威说明。更详细的字段含义见 [配置说明](../configuration/README.md)，生产迁移顺序见 [数据库迁移与回滚](../operations/migration-and-rollback.md)。

## 命令关系

| 命令 | 作用 | 是否创建外部资源 |
| --- | --- | --- |
| `go run ./cmd/app config init` | 生成并回环校验默认 `config.yaml` | 写入配置文件，不创建数据库表 |
| `go run ./cmd/app db migrate status` | 只读检查迁移状态 | 可能打开数据库连接，不执行 DDL |
| `go run ./cmd/app db migrate up` | 执行当前二进制要求的前滚迁移 | 创建或修改数据库 schema |
| `go run ./cmd/app` | 启动长期 Service | 绑定 HTTP 与 management listener，只读校验 schema |

首次启动推荐顺序：

```powershell
go run ./cmd/app config init
go run ./cmd/app db migrate up
go run ./cmd/app
```

`status` 是检查命令，不是首次启动前的必需步骤；如果在 `up` 之前执行，看到数据库尚未兼容是正常结果。

## 成功信号

Service 启动成功后，日志应出现 `application generation started` 与 `application ready`。默认 readiness 检查：

```powershell
Invoke-RestMethod http://127.0.0.1:9090/readyz
```

业务 HTTP 默认地址是 `127.0.0.1:8080`，management 默认地址是 `127.0.0.1:9090`。停止进程使用 `Ctrl+C`；正常停止会进入 draining，然后打印 stopped 相关日志。

## 已有配置文件

`config init` 默认拒绝覆盖已有 `config.yaml`。需要比较新默认值时，先生成到临时路径：

```powershell
go run ./cmd/app config init --output .data/generated-config.yaml
```

只有确认要丢弃当前本地修改时才使用 `--force`。配置文件、环境变量和示例文件的关系见 [配置说明](../configuration/README.md)。

## 日常开发流程

修改代码后通常只需要重新启动 Service。涉及迁移 SQL、迁移目标版本或 Todo schema 兼容性时，先执行：

```powershell
go run ./cmd/app db migrate status
go run ./cmd/app db migrate up
go run ./cmd/app db migrate status
```

Service 自身不会执行 DDL；它只在启动和 reload 时只读校验数据库是否兼容。

## 常见问题

| 现象 | 处理方式 |
| --- | --- |
| `missing config` 或找不到配置 | 确认当前目录是仓库根目录，先执行 `config init`，或显式传入 `--config`。 |
| `target config already exists` | 这是防覆盖保护；改用 `--output` 生成到临时路径对比。 |
| 生成的配置被报告 `unknown config section` | 这是应用配置 owner 漂移，应作为回归处理；生成、迁移和 Service 必须识别同一套官方配置节。 |
| migration 显示版本不兼容或 dirty | 先读 `status` 输出并按 [数据库迁移与回滚](../operations/migration-and-rollback.md) 处理，不要手改版本表冒充成功。 |
| tracing 启用后启动失败 | 默认 tracing 是禁用的；启用时必须提供合法 OTLP endpoint。HTTP endpoint 仅允许在明确 `insecure: true` 且符合本地受控场景时使用。 |
| 端口被占用 | 释放占用进程，或用 `APP_HTTP__ADDR`、`APP_MANAGEMENT__ADDR` 覆盖地址。 |

## 日志要求

开发阶段默认日志级别可见 debug。启动、reload、ready、drain、migration 和失败路径都应有结构化日志；新增开发代码必须遵守 [开发日志规范](../development/logging.md)，不能用 `fmt.Println`、静默吞错或只在测试里观察状态代替日志。
