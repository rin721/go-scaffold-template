# R002：Go module、模板与版本升级语义

> 后续决策：用户于 2026-08-15 排除 generator 和外部 Runtime 依赖，选择完整源码复制。下列比较作为研究历史保留，当前方案以 copy-owned template 为唯一权威。

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

## 4. 推断与当前决策约束

- 完整复制后 `internal` 位于目标 module 自己的目录树内，因此不需要把 Kernel 契约改成公共依赖。
- Git tag/release 可以标识复制 baseline，但不能被解释成已复制项目可执行 `go get` 的整体升级版本。
- GitHub template 产生无关历史，进一步证明上游 merge 不能成为默认升级协议。
- 全部复制文件归新项目；上游只能发布 release note、安全公告和人工迁移指南，不能覆盖副本。
- generator、公共 Runtime 和组合模式已被用户排除，不再需要设计其版本或所有权模型。

## 5. 局限

官方资料定义通用语义，不替项目决定具体复制命令、baseline 发布节奏或迁移公告格式。这些仍需由 020 的隔离副本实验验证。

## 6. 对 020 的影响

020 的非文档阶段应验证可丢弃副本、身份迁移、Todo 保留/移除和来源记录，再固化决策结果。不得先发布 tag 或修改生产包；本任务验证复制模型，不交付 generator。
