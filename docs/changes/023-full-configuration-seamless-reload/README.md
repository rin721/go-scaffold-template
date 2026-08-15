# 023 全配置无感重载

## 当前状态

- 任务类型：历史研究与实施计划，已由 024 单轨吸收。
- 基线：`e251b73518a457ec97c529d067ddfffe77be203a`。
- 研究门禁：已通过，见 [R001](research/R001-current-reload-boundaries/report.md) 与 [R002](research/R002-generation-listener-patterns/report.md)。
- 计划状态：**已取代**；用户于 2026-08-15 确认 [024 生产就绪模板一次性竣工](../024-production-ready-one-shot-completion/README.md)，023 不再是并行施工 authority。
- 当前实现：023 的稳定读取、Application Generation、ListenerHub 与七节配置重载成果由 024 的 `ONE-004..007` 接收；当前行为以根 README 和 024 证据为准。

## 一句话结论

真正的“全配置无感重载”不能通过删除 `RestartRequired` 获得；必须单轨升级为不可变 Application Generation：同一稳定 Snapshot 构造完整 Capabilities、Todo、Router 与 `http.Server` 候选，进程级 ListenerHub 在一个提交点把新连接切到新代，在途请求固定旧代并排空。

## 目标范围

本任务覆盖 `config.example.yaml` 当前七个配置节：

- `logger`
- `database`
- `cache`
- `i18n`
- `storage`
- `http`
- `todo`

目标只针对长期 Service 模式。Application CLI 每次 invocation 都建立并释放自己的资源，本身没有运行中 watcher。

## “无感”的精确定义

1. 进程 PID 与 Host 不退出，配置 watcher 不需要重启。
2. 候选失败时当前 generation、listener、请求与资源不变。
3. 有效候选在单一线性化点提交；提交前接受的连接留在旧代，提交后接受的连接只进入新代。
4. 一次请求只使用一个 Snapshot 对应的 Logger、Database、Cache、I18n、Storage、Todo Policy、Router 与 HTTP transport generation。
5. 旧代停止接收新连接后完成 graceful drain，再按反向 owner 顺序释放资源。
6. 相同 HTTP 地址的配置变化不发生 close-then-bind 窗口；地址变化时新地址先成功绑定，旧地址上已接受的请求继续完成。
7. 稳定非法配置或候选 Ready 失败保留旧代；提交后的 cleanup debt 明确进入 degraded，不能伪装回滚。

“无感”不表示外部客户端会自动从已删除旧地址迁移到新地址，也不表示切换 DSN/bucket 会自动迁移业务数据。

## 阅读顺序

1. [研究索引](research/README.md)
2. [R001 当前边界](research/R001-current-reload-boundaries/report.md)
3. [R002 目标模式](research/R002-generation-listener-patterns/report.md)
4. [需求](requirements.md)
5. [ADR-002](decision.md)
6. [设计](design.md)
7. [任务](tasks.md)

## Authority 说明

本目录只保留 023 的研究、需求、设计和任务历史，不再授权独立实施。后续扩展、验证、提交和竣工标签统一由 024 管理；外部 push、tag、Release、GHCR 与 attestation 仍未授权。
