# Config、Default、Binding、Validation、Snapshot 与 Reload 契约

## 1. 结论

实施前 Config 已具备显式有序 Source、File/Env 覆盖、不可变 Snapshot 意图、摘要/来源、分节 typed Config、owner defaults 和 Kernel candidate transaction，这些方向已被保留。

实施通过配置节 owner 的最小 `config.Binding` 闭合 Source 值域与身份、Default 与 runtime binder、strict binding、validation、Snapshot 不可变性以及 Application 与 Kernel 的同一 reload candidate；没有引入无类型公共 Map API、动态 schema registry、隐藏 Provider 或第二套 reload runtime。

状态：**已实施设计记录**。基础实施快照见 [R021](../research/R021-foundation-closure-implementation/report.md)；EnvSource/Loader 的同源与跨源路径确定性由 [022 FOUNDATION-CONFIG-001](../../022-http-api-template-readiness/plans/foundation-config-001.md) 补齐。实施前现状见 [R017](../research/R017-current-contract-inventory/report.md)，外部研究见 [R019](../research/R019-config-contracts/report.md)。

## 2. 实施前数据流与责任

```text
cmd/app
  -> FileSource(path)
  -> EnvSource(prefix)              # 后加载，覆盖 File
  -> Loader.Load(ctx)
     -> recursive map merge
     -> Snapshot(raw, redacted, digest, provenance)
  -> Kernel.Start/Reload
     -> Snapshot.Section(path)
     -> component.Stage
        -> typed DefaultConfig 预填
        -> mapstructure DecodeSection(WeaklyTypedInput=true)
        -> owner normalize/validate
     -> Build/Ready
     -> publish/swap

composition
  -> ordered DefaultBinding(section ID, path, DefaultContract)
  -> DefaultManager.Generate
     -> generic Object validation
     -> YAML/JSON encode
     -> safe file publish
```

实施前已确认问题：

- `DefaultContract` 生成的数据不会先经过运行期同一 typed decoder/semantic validator；目前一致性依赖各 owner 手工复用 `DefaultConfig()` 的惯例。
- `mapstructure` 未开启 `ErrorUnused`，未知字段被忽略；`WeaklyTypedInput` 允许空字符串到数值零、布尔/数值/字符串等宽松转换。
- YAML/JSON Source 没有项目级“重复字段和未知字段必须失败”的统一验收。
- 多个 Config 把数值 `0` 归一化为默认；logger 用 pointer bool 区分缺失/false；Cache/Storage 用枚举表达 disabled。语义存在但分散。
- Snapshot 深拷贝只覆盖常见 `map[string]any`/`[]any`；Source 契约允许其他 mutable value，无法证明任意 Source 下完全不可变。
- Kernel 自己调用 Loader。未来 Application-owned section 无法参加同一 preflight；若另行 Load 会产生两个时间点，若不处理又可能由 Kernel 单独更新整份 digest。

## 3. 语义所有权

| 责任 | 唯一 owner | 不属于它的责任 |
|---|---|---|
| Source 读取、顺序、merge、canonical tree、Snapshot | Config foundation | 业务字段默认值、资源连通性 |
| section path、typed Config、defaults、字段/跨字段校验、敏感字段、change class | 使用该配置的 capability/Adapter/module | 文件读取、全局 Source 优先级、资源关闭 |
| candidate 资源构造、probe、释放、换代 | Runtime component/resource owner | 修改 raw Snapshot 或隐藏 fallback |
| application 与 Kernel 候选协调、process state、restart decision | Application coordinator | 成为 DI Container 或复制 Kernel resource transaction |
| 默认配置命令的目标路径、格式、overwrite/publish | Bootstrap CLI/Default manager | 创建或启动运行期资源 |

配置节由“使用方语义”拥有。例如 Database section 由项目 Database capability/Adapter 拥有，而不是由 YAML 库或 GORM 类型拥有；第三方配置类型不得穿透公共契约。

## 4. 最小 Section Contract

概念模型如下；当前实现落在 `config.Binding`、`app.Configuration`、`config.ValidateCandidate` 与 `Coordinator`，具体签名以代码为准：

```text
SectionID               # 非空、稳定、全局唯一
ConfigPath              # 非空、无重叠冲突
Defaults(ctx)            # 只产生安全、确定、无资源副作用的值
BindStrict(section)      # 输出项目自有 typed Config
Validate(config)         # 字段与跨字段语义，无资源副作用
ClassifyChange(old,new)  # Unchanged/LiveReplace/RestartRequired/NotReloadable
Sensitivity              # 字段级 redaction/default safety 元数据
```

这可以由普通结构体、泛型辅助函数或少量函数值表达；不强制每个 Config 创建只有一个实现的 interface。只有需要跨 owner 聚合的行为进入公共契约。

### 4.1 注册冻结门禁

- SectionID、path 非空且唯一；path 不允许父子重叠所有权。
- Defaults/Bind/Validate/Classify/Sensitivity 的必需项完整；nil 不代表禁用。
- 注册顺序稳定；同一结果不依赖 map 遍历或包 init 顺序。
- 每个 section 的 typed Config、默认投影和 change classifier 来自同一 owner。
- 冻结成功后注册不可变；任何冲突在 Source I/O 或资源副作用前失败。

