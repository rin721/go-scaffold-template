# R002 oapi-codegen v2.8.0 路由分区能力与方案比较

## 1. 研究问题

当前仓库固定使用 `oapi-codegen v2.8.0`、OpenAPI 3.0.3、strict server 和 Chi server。本报告核对该版本能否按 tag/operation/spec 分区，并比较三种多模块装配方式，避免把工具能力、当前选择和目标实现混写。

## 2. 官方事实

按 v2.8.0 官方文档与源码：

- `strict-server` 与某一种具体 server generator 一起生成，当前仓库选择 `chi-server`。
- `output-options` 提供 `include-tags`、`exclude-tags`、`include-operation-ids` 与 `exclude-operation-ids`，可以限制一次生成覆盖的 operation 集合。
- `import-mapping` 支持把拆分 OpenAPI 文件的外部引用映射到 Go package；官方也提供同一 Go package 多次生成的示例。
- 官方倾向生成可读的直接代码，并明确生成物应被审阅；工具没有要求应用必须采用运行时 Route Registry。

这些事实证明“按模块生成”技术上可行，但不证明它对当前仓库最简单。

## 3. 当前仓库约束

- 只有一份 `api/openapi.yaml`，当前只有 Todo tag 与四个 operation。
- `api/oapi-codegen.yaml` 一次生成 models、strict server、Chi routes 和 embedded spec，并使用 `skip-prune: true`。
- operation inventory 也从同一规范生成，并承担授权、日志、trace 与 metrics 的稳定 operation identity。
- 025 已确定公开 OpenAPI 与纯生成代码属于应用级 authority，业务手写 Adapter 属于模块。

## 4. 方案比较

| 方案 | 优点 | 成本与风险 | 结论 |
| --- | --- | --- | --- |
| 单一生成 binding + 应用静态 aggregate | 单一规范、单一路由表、单次 validator；模块只实现自己的窄 operation；编译期完整性强 | application aggregate 有少量显式转发代码 | 当前采用 |
| 按 tag/operation 生成多个 server binding | 每个生成接口天然按模块裁剪 | 需要多份 generator 配置/生成文件，处理共享 models、embedded spec、命名和 import mapping；仍需统一全局 middleware 与错误语义 | 达到规模触发器后再评估 |
| 模块贡献运行时 Route Registry 或手写 method/path | 新模块表面上只注册自己 | 重复 OpenAPI 路径事实、引入冲突/顺序/缺失校验和动态失败，削弱编译期完整性 | 拒绝 |
| 每个模块生成并挂载整份 spec | 复制当前构造最直接 | 重复完整路由、validator 和接口，路径冲突且 owner 不清 | 拒绝 |

## 5. 推荐推断

保留一份完整生成 `api.StrictServerInterface` 和一份生成 route binding。模块 HTTP binding 定义并实现自己的 operation 子集；application composition 的静态 aggregate 是唯一满足完整接口的位置，并显式转发到各模块 Handler。

这样新增模块的固定步骤是：

1. 在唯一 OpenAPI authority 增加带稳定 tag/operationId/policy 的契约并生成；
2. 新模块实现自己拥有的 operation Handler；
3. aggregate 增加该模块字段与对应转发方法；
4. composition 注入新 Handler；
5. route binding、Router、Server 和全局 middleware 不复制、不新增路径字符串。

完整接口扩张只使 aggregate 在编译期报出“尚未连接的新 operation”，不会迫使 Todo Handler 实现其他模块的方法。这一失败位置正是唯一 composition root，信息直接且可修复。

## 6. 升级触发器

只有出现以下事实之一，才重新评估按 tag/spec 分区：

- 单个生成文件或完整接口已经显著阻碍并行开发和审阅；
- 不同 API 域需要独立版本、发布节奏或 Go package ownership；
- schema 已自然拆成可治理的共享规范，import mapping 不会制造循环或重复 authority；
- 生成时间或编译影响已有测量证据，而不是预想中的性能问题。

到达触发器时应建立新研究和计划，不能在 026 中预留双轨 generator 或 compatibility wrapper。

## 7. 局限与研究门禁

本研究固定在仓库当前使用的 v2.8.0；后续升级必须复核配置 schema、生成 diff 和 strict server 行为。官方支持的是生成过滤与 spec/package 拆分能力，不自动解决项目的模块 owner、授权和生命周期，这些仍由本项目设计决定。

工具能力、当前规模和三种路径的取舍已经足以支持计划，研究门禁通过。
