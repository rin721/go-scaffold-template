# 012 业务模块架构

## 状态

- 当前状态：**FND 与 GOV-F 已实施并完成本地门禁；业务详细设计继续被真实用例门禁阻塞**。
- 文档建立、契约校正与基础实施日期：2026-08-14。
- 方案代码基线：`main@2daf47ad111141b27a1d8e100bb3d6e4cc1ea743`；实施从分支提交 `7eaba7174fb54c06c8bd31c7f7bc345d3f161936` 开始。
- 用户已在方案报告后的独立消息中明确要求“开始实施 `012` 方案计划”，授权范围为 FND 与 GOV-F；没有授权虚构业务用例或进入 VSL。
- 实施快照、验证命令与剩余边界见 [R021](research/R021-foundation-closure-implementation/report.md)。

## 一句话结论

当前已形成从 mode-specific Bootstrap、严格配置、全应用单候选，到 Supervisor、HTTP lifecycle、readiness/degraded、重载/终止和 package graph 门禁的基础闭环。实现保留显式 Kernel Plan、stable facade、Lease、默认配置安全发布和候选事务，没有引入第二个 DI/生命周期容器，也没有把普通业务对象塞入 Kernel。基础门禁通过只允许重新讨论真实业务设计；因首个真实用例尚未确认，BIZ-D/VSL 仍保持阻塞。

## 实施判定

### 已满足或应保持

- 依赖来源与构造顺序可定位的显式 Plan，Freeze 后安装，没有扫描、Resolver 或 `init` 注册。
- Kernel 组件的 Stage/Build/Start/Ready/Publish、候选失败回滚、旧代排空和 stable facade/Lease。
- baseline Logger、配置候选预检、反序清理以及当前 Database/Cache/I18n/Storage 的项目契约边界。
- 显式标准流、typed CLI error、ordered defaults，以及默认配置全内存编码、no-overwrite/force 和临时文件清理。

### 已闭合的基础能力

- Bootstrap CLI 只构造六个配置节、DefaultManager 和 command tree；完整服务资源只在无参数 Service mode 构造。
- 同一配置节 registration 同时提供 defaults 与 strict typed validator；Source、重复字段、unknown/type、Snapshot 值域和默认生成回环均有失败测试。
- Coordinator 是 Loader 唯一调用者；Kernel 与 application-owned HTTP 从同一候选读取，RestartRequired 在资源副作用前预检。
- Supervisor 监督 blocking runner 的 ready、异常 error/nil 完成和不合作退出；终止先取消、反序 Stop，再在总期限内等待。
- HTTP Server 由单一 owner 预绑定并执行阻塞 Serve、有界 Shutdown/Close/Wait；默认 Service 有 listener，但没有业务路由。
- Host 接入 process readiness/liveness 与安全 diagnostics；committed cleanup failure 进入 degraded、要求重启并阻断后续 reload。
- 解析后的 Go package graph、注册冲突、生命周期、race 和静态检查进入可执行门禁。

### 继续阻塞

- 尚无用户确认的首个真实业务 actor、不变量、数据/事务 owner、入站协议和验收数据。
- 因此 Handler、Service、Repository、Model、业务 Route/Command contribution 与公开错误协议仍不得实施。

## 方案性质

原 012 的“Kernel 资源平面 + 手工业务对象图”方向继续保留；R021 记录 FND/GOV-F 的实现快照并替代实施前事实 R017 和方案综合结论 R020。历史记录继续保留，但当前行为以根主题文档和代码为准。Handler、Service、Repository、Model、Route contribution 仍只是不冻结接口的候选约束。

## 阅读顺序

1. [requirements.md](requirements.md)：本轮目标、范围、约束和业务延伸门禁。
2. [当前事实与缺口](requirements/current-facts-and-gaps.md)：配置到验证的逐段代码事实。
3. [底层契约清单](requirements/foundation-contract-catalog.md)：统一状态、语义 owner、调用方、输入输出、所有权与缺口。
4. [需求与验收矩阵](requirements/acceptance-matrix.md)：十一项门禁和基础闭环证据。
5. [design.md](design.md)：保留、补齐、拒绝和分阶段目标设计。
6. [CLI 与默认配置契约](design/cli-and-default-config-contracts.md)：mode、注册、I/O、退出、副作用与安全生成。
7. [Config 契约](design/config-contracts.md)：Source、Default、Binding、Validation、Snapshot 与 Reload。
8. [运行责任图与状态机](design/runtime-state-machine.md)：装配、启动、运行、重载、终止、错误和诊断。
9. [研究档案](research/README.md)：R001-R021 的版本、关系和证据。
10. [tasks.md](tasks.md)：实施顺序、完成状态、验证证据和业务解锁条件。

## 文档结构

```text
012-business-module-architecture/
├── README.md
├── requirements.md
├── requirements/
│   ├── current-facts-and-gaps.md
│   ├── foundation-contract-catalog.md
│   └── acceptance-matrix.md
├── design.md
├── design/
│   ├── foundation-closure.md
│   ├── cli-and-default-config-contracts.md
│   ├── config-contracts.md
│   ├── runtime-state-machine.md
│   ├── composition-and-lifecycle.md
│   ├── module-boundaries.md
│   ├── inbound-http-and-cli.md
│   ├── infrastructure-and-cross-module.md
│   ├── errors-observability-and-i18n.md
│   ├── module-development-guide.md
│   └── migration-risks-and-decisions.md
├── research/R001-R021/metadata.yaml + report.md
├── tasks.md
└── tasks/
    ├── foundation.md
    ├── first-vertical-slice.md
    └── governance-and-verification.md
```

## 交付边界

本轮没有为尚不存在的业务需求创造 `User`、`Order`、空 CRUD、Module SDK 或公开 contribution API。底层闭环已经通过；仍必须先获得首个真实业务用例、数据边界和入口验收，并重新确认业务方案，才能恢复业务模块详细设计。
