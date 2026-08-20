# go-scaffold-template

`go-scaffold-template` 是一个 Go HTTP 服务脚手架，用一套显式 composition root 连接配置、日志、数据库、迁移、HTTP、management 和业务模块。当前默认应用包含 Todo 垂直切片，可作为本地开发、模块扩展和交付流程的参考实现。

## 五分钟本地启动

前置条件：安装仓库要求的 Go 版本，并在仓库根目录执行命令。

```powershell
go run ./cmd/app config init
go run ./cmd/app db migrate up
go run ./cmd/app
```

启动成功后，日志中应能看到 `application generation started` 与 `application ready`。默认 management readiness 地址：

```powershell
Invoke-RestMethod http://127.0.0.1:9090/readyz
```

停止服务使用 `Ctrl+C`，正常退出会打印 draining/stopped 相关日志。

如果本地已经存在 `config.yaml`，`config init` 会拒绝覆盖。不要为了“重新生成”随手使用 `--force`；需要对比时先输出到临时路径，详细关系见 [本地启动指南](docs/getting-started/local-development.md) 与 [配置说明](docs/configuration/README.md)。

## 文档地图

| 你要做什么 | 入口 |
| --- | --- |
| 本地启动、首次迁移、常见启动错误 | [本地启动指南](docs/getting-started/local-development.md) |
| 配置来源、环境变量、`config init`、`config.example.yaml` | [配置说明](docs/configuration/README.md) |
| API 路由与接口清单 | [API 文档](api/README.md) |
| 开发应用模块 | [应用模块开发](docs/development/application-module-development.md) |
| 声明、生产与消费消息 | [消息系统适配能力](docs/development/messaging-capability.md) |
| 声明 cron / fixedDelay 定时任务 | [定时调度能力](docs/development/scheduled-task-capability.md) |
| 日志打印规范 | [开发日志规范](docs/development/logging.md) |
| Kernel 与 Application Generation | [Kernel 运行与配置](internal/kernel/README.md) |
| Kernel App 组件 | [Kernel App 组件](internal/kernel/app/README.md) |
| 底层能力库 | [pkg 能力说明](pkg/README.md) |
| 构建、迁移、发布和安全运维 | [运维文档](docs/operations/README.md) |
| 任务级研究、计划与实施证据 | [变更记录](docs/changes/README.md) |
| AI Agent 协作规则 | [AGENTS.md](AGENTS.md) |

`docs/changes` 是历史账本，不作为当前启动或配置的唯一说明。当前使用方式以上面的主题文档为准。

## 架构摘要

应用入口 `cmd/app` 只负责进程 I/O、基线日志和信号处理；`internal/composition` 显式装配 Bootstrap CLI、migration one-shot 与长期 Service。长期 Service 使用 Application Generation 管理配置快照、资源复用、listener、定时任务与消息 Consumer 准入交接、ready 状态和优雅停止。

配置严格按 owner 注册，未知配置节会在资源副作用前失败；`config init`、`db migrate` 和长期 Service 必须识别同一套应用配置节，避免“生成的配置自己不能启动”的漂移。

日志是开发必备能力。开发阶段默认可见 debug 级别日志，业务和基础设施代码必须遵守 [开发日志规范](docs/development/logging.md)，在真正决定处理策略的边界记录，避免泄露凭据或重复打印同一错误链。

## License

Copyright 2026 Rin721.

This project is licensed under the Apache License 2.0.

You are free to use, modify, distribute, and use this project
for commercial purposes subject to the terms of the license.

See [LICENSE](./LICENSE) and [NOTICE](./NOTICE) for details.
