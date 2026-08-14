# 012 业务模块架构

## 状态

- 当前状态：**待确认；业务详细设计被底层闭环门禁阻塞**。
- 文档建立日期：2026-08-14；本轮底层复核日期：2026-08-14。
- 代码事实基线：`main@2daf47ad111141b27a1d8e100bb3d6e4cc1ea743`；本轮开始时当前分支除既有未跟踪 `tmp/` 外无新工作区改动，`tmp/` 未触碰。
- 研究阶段只补充 012 研究与方案文档，没有修改源码、配置、依赖、脚本或测试，也没有启动或生成；本报告后的独立发布仅获授权提交并 push 这些文档，不构成基础实施确认。
- 目标设计不代表当前实现。只有用户在本报告之后明确确认本轮基础闭环方案及任务范围，才允许实施。

## 一句话结论

当前 **Kernel 资源装配与候选换代已形成局部闭环，但全进程装配和治理尚未闭环，暂不足以支撑业务模块详细设计**。推荐保留显式 Kernel Plan、stable facade、Lease 和候选配置事务，在其上补齐 application 配置协调、Supervisor 运行监督、HTTP lifecycle、readiness/degraded 诊断与可执行治理；不引入第二个 DI/生命周期容器，也不把普通业务对象塞入 Kernel。

## 本轮判定

### 已满足或应保持

- 依赖来源与构造顺序可定位的显式 Plan，Freeze 后安装，没有扫描、Resolver 或 `init` 注册。
- Kernel 组件的 Stage/Build/Start/Ready/Publish、候选失败回滚、旧代排空和 stable facade/Lease。
- baseline Logger、配置候选预检、反序清理以及当前 Database/Cache/I18n/Storage 的项目契约边界。

### 尚未闭合

- Loader 只属于 Kernel，尚无整份 application snapshot 的唯一协调者。
- Supervisor 先等待 Task 再 Stop Participant，无法安全组合需由 Stop 触发退出的 HTTP Serve；Participant 也没有运行期错误回传。
- 关键 Task 提前返回 nil、忽略 context、终止排空超时和 committed cleanup error 都没有完整进程状态与后续策略。
- Kernel `Ready` 只是候选发布门禁；生产 readiness/liveness、generation、last reload/cleanup 和 degraded 状态未接入。
- `pkg/health` 是未接入生产的原语；HTTP listener、真实业务图和 application composition 仍不存在。
- 边界和唯一装配位置尚未由 package graph、注册冲突和全链路生命周期测试持续约束。

## 方案性质

本轮对原 012 做单轨校正：原有“Kernel 资源平面 + 手工业务对象图”的方向继续保留，但 Handler、Service、Repository、Model、Route contribution 等细节只作为 **底层门禁通过后的候选约束**，当前不得实施或继续冻结公共接口。R001/R010 已由更完整的 R011/R016 替代，避免两套现行结论并存。

## 阅读顺序

1. [requirements.md](requirements.md)：本轮目标、范围、约束和业务延伸门禁。
2. [当前事实与缺口](requirements/current-facts-and-gaps.md)：配置到验证的逐段代码事实。
3. [需求与验收矩阵](requirements/acceptance-matrix.md)：十一项门禁和基础闭环证据。
4. [design.md](design.md)：保留、补齐、拒绝和分阶段目标设计。
5. [底层闭环设计](design/foundation-closure.md)：状态、数据流、失败语义和候选比较。
6. [研究档案](research/README.md)：R001-R016 的版本、关系和证据。
7. [tasks.md](tasks.md)：实施顺序、确认状态、风险和业务解锁条件。

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
│   ├── foundation-closure.md
│   ├── composition-and-lifecycle.md
│   ├── module-boundaries.md
│   ├── inbound-http-and-cli.md
│   ├── infrastructure-and-cross-module.md
│   ├── errors-observability-and-i18n.md
│   ├── module-development-guide.md
│   └── migration-risks-and-decisions.md
├── research/R001-R016/metadata.yaml + report.md
├── tasks.md
└── tasks/
    ├── foundation.md
    ├── first-vertical-slice.md
    └── governance-and-verification.md
```

## 交付边界

本轮没有为尚不存在的业务需求创造 `User`、`Order`、空 CRUD、Module SDK 或公开 contribution API。底层闭环通过后，还必须先获得首个真实业务用例、数据边界和入口验收，才能恢复业务模块详细设计。
