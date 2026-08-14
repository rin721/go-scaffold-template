# R010：综合比较与结论

> 状态：先由 R016 校正，现行综合结论已由 [R020](../R020-foundation-contract-synthesis/report.md) 单轨替代。双平面和显式装配方向仍被保留，但“可以直接进入业务模块设计”的时机判断已失效；当前必须先完成底层契约门禁。

## 1. 比较问题

不做“最流行框架”排名，而是用当前仓库约束比较：依赖可见性、构造失败时机、生命周期、配置一致性、业务边界、开发体验、动态能力成本和迁移规模。

## 2. 比较矩阵

| 样本 | 图形成方式 | 生命周期 owner | 主要收益 | 当前主要冲突 | 012 处理 |
|---|---|---|---|---|---|
| 当前 Kernel | 显式 Plan + manual install | Kernel/Host | 配置候选、资源 reload、稳定 facade | 没有业务图/HTTP listener | 保留为资源平面 |
| Kratos/Wire | 生成静态构造链 | App/cleanup | 图可读、分层清晰 | Wire 归档、目录偏单服务 | 吸收结构，手工实现 |
| go-zero | 生成 main/Handler/Logic/context | Server/main | 开发路径统一 | 巨型 context、工具链重叠 | 吸收黄金路径，不用 context |
| Uber Fx | runtime 类型图 | Fx Lifecycle | 大图模块化、Hook 成熟 | 第二容器、图隐式、group 无序 | 不引入，吸收反向停止 |
| Hertz | 显式/生成路由 + Server | Spin | HTTP 生态、shutdown | 全局 DAL/信号 owner 与 Host 冲突 | 保留 httpx，建立 Participant |
| Wild Workouts | manual ports/adapters | main/adapters | caller-owned port、事务闭包 | 教学样本非框架 | 作为业务内层主参考 |
| Encore | compiler 派生服务/资源图 | 平台 runtime | infra 与观测自动化 | 专用工具链、包级资源、Kernel 重叠 | 当前拒绝 |
| Dapr | sidecar + 外部 component | 分布式 runtime | 跨语言 building blocks | 网络/部署/一致性成本 | 仅未来远程边界候选 |
| go-plugin/Mattermost | 子进程 RPC + host API | plugin host | 第三方扩展和隔离 | 协议、安全、版本治理 | 当前拒绝 |

## 3. 综合推导

### 3.1 为什么不是“所有对象都进 Kernel”

Kernel 的强项是底层资源候选配置、生命周期、reload 和 stable facade。普通 Service/Handler/Repository 没有独立资源生命周期，把它们加入 Plan 会把业务对象图绑定到 Kernel Component 语义，并诱导 runtime 查找。Kratos/Wild Workouts 表明普通构造链足以表达这部分。

### 3.2 为什么不是第二个 DI 容器

Fx 能管理大图，但当前图尚不存在规模证据，而且会与 Kernel/Host 重叠。手工 root + 模块局部装配能在编译器和 code review 中直接展示依赖。typed contribution 仅解决多模块完成品合并，不承担依赖解析。

### 3.3 为什么需要先改配置和 HTTP lifecycle

本地事实表明 Kernel 内部加载快照，HTTP Start 又阻塞并晚报监听错误。任何业务目录设计若绕过这两点，都会留下启动撕裂或 Host 假成功。故基础任务必须先于首个切片。

### 3.4 为什么当前只做模块化单体

Dapr 和 go-plugin 展示了远程/插件的真实成本：协议、网络、身份、版本、部署、监管与观测。当前同仓同进程业务没有这些需求，caller-owned Go port 是更小且可验证的边界。Encore 进一步证明自动发现若要可靠，背后需要完整编译平台，而非简单扫描。

## 4. 推荐目标

1. Kernel Plan 只保留底层 App resource。
2. application config coordinator 一次加载不可变快照并同时供应 Kernel 与业务 composition。
3. 唯一 composition root 手工选择模块；模块内部纯构造。
4. 按业务能力纵向组织 domain/application/adapters。
5. caller-owned Repository/跨模块 port，第三方类型限制在 Adapter。
6. module contribution 只包含已绑定 Route/Command/Participant/Cleanup，并集中校验。
7. Host 顺序为 Kernel、module、HTTP/application command，停止反向。
8. HTTP/CLI 共用 Service；事务属于用例，I18n 属于呈现边界。
9. 首版对象图与路由不可热重建，相关配置变化返回 `RestartRequired`。
10. 不实现 runtime DI、扫描、插件、sidecar、消息总线或假业务。

## 5. 采用度分级

- **直接采用原则**：显式构造、caller-owned port、domain/record/DTO 分离、正序启动/反序停止、Handler/Service 分离。
- **适配后采用**：Kratos cleanup、Fx Hook、Hertz shutdown、Wild Workouts transaction closure；均需落到现有 Kernel/Host/Access 语义。
- **观察但不落地**：go-zero 生成路径、Encore 编译分析；待两个真实模块出现重复成本后复评。
- **当前拒绝**：runtime DI、全局 ServiceContext、全局 DAL、Dapr sidecar、进程外插件、自动扫描。

## 6. 决策置信度与缺口

对双平面、手工 composition、纵向模块和显式 port 的置信度高：它们同时得到本地代码与多组源码样本支持。对具体 contribution API、I18n 资源聚合、首个 UnitOfWork 形状和 HTTP 公共错误协议仅有方向性结论，必须由真实业务用例确认。

首个垂直切片完成后应刷新 R001/R010，比较设计与真实实现，删除不再成立的假设，并把已验证结论同步到当前主题文档。