## 5. Source 与优先级契约

### 5.1 有序层

当前生产顺序固定为：

```text
typed owner defaults < configuration file < environment variables
```

其中 owner defaults 不应先转成完整 raw map 再与外部 Source 混成不可区分的数据；binding 时以 typed defaults 作为 missing 值基线，外部值覆盖对应字段。环境变量后加载，因此优先于文件。未来新增 remote/CLI override 必须先确认优先级与故障语义，不得通过注册先后偶然决定。

### 5.2 Source 输出值域

Source 只允许 canonical configuration tree：

```text
object: map[string]Value
array:  []Value
scalar: string | bool | signed/unsigned integer | decimal | null
```

不得包含 pointer、channel、func、第三方对象或共享 mutable slice/map。Loader 在边界规范化并完整复制；不支持的值在候选产生前失败。Source 必须有非空唯一 name，错误携带 source identity，且 `Load(ctx)` 对取消/超时可响应。

### 5.3 Merge 与冲突

- object 对 object 递归覆盖；scalar/array 由更高优先级整体替换。
- object 与 scalar/array 的类型冲突默认失败，不静默改形状；若真实兼容需求要求覆盖，必须逐 path 声明迁移规则。
- key 大小写、环境变量 `__` nesting、空环境值和 null 语义固定并测试。
- 同一 Source 内重复字段在解析阶段失败；不允许 YAML 转 JSON 时静默丢失早先值。
- provenance 至少记录 source 顺序与名称；字段级 provenance 是否需要，在真实诊断需求出现前不扩展。

## 6. Binding 与字段语义

### 6.1 Strict Binding

每个已注册 section 默认采用严格模式：

- 未知字段失败，并返回 section/path；
- 重复字段在 Source parser 阶段失败；
- 类型不匹配失败，不把任意 bool/number/string 相互转换；
- 只保留明确允许的 decode hook，例如字符串到 `time.Duration`，并为其定义格式与溢出错误；
- 输入中存在但 typed Config 未消费的字段失败；
- 绑定结果不保留 raw map 或第三方 decoder 对象。

当前 `mapstructure` 可以继续作为内部 Adapter，也可以替换；选型取决于能否实现上述行为和错误路径，不因“strict”要求而预先指定框架。

### 6.2 Missing、zero、empty、disabled、default

每个字段在 owner 文档和测试中选择一种明确语义：

| 输入状态 | 允许的含义 | 推荐表达 |
|---|---|---|
| missing | 应用 owner default，或必填错误 | typed default / required metadata |
| explicit zero | 合法业务值、禁用、或非法；不得自动等同 missing | 数值 + 明确 validation |
| empty string/list | 合法空集合/空文本、清空、或非法 | typed field + validation |
| disabled | capability 明确关闭 | enum/专用类型，不能只靠 nil |
| null | 明确清除/缺失，或禁止 | pointer/optional，仅有真实需要时 |
| default | owner 声明的确定值 | 单一 Default Contract |

当前把超时/池大小 `0` 当默认的行为只能在 owner 确认“显式 0 不具有独立含义”后保留；否则需迁移为 pointer/optional。不得全局规定所有零值语义相同。

### 6.3 未知、废弃与版本

- 未知字段默认错误，防止拼写错误和已删除配置悄悄失效。
- deprecated 字段只有在外部兼容确需短期双轨时允许；必须有 owner、替代字段、warning、截止版本/日期、迁移和删除测试。
- removed 字段按 unknown/error 处理，不静默忽略。
- 只有存在需要迁移的持久配置格式时才引入 schema version 和 converter；当前不为未来可能性建立版本框架。
- 单轨替换后默认生成、权威文档、binding 和 tests 同步删除旧字段。

## 7. Validation 阶段

```text
1. Source syntax/shape
   文件格式、重复字段、canonical value domain、merge type conflict

2. Structural binding
   section/path、unknown field、type、decode hook、required/missing

3. Semantic validation
   字段范围、enum、跨字段不变量、disabled 组合、placeholder policy

4. Change preflight
   与当前 typed config 比较，分类 change，不产生资源副作用

5. Resource build/probe
   创建候选、连接/监听、ping/readiness；owner 负责失败清理

6. Transaction commit
   所有 owner 准备成功后 drain/swap/publish，再清理旧代
```

阶段 1-4 必须无运行资源副作用；默认配置生成执行 1-3，不执行 4-6。取消、超时和 validation error 是不同类别。资源 probe error 不得伪装为结构配置错误。

## 8. Snapshot 契约

一个有效候选 Snapshot 必须满足：

- raw tree 已规范化且完整深拷贝；对外 accessor 每次返回独立副本；
- digest 基于确定的 canonical encoding，与 map iteration 无关；
- provenance 顺序稳定，能解释实际优先级；
- redacted snapshot 与 raw snapshot 同结构，但 owner sensitivity 优先、启发式 key 规则保底；
- 日志、errors、diagnostics、diff 永不输出 raw sensitive values；
- 同一次 startup/reload 的所有 owner 接收同一个 candidate ID/digest；
- 只有整个候选 commit 后才更新 current Snapshot；失败保持旧 Snapshot 和所有旧 owner 状态不变。

