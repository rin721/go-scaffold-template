# 设计：用外部消费者证据选择脚手架产品形态

## 1. 设计原则

- **先证明消费链，再改变生产边界。** 当前 `internal` 和固定身份事实见 [R001](research/R001-current-distribution-boundary/report.md)。
- **创建与升级是两个协议。** 模板复制解决首次创建；Go module 才天然表达依赖版本，见 [R002](research/R002-go-distribution-versioning/report.md)。
- **应用拥有业务与组合。** 脚手架不能在升级时重写业务模块、路由选择或资源组合。
- **公共 API 面积是成本。** 只有跨服务稳定且值得兼容承诺的 Runtime 契约才可从 `internal` 演进为公共包。

## 2. 候选及当前假设

| 候选 | 决策前提 | 当前判断 |
| --- | --- | --- |
| template repository | 接受复制后不自动升级，只保证首次创建 | 可作为最低成本 v0 退路，但当前固定身份仍需解决 |
| generator-only | 需要身份 schema、原子输出、所有权和模板迁移 | 能解决创建一致性，Runtime 修复传播弱 |
| library-only | Runtime API 足够完整且应用胶水简单 | 当前不可行；完整 Runtime 在 `internal`，且会丢失脚手架基线 |
| Runtime + generator | Runtime 能保持窄，应用组合可由模板拥有 | 优先验证假设；升级语义最好，但治理成本最高 |

不采用加权打分制造伪精度。以下硬门禁任一失败，组合模式即失败：

1. 外部消费者必须导入大量具体 Adapter 或源仓库 `internal`；
2. 应用无法在自己仓库拥有 Composition/Module；
3. Runtime 升级必须重写用户代码；
4. generator 无法在冲突时安全停止；
5. 三种版本无法独立表达和追溯。

## 3. 隔离验证拓扑

验证获确认后只在 Git 忽略的 `tmp/scaffold-product-form/` 中工作，不修改当前生产 Go 源码：

```text
tmp/scaffold-product-form/
├── source-snapshot/       # 当前仓库的可丢弃快照，允许原型性边界调整
├── generated-service/     # 独立 module，模拟首次生成和业务定制
├── upgrade-vnext/         # Runtime/template 下一版本输入
└── evidence/              # 命令、清单、diff 摘要，不包含凭据和机器私有信息
```

`generated-service` 必须是独立 `go.mod`。未发布 Runtime 可使用 Go 官方建议的本地 `replace` 指向 `source-snapshot`，而不能指向当前工作树中未经验证的公共实现。

## 4. 三组验证

### 4.1 `PROBE-001`：template/generator 创建

输入一个非默认 module、应用名、可执行名、配置名、环境前缀和 `example=todo|none`，验证：

- 输出目录原子创建，非法输入不留半成品；
- Go imports、文档、配置和进程帮助中的身份一致；
- 产物不残留不应出现的 `go-scaffold2`/`APP_`；
- 相同输入重复生成的 tracked 内容一致；
- manifest 列出每个文件的 owner 和 template schema version。

这里允许一次性验证脚本或最小原型，但不得放入正式源码目录。

### 4.2 `PROBE-002`：library 边界

先用负向编译证明外部 module 不能导入当前 `internal/kernel`，再在 `source-snapshot` 中探索最小公共 Runtime seam。记录外部消费者真正需要的符号、第三方类型泄漏、配置/生命周期 owner 和 API 数量；不以“能编译”替代边界质量。

### 4.3 `PROBE-003`：组合模式升级

在消费者中加入 application-owned sentinel 和一个最小业务改动，然后模拟：

1. Runtime `vA -> vB`：只改变 module 依赖，消费者代码保持兼容；
2. template schema `sA -> sB`：只修改 generator-owned 文件或生成迁移建议；
3. 同一路径冲突：停止、报告，sentinel 内容不变；
4. Todo -> none 或新项目 none：验证示例与核心的边界，阻塞点如实进入 ADR。

## 5. 所有权模型

| Owner | 示例 | 演进方式 |
| --- | --- | --- |
| Runtime module | 稳定 Host/lifecycle 契约 | Go module 版本升级；消费者不复制源码 |
| Generator | 可完全再生且无用户编辑的薄入口/元数据 | schema migration；校验 hash 后替换 |
| Application | Composition、Module、业务、配置值和部署定制 | 永不静默覆盖；只给迁移建议或显式 patch |

具体文件归类只有实验后才能写入 ADR；上表是验证模型，不是已确认目录变更。

## 6. 决策输出

`ADR-001` 必须记录：

- 选择的唯一产品形态和被拒绝候选；
- Runtime、generator 和 application 三个边界；
- Runtime/generator/template schema 的版本和兼容语义；
- 创建、升级、冲突、回滚和停止策略；
- v0 限制、进入 v1 的证据门禁；
- 触发重新评估的条件。

若组合模式未通过硬门禁，ADR 应选择 generator/template-only，并直说“创建后由应用完全拥有，框架修复不会自动传播”。不得同时保留两个权威创建入口。

## 7. 后续影响

020 决策后，至少拆分独立实施变更：公共 Runtime（如被选择）、正式 generator/template、release/version、外部 consumer CI 和开发者文档。019 的 `API-AUTHORITY-001` 只有在产品边界确定后才能准确决定 API contract 位于 Runtime 还是应用模板。
