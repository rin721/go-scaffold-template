# go-scaffold-template

`go-scaffold-template` 是一个 Go HTTP 服务脚手架，用显式 composition root 串联配置、日志、数据库、迁移、HTTP、management、后台任务、定时调度、消息系统和业务模块。当前默认应用包含 Todo 垂直切片，可作为本地开发、模块扩展和交付流程的参考实现。

## 五分钟本地启动

前置条件：安装仓库要求的 Go 版本，并在仓库根目录执行命令：

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

## 项目手册

完整文档从 [docs/README.md](docs/README.md) 进入，按项目真实使用路径连续组织：认识项目、启动项目、使用能力、开发业务、接入基础设施、理解架构、扩展能力、调试排障、运行维护和深入底层设计。

| 阅读节点 | 入口 |
| --- | --- |
| 本地启动与首次迁移 | [本地启动指南](docs/getting-started/local-development.md) |
| 配置来源、环境变量和默认配置生成 | [配置说明](docs/configuration/README.md) |
| 应用模块、日志、执行、调度和消息开发 | [开发指南](docs/development/README.md) |
| Kernel、Application Generation 和模块边界 | [架构说明](docs/architecture/README.md) |
| API 路由与契约生成结果 | [API 文档](api/README.md) |
| 构建、迁移、发布、复制、安全、排障和运行维护 | [运维文档](docs/operations/README.md) |
| 研究快照与任务证据 | [研究档案](docs/research/README.md)、[变更记录](docs/changes/README.md) |

## 架构摘要

应用入口 `cmd/app` 只负责进程 I/O、基线日志和信号处理；`internal/composition` 显式装配 Bootstrap CLI、migration one-shot 与长期 Service。长期 Service 使用 Application Generation 管理配置快照、资源复用、listener、定时任务与消息 Consumer 准入交接、ready 状态和优雅停止。

配置严格按 owner 注册，未知配置节会在资源副作用前失败；`config init`、`db migrate` 和长期 Service 必须识别同一套应用配置节，避免“生成的配置自己不能启动”的漂移。

日志是开发必备能力。开发阶段默认可见 debug 级别日志，业务和基础设施代码必须遵守 [开发日志规范](docs/development/logging.md)，在真正决定处理策略的边界记录，避免泄露凭据或重复打印同一错误链。

## 文档权威边界

- 当前怎么启动、配置、开发和运维，以根 README 与 [项目手册](docs/README.md) 下的主题文档为准。
- `docs/changes/**` 保存任务级研究、计划、实施和验证证据，不替代当前主题文档。
- `docs/research/**` 保存阶段性研究快照，不把目标设计写成已经实现的能力。
- `pkg/**/README.md` 与 `internal/**/README.md` 是局部包说明，由主题文档链接进入，不作为全局阅读入口。
- 新增或修改能力时，先更新对应主题 authority；局部 README 只保留本包或本模块的实现边界和到 authority 的链接。

## License

Copyright 2026 Rin721.

This project is licensed under the Apache License 2.0.

You are free to use, modify, distribute, and use this project
for commercial purposes subject to the terms of the license.

See [LICENSE](./LICENSE) and [NOTICE](./NOTICE) for details.
