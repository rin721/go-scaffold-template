# R007：EnvSource 与 Loader 配置路径确定性

> 状态：实施前快照。`FOUNDATION-CONFIG-001` 已按本报告形成的计划实施；当前行为以代码、测试和 [施工计划证据](../../plans/foundation-config-001.md) 为准。

## 1. 研究问题与结论

本报告回答 `FOUNDATION-CONFIG-001` 形成施工计划前的四个问题：

1. 同一 `EnvSource` 的祖先/后代路径、大小写别名和空 path segment 当前如何处理；
2. 不同 Source 的 object、array、scalar、null 和大小写别名当前如何 merge；
3. 冲突能否在 Kernel/Application owner 校验和资源 Build 前稳定失败；
4. 修复是否需要改变 `Source`、`Loader`、Snapshot、优先级或引入新依赖。

结论：当前缺口局限在 `internal/kernel/config` 的 Source syntax/shape 边界，不需要新配置框架或公共 API。`Loader` 已按声明顺序加载、规范化并 merge，生产路径固定 `FileSource -> EnvSource`；`Coordinator.Prepare` 也先等待 `Loader.Load` 成功，随后才校验 owner，Kernel Build 更晚。因此推荐保留主链，只把 Env 路径构造和 Loader merge 收敛为同一套确定性 object/non-object 规则，并补齐顺序置换、大小写、空段、null 和“Build 未发生”的负向证据。

研究门禁通过只表示证据足以形成计划，不表示源码实施已经授权。

## 2. 方法、范围与快照

### 2.1 本轮复核

- Git：`main`，HEAD `6f1521a211c6aa6b137db8464be4633bd9ded809`，研究开始时工作树干净。
- Go：`go1.25.7 windows/amd64`。
- 代码：`internal/kernel/config/config.go`、`config_test.go`、`internal/kernel/coordinator.go`、`internal/kernel/kernel.go`、`internal/composition/todo.go`。
- 既有研究：复用 012-R019 的 strict/canonical 配置原则、022-R002 的底层调用链与 022-R004 的 Foundation 阻断判断；三者的 EnvSource 变化刷新条件尚未触发。
- 验证：`go test ./internal/kernel/config -count=1` 通过，只证明当前已有用例，不证明缺失的冲突语义。
- 标准库：`os.Environ` 文档只承诺返回 `key=value` 字符串副本，没有承诺业务可依赖的排序；Go 1.25.7 的 Windows 与 Unix 实现也来自不同平台环境表示。

### 2.2 本轮不做

- 不实现、不启动服务、不修改配置文件或环境；
- 不研究 remote Source、CLI override、schema version 或字段级 provenance；
- 不改变 owner default、strict Decode、reload、Snapshot digest 或生命周期；
- 不用外部框架能力代替本地代码证据。

## 3. 当前事实

### 3.1 Loader 主链已有的保证

`Loader.Load` 当前按注册顺序执行以下步骤：

```text
Source.Load
  -> canonicalMap
  -> mergeMap
  -> provenance append
  -> immutable Snapshot(raw/redacted/digest/provenance)
```

已经成立的事实：

- nil Source、空 Source name、重复 Source name、取消和 Source error 会失败；
- canonical tree 只接收 object、array、string/bool/number/null，并复制 map/slice；
- JSON/YAML 完全相同的重复 key 已在 parser 边界失败；
- `Snapshot.Decode` 使用 `ErrorUnused=true`、`WeaklyTypedInput=false` 和有限字符串 scalar hook；
- object/object 递归 merge，生产优先级是 File 后 Env，provenance 保留 Source 顺序；
- `Coordinator.Prepare` 只有在 `Loader.Load` 成功后才执行全部 binding 校验；`Kernel.startCandidate` 的 Stage/Build 在更晚的 `Coordinator.Start` 中发生。

这些能力不应在本任务中重写。

### 3.2 EnvSource 内部冲突会静默改形状

`EnvSource.Load` 遍历 `os.Environ()`，去掉 prefix，把剩余名称转小写并按 `__` 分段，然后调用无错误返回的 `setNested`。

`setNested` 有两个覆盖窗口：

- 先看到 `APP_DATABASE=value`，后看到 `APP_DATABASE__DSN=db` 时，原 scalar 被新 object 替换；
- 先看到 `APP_DATABASE__DSN=db`，后看到 `APP_DATABASE=value` 时，原 object 被新 scalar 替换。

因此两个方向都不会拒绝，最终结果取决于枚举顺序。相同问题适用于任意祖先/后代路径。

环境变量名称先转小写，所以在允许大小写不同变量并存的平台上，`APP_DATABASE__DSN` 与 `APP_database__dsn` 会成为同一路径并后写覆盖。当前测试使用 `t.Setenv`，无法在 Windows 稳定构造这种碰撞，也没有注入受控 entry 列表验证顺序置换。

### 3.3 空段已经失败，但失败层次没有统一

`APP_DATABASE____DSN` 会被拆为 `database`、空字符串、`dsn`。`setNested` 会先生成带空 key 的 object，随后 `Loader.canonicalMap` 以 `configuration key is empty` 失败。

这说明当前不会产生 Snapshot，但 EnvSource 自己没有把错误定位为环境路径语法，且它与祖先/后代冲突走的是不同机制。`FND-ACCEPT-004` 要求这些输入拥有同一 Source syntax/shape 边界和安全 path 诊断。

### 3.4 跨 Source 拒绝大体存在，但仍有两个不确定边界

`mergeMap` 已拒绝大多数 object 与 scalar/array 相互覆盖，同时允许：

