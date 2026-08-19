# R002 文档与实现一致性核对

## 1. 研究问题

项目权威文档（架构、模块接入方式、binding 契约、装配流程、配置方式、扩展方式、门禁规范）是否与当前代码实现和实际能力一致？是否存在文档描述尚未具备的能力，或代码已变化但文档停留在旧设计？

## 2. 方法与范围

- 只读取权威文档与当前实现源码作为证据，不编写实现。
- 复核对象：`docs/development/application-module-development.md`、`internal/module/README.md`、`api/README.md`、`docs/configuration/README.md`、`docs/changes/README.md`、`internal/kernel/composition/architecture_test.go`，对照 `internal/composition/**`、`internal/tools/contract-gen/main.go`、各业务模块。

## 3. 证据与发现（事实）

### 3.1 已由近期变更对齐过的部分（一致）

- `application-module-development.md` 已更新为 031 分层（handler/ + binding/http）、032 i18n 接入、033 统一 binding 契约清单（4.1）与业务模块自有 i18n binding（8.4）；与当前 Todo/contract-gen 实现基本一致。
- `internal/module/README.md` 已含各模块契约形态（Todo/Ops/Auth/Migration），与当前模块结构一致。
- `api/README.md` 描述「Go 代码唯一权威，openapi.yaml 由 contract-gen 生成」，与 `contract-gen` 行为一致。
- `docs/changes/README.md` 索引与变更记录一致，下一序号 `034`。

### 3.2 需要进一步核对/可能存在偏差的点（推断，待实施时精化）

1. **`registeredModules()` 注册点说明**：`contract-gen/main.go` 里新模块需在 `registeredModules()` 手动追加一行。这是 composition 之外的注册点；需核对 `application-module-development.md` 或 `api/README.md` 是否明确把这一「生成器注册点」写清楚，避免文档暗示“接入只需改 composition”却漏掉此处。
2. **Ops 独立 management HTTP**：`internal/module/README.md` 已写明 Ops management 独立、不参与公开 contract-gen。需与 `api/README.md`/开发指南的「公开契约」范围交叉核对，确保两处描述一致。
3. **composition 多文件散落装配**：R001 发现 Todo 装配散落于 `http_api.go/service.go/ops.go/todo.go/generation.go`。需核对开发指南的「扩展方式/新增模块接入」段落是否诚实描述「新增 HTTP 模块需在哪些文件手改」，或文档是否只概括为「在 composition 聚合」而实际需要更多操作（潜在的文档-实现不一致）。
4. **门禁规范**：`architecture_test.go` 覆盖 pkg/kernel-app 边界、模块 handler 依赖、module contract 等。需核对 `pkg/README.md` 与开发指南所列门禁与 `architecture_test.go` 实际断言一致（尤其 032 的 `validateKernelAppConfigOwnership`、033 的模块契约分层约束）。
5. **配置说明**：`docs/configuration/README.md` 的 `i18n` 行已注明 `./locales`；需核对与 `config.example.yaml` 及 `kernel/app/i18n` 实际配置节一致（i18n.messageFiles 已聚合 Todo 模块资源）。

## 4. 推断

1. 文档总体已随 031/032/033 更新，较一致；风险集中在「扩展/新增模块接入」的文档是否与“实际需手改多个 composition 文件 + contract-gen 注册”一致。
2. 若文档只写「在 composition 聚合」而未列具体文件与注册点，则存在文档-实现不一致（文档暗示更简单）。
3. 门禁说明与 `architecture_test.go` 的具体断言应对齐，避免文档描述的门禁比代码更强或更弱。

## 5. 适用与不适用场景

- 适用：文档-实现收口、扩展接入流程说明、门禁说明复核。
- 不适用：重写文档体系、引入新架构能力。

## 6. 局限与剩余未知

- 本轮为抽查式一致性核对；逐条交叉比对需在实施阶段完成并列出最终偏移清单。
- 若后续采用「收敛装配入口」修正（见 R001 与 design），文档需随之更新，避免再次漂移。

## 7. 对 034 的影响

- 计划需包含一份「文档同步」任务：与装配收口、registeredModules 说明、扩展流程、门禁说明一起更新，确保“文档描述的能力 == 当前实现”。
- 研究门禁通过。