Snapshot 是配置事实，不是资源容器，不允许塞入连接、logger、client、closure 或 mutable runtime state。

## 9. Default 与运行期一致性

默认配置生成必须做回环验证：

```text
owner Defaults
  -> aggregate canonical tree
  -> same SectionContract.BindStrict
  -> same SectionContract.Validate
  -> encode file
  -> parser round-trip
  -> same bind + validate result
```

验收比较的是 normalized typed Config 或 owner 定义的 semantic equality，不要求 YAML 字节与内部 map 排序相同。任何 section 默认值无法被运行期读取、出现未知字段、丢字段或语义变化，都在触碰目标文件前失败。

默认模板可以省略“运行时会由 typed default 补齐”的字段，但省略策略也属于 owner contract；不能出现生成器认为省略、运行期 owner 又采用不同隐藏默认的漂移。

## 10. Reload Contract

### 10.1 变化分类

| 分类 | 含义 | 允许动作 |
|---|---|---|
| `Unchanged` | typed semantic equality 不变 | 不构造资源，不改变 generation |
| `LiveReplace` | 可通过现有 candidate transaction 无损换代 | prepare -> ready -> drain -> commit -> cleanup |
| `RestartRequired` | 当前进程不能安全换代 | 在任何 candidate 资源副作用前拒绝，旧代继续服务 |
| `NotReloadable` | 出于安全/一致性明确禁止运行期变化 | 返回专用错误和诊断；是否终止由进程策略决定 |

不得用 unknown/fallback 表示分类；未实现 classifier 的 section 默认 `RestartRequired`，并在 registration 中显式记录。

### 10.2 单候选协调

```text
Watcher/event
  -> Application coordinator 唯一 Loader.Load
  -> strict bind + validate all sections
  -> classify all changes
  -> 任一 RestartRequired/NotReloadable: 无副作用返回
  -> prepare Application + Kernel candidates
  -> all ready
  -> coordinated drain/commit/publish
  -> current snapshot/generation update once
  -> cleanup previous
```

协调者不复制 Kernel 的 component transaction，也不向业务暴露 registry。它只保证所有配置 owner 对同一候选达成预检和提交边界。Kernel 继续唯一拥有 DB/Cache/Storage/Logger 等底层资源换代。

### 10.3 失败不变量

- Source/bind/validate/classify/prepare/ready 失败：当前 Snapshot、Capabilities、业务状态和 readiness 不变。
- reload drain 失败：可 Rollback/Resume 旧代；恢复成功后 ready 可保持，错误和 last reload failure 可观察。
- commit 后旧代 cleanup 失败：新代仍 current，进程进入 `degraded`，记录 owner/generation；默认阻断后续 reload，等待重启或明确处置。
- terminal drain 失败：不得 Resume 或重新 ready；继续有界 best-effort stop/cleanup 并返回完整错误。
- reload 与 stop 互斥；stop 开始后新 reload 被拒绝。

## 11. 方案比较

| 方案 | 收益 | 代价与风险 | 结论 |
|---|---|---|---|
| 保持宽松 mapstructure 与分散 defaults | 改动少 | 拼写错误静默、零值漂移、生成与运行不一致 | 不满足 |
| 建立全局巨型 Config/schema/Map API | 入口集中 | 语义 owner 混乱、跨模块耦合、弱类型扩散 | 拒绝 |
| 引入第二套配置/DI/reload 框架 | 可能带成熟功能 | 与 Kernel candidate transaction 重复，迁移和双轨风险高 | 拒绝 |
| 按 section owner 补 registration、strict bind 和 application candidate coordinator | 保留现有强项，问题与契约一一对应 | 要迁移宽松字段语义并补大量负向测试 | 推荐 |

## 12. 验收门禁

- 所有 Source name 非空唯一、顺序明确、值域 canonical、完整响应取消。
- 文件/环境覆盖、object/scalar 冲突、case、empty/null 和 duplicate field 行为有表驱动测试。
- 每个 section 未知字段、类型错误、decode hook 格式错误必然失败并定位 source/section/path。
- 每个字段的 missing/zero/empty/disabled/default 有 owner 断言；不允许靠偶然零值。
- `config init` 生成物通过同一 strict binder/validator round-trip；无真实 secret。
- Snapshot accessor 对所有允许值类型都无法修改内部状态，digest/provenance 确定且 diagnostics 脱敏。
- startup/reload 中 Loader 恰好调用一次；Application 与 Kernel 看到同一 digest/generation。
- RestartRequired 在任何资源构造前失败；候选失败旧代不变；cleanup degraded 持久可见并阻断 reload。
- 配置字段删除/废弃时，默认生成、binding、文档和测试单轨同步，无永久兼容分支。

这些证据全部满足后，Config 只达到“可承载真实业务配置”的基础门禁；仍需 Supervisor、HTTP、Diagnostics 和治理门禁共同通过，才能继续业务模块详细设计。
