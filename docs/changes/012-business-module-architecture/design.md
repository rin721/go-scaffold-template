# 目标设计：业务模块架构

## 1. 文档性质

本文件描述 **待确认的目标设计**，不是当前实现说明。当前代码事实见 [current-facts-and-gaps.md](requirements/current-facts-and-gaps.md)，外部证据见 [research/README.md](research/README.md)。后续只有在用户明确确认本方案及对应任务 ID 后，才允许修改实现。

## 2. 核心决策

目标架构由两个职责不同、不可混用的平面组成：

1. **Kernel 资源平面**：继续使用现有显式 `Plan` 管理依赖配置、生命周期和重载的进程级底层资源，例如 Logger、Database、Cache、I18n 与 Storage。
2. **业务对象图平面**：在唯一 application composition root 中用普通 Go 构造函数静态装配 Module、Service、Repository Adapter、Handler 与 Command。

业务对象图不进入 Kernel `Plan`，也不新增运行时 DI、反射扫描、`init` 注册、Service Locator、全局可变 Registry 或动态插件协议。选择这种双平面不是因为对象属于“基础设施”或“业务”的抽象标签，而是依据它是否需要 Kernel 当前提供的候选配置事务、资源生命周期和稳定 facade。

## 3. 目标运行结构

```text
cmd/app
  └─ application composition root
       ├─ config coordinator（唯一加载者）
       ├─ Kernel Plan（底层资源）
       ├─ business modules（普通构造函数）
       │    ├─ domain
       │    ├─ application service + caller-owned ports
       │    └─ adapters: database / http / cli / cache
       ├─ typed contributions validator
       └─ Host
            ├─ Kernel participant
            ├─ module participants
            └─ HTTP participant（最后启动、最先停止）
```

依赖方向固定为：

```text
cmd -> composition -> module outer assembly/adapters -> application -> domain
```

`domain` 和 `application` 不导入 Kernel、HTTP、CLI、GORM、Redis、Cobra、Chi 或其他第三方基础设施类型。Adapter 向内依赖业务契约；composition root 只负责选择实现并连接对象，不承载业务规则。

## 4. 关键不变量

- 每个业务能力只有一个语义所有者；顶层按业务能力纵向组织，不建立全局横向 `handlers`、`services`、`models`、`repositories`。
- 业务模块的依赖在构造阶段完整声明，运行期间不按名字查找对象。
- 路由、命令和生命周期项是已经绑定好依赖的 typed contribution，不是 Provider 或依赖声明语言。
- 初始启动使用一份不可变配置快照；影响业务对象图、路由或监听器的配置变化要求重启。
- 构造阶段不执行 I/O，不使用尚未启动的 Capability；资源探测和后台任务由有 owner 的 Participant 承担。
- Kernel 先启动、模块 Participant 后启动、HTTP 最后启动；停止严格反向。
- Repository、事务和 Cache 不能突破当前 Lease/回调边界；第三方对象不能泄漏到业务层。
- 错误在业务层保持稳定分类和原因链，只在 HTTP/CLI 呈现边界映射状态与本地化文本。
- 当前只支持编译期选择的进程内模块；真实需求出现前不建设消息总线、远程模块或进程外插件。

## 5. 主题设计

- [模块边界与依赖规则](design/module-boundaries.md)
- [装配、配置与生命周期](design/composition-and-lifecycle.md)
- [HTTP 与 CLI 入站边界](design/inbound-http-and-cli.md)
- [数据库、事务、缓存与跨模块协作](design/infrastructure-and-cross-module.md)
- [错误、日志、可观测性与 I18n](design/errors-observability-and-i18n.md)
- [新模块开发黄金路径](design/module-development-guide.md)
- [迁移、风险、决策与未决项](design/migration-risks-and-decisions.md)

## 6. 方案来源与取舍

本设计综合了本仓库事实与九组外部样本，而非照搬某个框架：

- [R001 当前仓库事实](research/R001-current-project-facts/report.md) 证明 Kernel 资源平面可复用，但业务对象图、统一快照和 HTTP Participant 仍是缺口。
- [R002 Kratos/Wire](research/R002-kratos-wire/report.md) 证明静态 composition root、业务分层和清理函数可以自然协作；但 Wire 已归档，目标方案保留手工构造，不引入生成依赖。
- [R003 go-zero](research/R003-go-zero/report.md) 证明生成式 Handler/Logic/ServiceContext 路径易于规模化；但巨型 `ServiceContext` 会扩大依赖面，目标方案只接受最小构造依赖。
- [R004 Uber Fx](research/R004-uber-fx/report.md) 展示了大对象图、Module 和有序生命周期的成熟实现；但运行时类型图和 value group 的隐式性与当前显式装配约束冲突。
- [R005 Hertz](research/R005-cloudwego-hertz/report.md) 展示了 HTTP 生态和生成路由；但全局初始化与监听错误语义不能直接用于本仓库。
- [R006 Wild Workouts](research/R006-wild-workouts/report.md) 提供了使用方定义 Repository、显式事务闭包和 Adapter 隔离的可迁移经验。
- [R007 Encore](research/R007-encore/report.md)、[R008 Dapr](research/R008-dapr/report.md) 和 [R009 go-plugin/Mattermost](research/R009-hashicorp-go-plugin/report.md) 分别代表编译器治理、分布式 sidecar 与进程外插件；它们解决的是更高运维或扩展需求，当前全部作为边界参照而不是落地依赖。

横向推导和采用度见 [R010 综合比较](research/R010-comparative-synthesis/report.md)，完整版本快照和检索规则见 [研究索引](research/README.md)。

## 7. 实施前置条件

方案确认仍不足以虚构首个业务模块。实施前至少要明确：首个真实用例及业务所有者、是否需要 HTTP/CLI、数据与事务边界、缓存必要性、配置节与 I18n 资源方式。涉及公共接口、依赖、模块边界或配置迁移的实质变化必须重新回到待确认状态。
