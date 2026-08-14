# R002：Go module、模板与版本升级语义

## 1. 研究问题

Go 官方的包可见性、module 发布、语义版本和项目模板机制，如何约束一个成熟 Go 脚手架的创建与升级模型？

## 2. 官方事实

### 2.1 `internal` 是编译期边界

本机 `go help gopath` 的 `Internal Directories` 规则规定，`internal` 及其下代码只允许由其父目录树内的代码导入。这是 Go tool 强制的可见性约束，不是文档约定。因此外部服务复用当前 `internal/kernel` 不可行。

### 2.2 Go module 是依赖版本通道

[Go module 开发与发布](https://go.dev/doc/modules/developing)说明：module path 和 repository tag 用于定位版本；语义版本向消费者传递稳定性和兼容预期；公共 API 应聚焦并以兼容升级为设计约束。

[官方 release workflow](https://go.dev/doc/modules/release-workflow)进一步说明：未发布 module 可由外部 client 的 `replace` 指向本地目录进行验证；v0 不保证稳定，v1 开始承诺同一 major 内兼容。这支持“先用隔离消费者验证窄公共 Runtime，再决定 v0 发布边界”。

### 2.3 模板解决实例化，不天然解决持续升级

[GitHub template repository](https://docs.github.com/en/repositories/creating-and-managing-repositories/creating-a-template-repository)会复制目录和文件，但生成仓库与模板仓库具有无关历史，不能通过两者间的 PR 或 merge 作为标准升级通道。

[Go `gonew` 实验](https://go.dev/blog/gonew)证明 Go module 可以承载模板，并能用目标 module name 实例化项目；同一官方文章也明确它是刻意最小、能力有限的实验性工具。因此可以借鉴“版本化模板输入 + 目标 module 参数”，但 020 不应把 `gonew` 设为不可替换的核心依赖。

## 3. 四种形态比较

| 形态 | 首次创建 | 依赖升级 | 用户代码所有权 | 公共 API 成本 | 主要风险 |
| --- | --- | --- | --- | --- | --- |
| template repository | 低成本复制 | 无内建通道 | 全部归应用 | 低 | 固定身份、长期漂移、升级靠手工 |
| generator-only | 可参数化和校验 | 生成器升级不等于应用升级 | 生成后归应用 | 低 | 重生成覆盖、模板迁移复杂 |
| library-only | 需手写应用胶水 | Go module 原生支持 | 业务归应用 | 高 | 过大 API、框架耦合、无法生成一致基线 |
| Runtime + generator | 创建与依赖升级分离 | Runtime 走 module，模板走 schema migration | Composition/Module 归应用 | 中到高且可收敛 | 双版本治理、边界设计成本最高 |

## 4. 推断与决策约束

- 成熟脚手架必须把“可升级依赖”和“一次性/可迁移生成资产”分开；不能用 Git merge 假装升级协议。
- 公共 Runtime 只应包含跨服务稳定、能够单独测试并值得兼容承诺的生命周期/宿主契约；业务模块、路由选择和资源组合应由生成后的应用拥有。
- generator 必须声明输入 schema、输出清单、文件所有者和冲突策略。默认策略应是拒绝覆盖用户拥有文件，而不是按路径猜测。
- Runtime version、template schema version 和 generator version 是三个不同语义；若组合模式成立，必须在生成元数据中记录它们。
- 若最小公共 Runtime 仍迫使应用依赖大量具体实现，组合模式应判定失败，v0 选择 generator/template-only 更诚实。

## 5. 局限

官方资料定义通用语义，不替项目选择 generator 技术、公共 API 或发布节奏。组合模式的收益必须由 020 的外部消费者实验验证。

## 6. 对 020 的影响

020 的非文档阶段应先做可丢弃原型，再写 ADR。不得先发布 tag、移动生产包或承诺 v1；本任务的目标是选择产品边界，不是交付生成器。
