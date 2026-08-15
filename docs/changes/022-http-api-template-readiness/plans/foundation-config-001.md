# FOUNDATION-CONFIG-001：配置 Source 路径确定性施工计划

## 1. 状态与确认边界

- Program ID：`FOUNDATION-CONFIG-001`
- 优先级：P0
- 当前状态：**已确认并实施完成**
- 研究依据：[R007](../research/R007-config-source-determinism/report.md)，并复用 [012-R019](../../012-business-module-architecture/research/R019-config-contracts/report.md)、[022-R002](../research/R002-foundation-closure-audit/report.md) 与 [022-R004](../research/R004-foundation-closure-synthesis/report.md)。
- 目标门禁：`FND-CONFIG-001..003`、`FND-ACCEPT-004`
- 本计划只冻结 Config Source syntax/shape 与 Loader merge；不启动服务、不修改真实配置或环境、不部署、不推送。

用户已在计划报告后的后续消息明确确认 `FOUNDATION-CONFIG-001` 当前计划；本轮据此实施 `CFG-001..008`，不得扩张到 `FOUNDATION-ACCEPTANCE-001` 或其他 Program item。

## 2. 目标与验收结果

完成后必须同时成立：

1. 同一 `EnvSource` 的重复逻辑路径、祖先/后代冲突、空 segment 和大小写碰撞确定性失败，不依赖环境枚举顺序；
2. 不同 Source 只允许 object/object 递归 merge 或 non-object/non-object 高优先级整体替换；object 与 scalar/array/null 任一方向改形状都失败；
3. 每个 object scope 内大小写等价 sibling 唯一，Loader 不再从 map 枚举中任意选择目标；
4. 冲突错误携带 Source identity、安全 dotted path 和类别，不包含原始配置 value；
5. File < Env 优先级、Source provenance、Snapshot digest/redaction/immutability、strict owner binding 和 Coordinator 单候选边界不变；
6. 所有冲突在 `Coordinator.Prepare` 返回，不进入 Kernel Stage/Build 或 application module construction；
7. 不新增依赖、公共 API、第二套配置框架、兼容 alias 或逐 path policy registry。

本任务完成仍只关闭配置 P0。`FOUNDATION-ACCEPTANCE-001` 完成并复核十一门之前，022 继续是 `Foundation-partial`，不得声明 `Foundation-closed` 或解锁新业务详细设计。

## 3. 非目标

- 不改变 `Source` interface、`Loader.New/Load`、Snapshot 或 `config.Binding`；
- 不改变 File/Env 注册顺序、环境变量 prefix 或 `__` nesting；
- 不新增 remote Source、CLI override、schema version、字段级 provenance 或 management API；
- 不为 null 设计“删除字段/恢复默认”语义；null 只作为既有 non-object canonical value；
- 不全局 lower-case File/Map key，不主动改变现有合法配置的 key spelling 或 digest；
- 不修改 owner default、typed Decode、reload class、资源生命周期或 diagnostics 数据结构；
- 不启动真实 Database、Redis、Storage、HTTP listener 或 watcher。

## 4. 冻结语义

### 4.1 Path identity

- 环境变量去掉 prefix 后，以 `__` 划分 segment；任意空 segment 都是 syntax error。
- Env segment 继续转小写，保持现有合法输入行为。
- 同一 object scope 的逻辑 key 使用 `strings.EqualFold` 判等；一个 scope 只允许一个等价 spelling。
- error path 使用不带 value 的 dotted path，例如 `database.dsn`；不得输出 DSN、password、token 或环境值。
- 多项错误同时存在时，对 normalized dotted path 稳定排序，报告第一个冲突；测试不依赖 `os.Environ` 或 Go map 顺序。

### 4.2 EnvSource 同源规则

| 输入关系 | 目标结果 |
| --- | --- |
| `APP_DATABASE__DSN` 单独出现 | 生成 `database.dsn` scalar |
| 同一逻辑 path 重复或仅大小写不同 | 拒绝 duplicate logical path |
| `APP_DATABASE` 与 `APP_DATABASE__DSN` 任意顺序 | 拒绝 ancestor/descendant shape conflict |
| `APP_DATABASE____DSN`、前导/尾随空段 | 拒绝 empty path segment |
| `APP_DATABASE__DSN=` | 接受显式空字符串，交给 owner typed/semantic validation |
| 名称恰好等于 prefix | 继续忽略，不生成空 root path |

