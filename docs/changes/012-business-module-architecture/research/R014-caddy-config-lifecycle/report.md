# R014：Caddy 配置生命周期

## 1. 当前问题

Kernel 已采用候选构造、Ready、排空、提交和旧代清理。需要判断这一方向是否合理，以及应用级配置是否应扩大为动态对象图。

## 2. 外部事实

Caddy 的架构文档把模块生命周期表达为 load、provision、validate、use、cleanup；配置 API 在新配置成功装载后切换，失败保持旧配置。扩展文档明确新旧实例可在配置变化时短暂重叠，因此资源必须有确定 cleanup，且 provision 不应进行无边界昂贵工作。

来源：[Caddy Architecture](https://caddyserver.com/docs/architecture)、[Caddy API](https://caddyserver.com/docs/api)、[Caddy v2.11.2 源码](https://github.com/caddyserver/caddy/tree/v2.11.2)。

## 3. 与当前实现比较

| 维度 | 当前 Kernel | Caddy 参照 | 结论 |
|---|---|---|---|
| 候选验证 | Stage/Build/Start/Ready | load/provision/validate | 方向一致，保留 |
| 旧代可用 | 候选失败保留旧 Generation | 新配置失败保留旧配置 | 方向一致，保留 |
| 新旧重叠 | prepare 时旧代仍可借用 | reload 可重叠实例 | 方向一致，需持续验证 owner |
| 动态范围 | 仅明确 Kernel components | 可动态加载广泛模块/配置 | 本项目不应扩大到业务对象图 |
| 清理失败 | 返回 committed cleanup error，但无持久 degraded | cleanup 是模块生命周期责任 | 本项目需要补诊断和后续策略 |

## 4. 可选方案

- 保持当前三个策略（Direct、KernelInstanceSwap、RestartRequired）并补全进程协调：收益最大、迁移最小，推荐。
- 实现通用 NativeAtomicReload/ComponentHandoff/自动观测回滚：能覆盖未来复杂资源，但当前没有真实组件需求，违反目标和复杂度门禁。
- 关闭现有 reload，统一要求重启：模型更小，但会丢弃已经实现和验证的稳定 facade/候选事务，收益不足。
- 把 Caddy module registry/动态路由直接搬入：会引入全局注册和隐藏发现，与仓库红线冲突。

## 5. 推荐与验证

保留 Kernel 当前换代方向；application coordinator 先对整份候选执行所有 owner 的解码、校验和 RestartRequired 预检，再让 Kernel 使用同一候选。业务对象图、路由、listener 初版不可热替换。

提交后旧代 cleanup 失败必须持久化为 degraded/restart-required 诊断，并在问题被明确处理前阻断后续 reload，避免无记录地累积旧资源；这不是自动回滚，因为新代已经对外服务。正常 reload 的排空失败可以回滚，进程终止排空则不得 Resume 接单。

证据强度：方向为高（本地测试与外部成熟实现一致）；具体 cleanup 重试策略为中（不同资源 Close 是否可重试不同）。实施必须用每代只关闭一次、失败保留旧代、committed cleanup degraded、后续 reload 阻断和整份 snapshot digest 测试验证。
