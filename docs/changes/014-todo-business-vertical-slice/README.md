# 014 Todo 业务垂直切片

## 状态

- 任务性质：非纯文档业务实现。
- 当前状态：**已完成实现与本地验证**。
- 研究与计划日期：2026-08-15。
- 代码基线：`abca7f44cf9c35ec60796fec9680964e9cc4298e`。
- 当前授权：用户已在计划报告后的独立消息中明确要求“执行014方案”，授权实施当前 requirements/design/tasks 中的 BUS、DB、CFG、HTTP、CLI、CMP、GOV 与 VER 任务。

## 一句话结论

Todo 将作为仓库首个真实业务垂直切片，采用模块内 `model/service/repo/handler/binding` 分层，以同一个 Service 同时提供 REST API 与 Application CLI，并用现有 Database Access 持久化到 SQLite。实施不新增第三方依赖、不把业务对象放入 Kernel Plan，也不引入扫描、Service Locator 或第二套生命周期容器。

## 目标结果

- `POST/GET/PATCH /api/v1/todos...` 可创建、查询、分页列表和完成 Todo。
- `todo create/get/list/complete` 命令执行同一批用例，并在 one-shot 生命周期内启动、迁移和停止所需资源。
- `todo` 配置节进入 strict binding、默认配置生成和 reload restart preflight。
- Todo Schema 通过项目 `pkg/database` additive migration 创建，默认 SQLite 数据可跨 CLI 与 HTTP 进程保留。
- 新模块目录、route contribution、错误映射和架构测试可作为后续业务模块的真实学习样板。

## 阅读顺序

1. [研究档案](research/README.md)：当前能力、缺口与可行性证据。
2. [requirements.md](requirements.md)：业务规则、HTTP/CLI/配置契约与验收标准。
3. [design.md](design.md)：目录、依赖、数据流、生命周期、失败语义和文件影响。
4. [tasks.md](tasks.md)：稳定任务 ID、依赖、实施顺序和验证门禁。

## 确认边界

用户已在计划报告后的独立消息中确认当前计划；本次实现只覆盖已确认任务。后续若修改公开 API、配置键、依赖选择、模块边界、迁移语义或外部副作用，必须建立新变更并重新确认。