实现边界采用 `EnvSource.Load -> loadEnvironment(ctx, prefix, entries)` 一类内部 helper：生产传入 `os.Environ()`，测试传入受控 entry slice。helper 名称可以按 Go 可读性微调，但不得把 enumeration seam 暴露为公共 Source API。

### 4.3 Loader 跨源规则

使用两类结构形状：

- `object`：`map[string]any`
- `non-object`：array、string、bool、number、null

| 低优先级已有值 | 高优先级新值 | 目标结果 |
| --- | --- | --- |
| object | object | 按唯一 logical key 递归 merge |
| object | non-object（含 null） | 拒绝 shape conflict |
| non-object（含 null） | object | 拒绝 shape conflict |
| non-object | non-object | 高优先级整体替换 |
| 同 scope 出现大小写等价 sibling | 任意 | 在 Source canonical 边界拒绝，不进入 merge |

File < Env 顺序不变。合法覆盖继续保留 Source spelling：已有 File key 被 Env 以不同大小写命中时，更新同一 logical key，不新建第二个 sibling。provenance 仍只记录成功参与快照的 Source 顺序；失败候选不产生 Snapshot。

### 4.4 失败语义

- `EnvSource.Load` 负责环境 path syntax/同源冲突，返回 leaf validation error；
- `Loader.Load` 继续用 `load/normalize/merge config source <name>: %w` 增加 Source 边界上下文并保留 error chain；
- 用未导出的 typed path error 保存 `kind` 与安全 path，使包内测试可经 `errors.As` 断言 category；不新增对外 error code 或 sentinel，不冻结整句英文文本；
- 失败不返回部分 Snapshot，不追加失败 Source provenance，不触发 binding、Stage、Build、Ready、commit 或 reload side effect；
- 空 value 不是 Source error，最终是否有效由该 section owner 决定。

## 5. 内部实现设计

### 5.1 `internal/kernel/config/config.go`

计划在现有文件内收敛以下职责，不新建通用 `utils` 包：

1. EnvSource 先收集 prefix 命中的 entry，解析为安全 path record，稳定排序后校验并构造 canonical object；
2. 把当前无错误 `setNested` 替换为有错误返回的 path insertion，拒绝 duplicate、ancestor/descendant 和空段；
3. `canonicalMap` 在每个 object scope 校验空 key及 `EqualFold` sibling collision，再递归复制值；
4. `mergeMap` 按稳定 key 顺序递归，并使用统一 `object`/`non-object` 分类；删除当前 object/null 例外；
5. `matchingKey` 只在 canonical 唯一 sibling 集合上匹配，因此不得再从多个等价 key 中任意选择；如实现改为返回 `(string, bool)`，仍保持内部函数；
6. context 在环境 entry 收集/处理期间继续检查，取消错误由现有 Loader wrapper 保留。

不建立 trie 公共类型、schema registry、反射模型或第三方 Adapter。若局部 path record/分类函数能清楚表达不变量，可作为未导出小类型留在 `config.go`。

### 5.2 事务边界保持不变

```text
Env/File/Map Source
  -> source-local syntax + canonical sibling validation
  -> deterministic Loader merge
  -> Snapshot
  -> Coordinator.ValidateCandidate
  -> Kernel/application prepare
  -> Stage/Build/Ready/commit
```

本任务不把结构校验移入 owner binding，因为那会让多个 owner 重复处理全树冲突，也无法保证未知顶层或两个 section 之间的 path 唯一性。

## 6. 文件影响

### 6.1 非文档实施文件（确认后）

- `internal/kernel/config/config.go`：Env entry 解析、path insertion、case collision 与 deterministic merge。
- `internal/kernel/config/config_test.go`：受控 entry permutation、同源/跨源矩阵、null、case、empty value、provenance 与安全错误。
- `internal/kernel/kernel_test.go`：冲突在 `Coordinator.Prepare` 阶段失败且 component Stage/Build 计数保持零的集成证据。

### 6.2 实施后同步的权威/任务文档

- `README.md`：在当前环境覆盖说明中补充歧义路径 fail-fast，不改变运行命令。
- `internal/kernel/README.md`：记录 object/non-object、case 与 Env path 规则及失败边界。
- `docs/changes/012-business-module-architecture/design/config-contracts.md`：把既有目标语义链接到 022 的当前实施证据，不复制施工正文。
- `docs/changes/022-http-api-template-readiness/{README.md,requirements.md,design.md,acceptance.md,tasks.md}`：只在真实实现和验证完成后更新当前状态与 evidence。
- `docs/changes/022-http-api-template-readiness/plans/foundation-config-001.md`：逐项记录 `CFG-*` 证据、验证和 Commit。

