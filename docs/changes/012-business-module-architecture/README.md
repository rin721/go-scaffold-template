# 012 业务模块架构

## 状态

- 当前状态：**待确认**。
- 文档建立日期：2026-08-14。
- Git 事实基线：`main@2daf47a`，工作树在本任务开始前仅有未跟踪目录 `tmp/`。
- 方案编写轮次只完成代码取证、外部研究、需求、目标设计与任务拆分；没有修改源码、配置、脚本、依赖、测试或生成物，也没有启动进程。后续独立发布轮次仅获授权提交并 push 当前方案，不构成实施确认。
- 目标设计不代表当前实现。只有用户在本报告后的后续消息中明确确认 012，实施任务才能开始。

## 一句话结论

推荐在当前 Kernel 之上建立按业务能力纵向组织、由唯一 composition root 手工构造的模块化单体：Kernel Plan 继续只治理进程级底层资源，业务模块使用普通 Go 构造函数、使用方定义的窄接口和项目自有 Adapter；HTTP 与 CLI 是复用同一 Service 的入站边界，不引入运行时 DI、自动扫描、`init` 注册、Service Locator 或进程外插件协议。

## 研究目标

- 从当前代码而非蓝图确认已实现能力、依赖方向、启动/重载/停止链和业务缺口。
- 用官方源码、文档、示例与测试横向研究手工装配、代码生成、运行时 DI、HTTP 框架、模块化单体、编译平台、sidecar 和插件 host。
- 形成可检索的研究档案、适配当前 Kernel 的单轨目标设计、新模块黄金路径和具备依赖/完成条件的实施计划。

## 阅读顺序

1. [requirements.md](requirements.md)：目标、范围、关键场景和验收总览。
2. [当前事实与缺口](requirements/current-facts-and-gaps.md)：逐条区分已实现、未实现和推断。
3. [需求与验收矩阵](requirements/acceptance-matrix.md)：功能、质量、兼容与证据门槛。
4. [design.md](design.md)：决策摘要、边界和主题设计索引。
5. [研究档案](research/README.md)：研究规范、检索方法、代表项目报告和综合比较。
6. [tasks.md](tasks.md)：稳定任务 ID、依赖、工作量、确认门禁和实施批次。

## 文档结构

```text
012-business-module-architecture/
├── README.md
├── requirements.md
├── requirements/
│   ├── current-facts-and-gaps.md
│   └── acceptance-matrix.md
├── design.md
├── design/
│   ├── module-boundaries.md
│   ├── composition-and-lifecycle.md
│   ├── inbound-http-and-cli.md
│   ├── infrastructure-and-cross-module.md
│   ├── errors-observability-and-i18n.md
│   ├── module-development-guide.md
│   └── migration-risks-and-decisions.md
├── research/
│   ├── README.md
│   └── Rxxx-*/metadata.yaml + report.md
├── tasks.md
└── tasks/
    ├── foundation.md
    ├── first-vertical-slice.md
    └── governance-and-verification.md
```

## 交付边界

本任务不是业务需求实现，也不虚构 `User`、`Order` 等示例模块。首个垂直切片必须来自后续明确的真实业务场景；在此之前，目录、接口名和伪代码仅表达目标职责与不变量，不是已存在 API。
