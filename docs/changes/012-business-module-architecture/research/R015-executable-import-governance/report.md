# R015：可执行依赖治理

## 1. 当前问题

012 已写出“domain/application 不导入 Kernel/HTTP/第三方”和“模块不穿透 Adapter”等规则，但当前只有文档与人工搜索。需要确认 Go 自身能保证到什么程度，以及成熟仓库如何补齐。

## 2. 外部事实

Go 官方 module layout 说明 `internal` 只阻止父目录树之外的代码导入；同一 module 内的其他包仍可导入，所以它不能表达 application → adapter 禁止、模块 A → 模块 B adapter 禁止等方向规则。

Kubernetes `import-boss` 用声明式允许/禁止列表检查 package graph，可覆盖直接、传递和 inverse restriction。关键启示不是必须采用该工具，而是架构边界需要对解析后的 import graph 执行，并同时有合法/违规样例验证规则本身。

来源：[Go module layout](https://go.dev/doc/modules/layout)、[Kubernetes import-boss](https://github.com/kubernetes/kubernetes/blob/v1.34.0/cmd/import-boss/README.md)。

## 3. 方案比较

| 方案 | 收益 | 代价/风险 | 判定 |
|---|---|---|---|
| 只靠文档和 code review | 无工具成本 | 会随人员/路径漂移，无法成为验收证据 | 不足 |
| 只靠 `internal` | Go 原生 | 不表达 module 内方向和第三方边界 | 不足 |
| 直接引入 import-boss | 规则能力成熟 | 新工具/依赖和仓库约定成本，当前规模可能过大 | 暂不推荐 |
| 用标准 Go package graph 建小型架构测试 | 无生产依赖，规则贴合本仓库 | 需维护清晰 allow/deny 和 fixtures | 推荐 |

## 4. 推荐治理闭环

- 基于 `go list -deps -json`、`go/packages` 或标准 parser 读取真实 import，而非 grep。
- 规则按语义 owner 集中声明：底层 Kernel 边界、application/domain 禁止项、模块间 adapter 穿透、composition 唯一构造位置。
- 每条规则至少一个故意违规 fixture/测试证明会失败，一个合法例子防止过度约束。
- contribution/Participant/Task ID 的非空、唯一、确定顺序和相同规范化函数也由测试约束。
- 文档只解释原因和迁移方式，测试/静态检查提供持续证据；两者不互相替代。

证据强度：高。是否采用 `go/packages` 仍需实施时按现有依赖和测试成本选择，不因此预先新增第三方工具。业务包尚未存在时先固化 Kernel/composition/Supervisor 的现有规则；业务边界门禁在真实路径确定后扩展，避免为虚构目录写正则。