预计不修改 `go.mod`/`go.sum`、配置样例、owner Config、composition、Coordinator、Kernel 实现或生命周期代码。若实施必须触及这些边界，先执行第 11 节重新确认。

## 7. 测试矩阵

### 7.1 EnvSource 单元测试

通过内部受控 entry slice 覆盖每组正反排列，不依赖宿主 OS 是否允许大小写重复环境名：

1. scalar -> object 与 object -> scalar 均失败，error path 相同；
2. 三层祖先/后代冲突的所有关键排列均失败；
3. 完全重复和仅大小写不同的 logical path 失败；
4. 中间、前导、尾随空 segment 失败；
5. 空 value 被保留为 `""`；值中包含 `=` 时完整保留；
6. 无关 prefix 被忽略，名称等于 prefix 被忽略；
7. 预取消 context 返回可识别 cancellation chain；
8. error 不包含注入的 secret value。

### 7.2 Loader 单元测试

1. object/object 递归 merge，File spelling 与 Env 高优先级值正确；
2. scalar/array/null 之间的 non-object 覆盖保持成功；
3. object -> scalar/array/null 和反向全部失败；
4. 同 Source 同 scope 的 `Database`/`database` alias 失败；
5. 多冲突 map 重复运行返回同一 canonical first path；
6. 合法覆盖的 digest 稳定、provenance 仍为声明顺序；
7. unknown field、duplicate JSON/YAML、strict cross-type、redaction 与 Snapshot copy 既有测试不回退。

### 7.3 Coordinator/Kernel 集成测试

建立一个带 Stage/Build 计数的测试 component：`Coordinator.Prepare` 接收结构冲突 Loader 时必须直接返回 error，prepared candidate 为空，Stage/Build 均为零；随后使用合法候选的既有启动路径仍通过。该测试不启动真实外部资源。

## 8. 实施任务

| Task ID | 依赖 | 工作 | 完成条件 | 当前状态 |
| --- | --- | --- | --- | --- |
| `CFG-001` | 用户确认 | 在 config owner 内增加受控 Env entry 解析边界与稳定 path record | 生产仍只调用 `os.Environ`，测试可置换 entry，不增加公共 API | 已完成 |
| `CFG-002` | `CFG-001` | 用 error-returning path insertion 替换 last-write-wins `setNested` | duplicate、case、ancestor/descendant、empty segment 全部 fail-fast | 已完成 |
| `CFG-003` | `CFG-002` | 使 canonical object sibling identity 唯一并稳定遍历 | 同 scope `EqualFold` alias 被拒绝，多冲突首 path 稳定 | 已完成 |
| `CFG-004` | `CFG-003` | 统一 Loader object/non-object merge，删除 null 例外 | object/null 双向失败；合法 non-object replacement 与 object merge 保持 | 已完成 |
| `CFG-005` | `CFG-001..004` | 补 EnvSource 与 Loader 表驱动/置换测试 | 第 7.1、7.2 节矩阵全部通过且错误不泄露 value | 已完成 |
| `CFG-006` | `CFG-004` | 增加 Coordinator/Kernel Build-zero 集成门禁 | 冲突在 candidate 前停止，Stage/Build 计数均为零 | 已完成 |
| `CFG-007` | `CFG-001..006` | 同步当前权威文档与 022 状态/证据 | 实现事实、目标设计和未完成 Foundation acceptance 分离 | 已完成 |
| `CFG-008` | `CFG-001..007` | 完成验证、Diff 审计和单轨残留搜索 | 第 9 节全部通过，无旧 `setNested`/null 例外/重复语义残留 | 已完成 |

## 9. 验证命令与门禁

确认实施后按风险顺序执行：

```powershell
gofmt -w internal/kernel/config/config.go internal/kernel/config/config_test.go internal/kernel/kernel_test.go
go test ./internal/kernel/config -count=1
go test ./internal/kernel/... -count=1
go test ./...
go test -race ./...
go vet ./...
go build ./...
$foundationConfigPreviousGOOS = $env:GOOS
$foundationConfigPreviousGOARCH = $env:GOARCH
try {
    $env:GOOS = 'linux'
    $env:GOARCH = 'amd64'
    go build ./...
} finally {
    $env:GOOS = $foundationConfigPreviousGOOS
    $env:GOARCH = $foundationConfigPreviousGOARCH
}
git diff --check
```