- object/object 递归 merge；
- scalar、array 之间由高优先级 Source 整体替换。

仍有两个缺口：

1. `null` 属于既有 canonical scalar 值域，但 `existing != nil` / `value != nil` 条件允许 object 与 null 相互改形状；012 的设计没有真实 null 清空场景，并要求 object 与 scalar/array 冲突失败。
2. `matchingKey` 通过遍历 map 查找 `strings.EqualFold` key。如果同一 object 内已经存在大小写等价的两个 sibling，命中哪个 key 取决于 map 枚举。`canonicalMap` 当前没有拒绝这种别名；`mergeMap` 自身也遍历 map，在存在多个冲突时先报告哪个 path 不稳定。

这里的目标不是强制所有 key 转为小写。当前 File key spelling、Env lower-case 和 mapstructure 的匹配行为可以保留；只需保证每个 sibling scope 内大小写等价的逻辑 key 唯一，并以稳定顺序校验/merge。

### 3.5 错误可以在资源副作用前停止

生产调用链为：

```text
internal/composition.prepareTodo
  -> config.New(FileSource, EnvSource)
  -> kernel.New + composition.Compose       # 只建立定义和 Plan
  -> kernel.NewCoordinator
  -> Coordinator.Prepare
       -> Loader.Load
       -> ValidateCandidate
  -> module decode/construction
  -> Coordinator.Start
       -> Kernel Stage/Build/Ready
```

因此只要 Env/merge 冲突从 `Loader.Load` 返回 error，就不会产生 candidate，也不会进入 module construction、Kernel Stage 或资源 Build。当前缺少的是显式集成测试证据，不是调用链需要重构。

## 4. 推断与方案比较

以下是基于上述事实的设计推断，不是当前已实现行为。

| 方案 | 收益 | 缺口/风险 | 判断 |
| --- | --- | --- | --- |
| 对 Env entry 排序后继续 last-write-wins | 输出看似稳定 | 仍把歧义配置当成功，违反 `FND-CONFIG-001` | 拒绝 |
| 只让 `setNested` 返回 error | 修复同源祖先/后代 | null、大小写 sibling 和多冲突顺序仍留在 Loader | 不完整 |
| 所有 key 全局转小写 | 实现简单 | 改变现有 Snapshot key spelling/digest，兼容影响未研究 | 拒绝 |
| 在 config owner 内统一 path identity、object/non-object 分类与稳定遍历 | 保留 API/优先级，完整覆盖同源和跨源冲突 | 需要精确负向矩阵 | **推荐** |
| 引入 Viper/远端配置框架 | 获得更多功能 | 新依赖和第二套语义，不能自动证明项目冲突契约 | 拒绝 |

推荐规则：

- 同一 EnvSource 的重复逻辑路径、祖先/后代路径、空段和大小写别名一律拒绝；空 value 仍是合法显式 scalar；
- 每个 Source 的同层大小写等价 key 必须唯一；
- object/object 递归 merge；object 与任何 non-object（包括 null）互相覆盖失败；non-object 之间继续由高优先级 Source 整体替换；
- 多个冲突同时存在时，按 canonical dotted path 的稳定顺序选择首个错误；
- error 包含 Source identity、安全 path 和冲突类别，不包含配置 value；
- `Source`、`Loader.New/Load`、Snapshot、File < Env 顺序和 provenance 结构不变。

## 5. 适用、不适用与局限

### 5.1 适用

- 当前 File/Env 配置加载；
- 自定义 `MapSource` 和未来 Source 输出的 canonical object；
- startup、Application CLI operation 和 reload 前的同一 Loader 门禁；
- 后续 Source 接入前复用同一结构冲突规则。

### 5.2 不适用

- 通过 null 表达字段删除或继承清空；当前没有该产品需求；
- 配置 key 重命名、deprecated alias 或逐 path 迁移；需要独立兼容期限和删除计划；
- 字段级 provenance、secret schema、remote watch 或配置中心；
- 证明 Foundation-closed；本任务完成后仍需 `FOUNDATION-ACCEPTANCE-001` 总验收。

### 5.3 局限与剩余未知

- 本轮没有修改代码，因此没有顺序置换测试或 Linux 运行证据；施工计划必须用受控 entry slice 避免依赖宿主环境能力。
- 当前项目 key 均为英文标识符；未定义 Unicode confusable 的产品语义。计划只沿用 Go `strings.EqualFold` 的 sibling identity，不扩大为 Unicode 安全治理。
- 没有真实 null 清空需求；如果实施时发现兼容输入依赖 object/null 改形状，必须退回研究和重新确认，不能静默保留例外。
- 未来 Source 如需逐 path merge exception，必须单独设计；本任务不预建 policy registry。

## 6. 对 FOUNDATION-CONFIG-001 的影响

研究门禁已具备形成计划的证据，推荐施工范围仅包括：

1. 在 `internal/kernel/config` 增加可用受控 entry 测试的 Env 解析边界；
2. 统一 Env path 插入、canonical sibling identity 和 Loader object/non-object 冲突；
3. 增加 permutation、case、empty、null、precedence/provenance 和 Build-zero 测试；
4. 同步 Kernel 当前说明与 022 验收证据；
5. 不新增依赖、不新增公共 API、不改变 owner binding、Coordinator 或生命周期。

唯一施工级计划见 [`FOUNDATION-CONFIG-001`](../../plans/foundation-config-001.md)。在本报告形成时，该计划必须保持“待确认”，直到用户在计划报告后的后续消息明确确认；该门禁后来已完成，当前状态以计划及实施证据为准。
