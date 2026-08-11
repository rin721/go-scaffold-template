# 产品需求：应用启动入口

## 背景

仓库已经实现 Kernel、Database Capability、显式 composition、Host 和启动前 `config init`，但没有 `package main`，使用者无法直接构建或运行项目。

## 功能需求

- 在 `cmd/app` 提供唯一进程入口。
- 有命令行参数时进入启动前 CLI，至少允许在配置文件不存在时执行 `config init`。
- 无参数时按 `config.yaml`、`APP_` 环境变量的覆盖顺序加载配置，经 `kernel.New -> composition.Compose -> kernel.NewHost -> Host.Run` 启动。
- 进程必须响应 `SIGINT` 和 `SIGTERM`，由 Host 负责完整停止流程。
- CLI 参数错误继续使用 `pkg/cli` 的稳定退出码；其他启动错误返回非零退出码并保留错误链。
- Database 配置不完整、配置文件缺失或连接失败时必须启动失败，不得使用假实现、隐藏默认值或空服务掩盖问题。
- 本地配置文件可能包含 DSN，不得进入 Git。

## 验收标准

- `go build ./cmd/app` 成功。
- `go run ./cmd/app config init --output <path>` 在默认配置文件不存在时成功，并生成 Database 默认配置骨架。
- 无参数运行时会真实加载配置并启动 Database；缺少配置时能识别到原始文件错误。
- 非法 CLI 命令返回 usage 退出码，错误输出包含应用入口上下文。
- 单元测试、race、vet 和 Diff 检查通过。

## 非目标

- 不新增 HTTP、RPC、后台任务或具体业务 Participant。
- 不内置 PostgreSQL/MySQL，不增加用于伪造启动成功的内存 Database。
- 不新增运行时 Service Locator、自动扫描或第二套生命周期。
- 不把含真实 DSN 的本地配置提交为示例文件。