另外执行：

- 搜索 `setNested`、`existing != nil`、`value != nil` 和旧冲突文案，确认旧语义没有第二轨残留；
- 检查所有 `config.New`、`EnvSource`、`Loader.Load` 调用方，确认无需迁移公共 API；
- 运行仓库现有 Markdown 链接检查或等价的排除 fenced/inline code 的校验；
- 审阅完整 diff，确认没有环境值、凭据、构建产物或范围外文件。

Linux 这里只做无输出的 `go build ./...` 交叉编译，不声称 Linux runtime 环境变量行为已经运行验收。若 race、跨平台或现有全量检查失败，必须区分本任务回归、既有问题和环境限制，不能把失败描述为通过。

## 10. 实施提交边界

用户已选择计划阶段不单独提交。确认实施后，R007/本计划、本任务 Go 变更、测试、权威文档和实施证据作为一个任务提交；只暂存明确列出的任务文件，使用 Conventional Commits，例如：

```text
fix(config): reject ambiguous source paths
```

不 amend、不 rebase、不 force-push；本计划不授权 push、PR、tag 或 release。

## 11. 必须退回研究并重新确认的变化

出现以下任一事实时停止实施，更新 R007/本计划并重新提交报告：

- 需要改变 `Source`、`Loader`、Snapshot、Binding 或 Coordinator 公共/跨包契约；
- 需要新增依赖、第二套配置框架、remote Source 或配置 watcher；
- 需要全局 lower-case key、改变 File < Env 顺序、改变合法配置 digest/provenance；
- 发现真实兼容输入依赖大小写 alias、object/null 改形状或逐 path merge exception；
- 需要字段级 provenance、schema version、deprecated alias 或 null 删除语义；
- 需要修改 owner Config、资源 Build/reload/lifecycle/diagnostics 边界；
- 测试证明冲突不能在 `Coordinator.Prepare` 前停止，必须重构 composition。

普通的未导出 helper 命名、局部数据结构和测试组织不构成实质变化，只要第 2 至 4 节契约不变。

## 12. 停止线

`CFG-001..008` 已完成。本任务在验证、Diff 审计和提交后停止；不得顺带启动服务、修改真实配置、推送或开展 `FOUNDATION-ACCEPTANCE-001`。

## 13. 实施证据

### 13.1 代码与测试

- `EnvSource.Load` 只把 `os.Environ()` 交给内部 `loadEnvironment`；测试以受控 entry slice 覆盖顺序置换，不增加公共 API。
- 未导出的 `configPathError` 保留 `kind` 与安全 path；Loader 继续用 `%w` 加 Source 边界，错误不携带配置 value。
- Env records 先规范化、稳定排序并统一校验 duplicate/case/empty/ancestor；旧 last-write-wins `setNested` 已由 error-returning `insertPath` 单轨替换。
- `canonicalMapAt` 在每层拒绝 `EqualFold` sibling collision；`mergeMap` 稳定遍历并只允许 object/object 或 non-object/non-object。
- Config 测试覆盖正反顺序、三层 ancestor、exact/case duplicate、空段、空值、值中 `=`、取消、object/null 双向、合法同形覆盖、provenance、稳定首错误与脱敏。
- Kernel 测试用 counting wrapper 证明冲突时 `Coordinator.Prepare` 不留下 candidate，component Stage/Build 均为零；合法候选仍各执行一次。

### 13.2 验证

以下命令均在 Go 1.25.7、Windows amd64 上通过：

```text
go test ./internal/kernel/config -count=1
go test ./internal/kernel/... -count=1
go test ./...
go test -race ./...
go vet ./...
go build ./...
GOOS=linux GOARCH=amd64 go build ./...
git diff --check
```

Markdown 相对链接检查排除 fenced/inline code 后通过。Linux 只完成交叉编译，没有声明 Linux runtime 环境枚举验收；真实服务、外部资源和 `FOUNDATION-ACCEPTANCE-001` 未执行。

### 13.3 提交

- Commit：本任务提交 `fix(config): reject ambiguous source paths`。
- 范围：R007/计划、Config 实现与测试、Kernel Build-zero 门禁、当前权威文档和 022 证据同一提交。
