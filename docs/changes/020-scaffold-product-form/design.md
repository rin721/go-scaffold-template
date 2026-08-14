# 设计：版本化源码复制与独立所有权

## 1. 决策

用户已选择 **copy-owned source scaffold**：`go-scaffold2` 发布完整源码基线，开发者复制源码后一次性迁移身份，所有代码从此归新项目。

此前研究比较过 template、generator、library 和组合模式。比较结论仍作为历史依据，但 generator/library/组合模式已被用户决策排除，不再进入验证或后续实现。

## 2. 为什么适合当前代码

当前底层装配是：

```text
pkg/<capability>
  -> internal/kernel/app/<capability>
  -> internal/kernel/composition
  -> internal/composition
  -> cmd/app
```

如果另一个 module 只依赖源仓库，Go 的 `internal` 规则会阻止导入完整 Runtime；复制源码后，`internal/**` 位于目标 module 自己的目录树内，这个问题自然消失。`pkg` 与 Kernel App 会一起复制，不需要发明公共 Runtime API，也不改变当前显式 Plan 装配、租约、Reload 和反向停止语义。

代价同样明确：复制后的服务不会自动获得上游改进。这个代价属于已选产品模型，不能再用隐藏 framework dependency 或自动覆盖用户文件规避。

## 3. 产品边界

### 3.1 脚手架仓库负责

- 发布经过验证的完整源码 baseline；
- 说明复制范围和排除范围；
- 提供一次性身份迁移清单和验证清单；
- 将 Todo 标为可保留/可移除示例；
- 为重要修复提供版本化 release notes 和人工迁移说明。

### 3.2 新项目负责

- 选择 baseline；
- 执行复制和身份迁移；
- 拥有并修改全部 `cmd`、`internal`、`pkg`、配置、文档和 CI；
- 自行决定是否采纳后续上游变化；
- 建立独立 Git 历史和发布节奏。

### 3.3 明确不存在

- generator-owned 文件；
- 运行期 scaffold dependency；
- 公共 Kernel Runtime module；
- 自动模板 schema migration；
- 上游仓库对副本的文件控制权。

## 4. 标准复制流程

020 的隔离验证使用以下目标流程，但不在未确认前实际执行：

```text
1. 固定 source commit
2. 复制 tracked baseline 到 tmp/scaffold-copy-validation/service
3. 排除 .git、tmp、.data、config.yaml 和其他本机/运行态文件
4. 迁移目标 identity
5. 保留 Todo 验证完整基线
6. 在另一副本验证 Todo 完整移除
7. 执行 build/test/vet/config init 与残留扫描
8. 记录 baseline provenance 和验证结果
```

复制的是已跟踪、被发布策略允许的内容，而不是当前工作目录的所有文件。这样不会把用户本地配置、SQLite 文件、缓存或本任务验证目录带入新项目。

## 5. 身份迁移模型

身份迁移不是任意文本全局替换。需要建立有所有者的迁移表：

| 身份 | 当前来源 | 目标行为 |
| --- | --- | --- |
| Go module path | `go.mod` 与 Go imports | 改成目标 module，随后 `go mod tidy` |
| 应用名/描述 | `cmd/app` 固定进程输入 | 改成目标服务身份 |
| 二进制/命令示例 | README、CI、运行命令 | 同步目标可执行名 |
| 配置文件名 | 入口默认值与文档 | 显式保留或改名，并同步引用 |
| 环境变量前缀 | 入口、测试、示例配置、文档 | 改成目标专用前缀并验证嵌套规则 |
| 架构测试身份 | boundary/architecture tests | 迁移为目标 module path，保持门禁语义 |
| 来源记录 | 新项目文档 | 记录 source commit/tag，不形成依赖 |

验证可以使用只读扫描命令；正式方案不需要 project generator，也不需要把身份改名做成长期运行工具。

## 6. Todo 示例策略

默认完整复制保留 Todo，因为它是当前唯一证明 HTTP、CLI、Database、Config、migration、module contribution 和 Host 能闭环工作的垂直切片。

同时必须验证“移除 Todo”所需的完整集合：

```text
internal/module/todo
internal/composition 中 Todo 构造与 Adapter
Todo 配置 binding/default/example
Todo routes、CLI commands、migration 与测试
README/架构文档引用
```

如果当前应用 composition 与 Todo 耦合导致移除后无法形成最小服务，020 只记录真实缺口并建立后续任务；不得在隔离验证中把占位业务模块写回生产源码。

## 7. 版本与升级语义

### Baseline 版本

- 脚手架以 Git tag/release 标识可复制 baseline；当前仓库没有 tag，因此发布能力仍是缺口。
- 新项目记录所用 tag/commit 和复制日期，便于判断安全公告是否适用。
- source commit 只表示来源，不建立父子仓库自动同步关系。

### 后续改进

按性质传播：

| 变化 | 脚手架发布内容 | 新项目动作 |
| --- | --- | --- |
| 安全修复 | 受影响 baseline、修复 diff、验证命令 | 人工审阅并迁移 |
| 缺陷修复 | release note、涉及文件和行为变化 | 按需迁移 |
| 新能力 | 当前架构文档与独立变更说明 | 自主选择，不自动加入 |
| 破坏性架构变化 | 新 baseline 与迁移指南 | 评估后人工改造或不升级 |

不使用版本号暗示已复制项目可以执行 `go get` 升级整个脚手架。

## 8. 隔离验证设计

确认后仅在 Git 忽略的 `tmp/scaffold-copy-validation/` 建立：

```text
tmp/scaffold-copy-validation/
├── source-manifest/       # source commit 与包含/排除清单
├── todo-service/          # 新 identity，保留 Todo
├── minimal-service/       # 新 identity，尝试完整移除 Todo
└── evidence/              # 命令与结果摘要，不保存凭据
```

验证副本不得通过 `replace`、Go workspace 或相对链接返回源工作区。验证结束后临时目录保持忽略，不提交。

## 9. 决策记录输出

验证完成后在 020 内形成 `ADR-001` 结果，记录：

- copy-owned 是唯一产品形态；
- 全部源码归新项目；
- `internal` 与 `pkg` 整体复制；
- baseline/provenance 语义；
- 无 generator、公共 Runtime 和自动升级承诺；
- Todo 默认保留与移除边界；
- 正式复制指南、release 和迁移公告需要哪些后续变更。

019 的 `API-AUTHORITY-001` 随后直接设计在复制后的完整应用源码中，不需要再判断 contract 属于外部 Runtime 还是 generator 模板。
