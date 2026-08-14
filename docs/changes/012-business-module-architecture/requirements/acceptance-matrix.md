# 需求与验收矩阵

## 0. 实施结论

下列表格第 1 至 6 节保留 `R017/R020` 形成方案时的逐项基线，避免改写历史缺口；当前关闭证据以 [R021](../research/R021-foundation-closure-implementation/report.md) 为准。

| 验收组 | 当前结论 | 主要可执行证据 |
|---|---|---|
| `AC-CLI-*` | 通过 | Bootstrap no-resource、完整树冲突、冻结、I/O/context/positionals、0/1/2/3/130 |
| `AC-DEF-*` | 通过 | strict round-trip、no-overwrite/force/symlink、取消与文件事务 fault injection |
| `AC-CFG-*` | 通过 | Source 身份/值域/merge、YAML/JSON duplicate、unknown/type、单候选与全 owner preflight |
| `AC-CMP-*` | 通过 | 显式 Plan、Bootstrap/Service 分离、单一 composition、注册冲突与 package graph |
| `AC-SUP-*` | 通过 | start/stop 顺序、runner error/nil、ready ack、不合作 runner、多错误与总期限 |
| `AC-HTTP-*` | 通过 | loopback bind failure、阻塞 Serve、active request drain、Stop/Wait 回归 |
| `AC-STATE-*` | 通过 | Coordinator/Supervisor diagnostics、Host Ready/Health、degraded gate |
| `AC-REL-*` | 通过 | candidate rollback、RestartRequired 无副作用、terminal no-resume、cleanup degraded |
| `AC-ERR-*` | 通过 | `errors.Is/As/Join`、安全 reload 日志、Snapshot/diagnostics 不暴露原始配置 |
| `AC-GOV-*` / `AC-EVO-*` | 通过 | 解析 package graph 的合法/违规 fixture、旧入口零残留、全量 test/race/vet/diff |

这表示 `AC-BIZ-GATE-001` 的基础闭环条件通过，不表示业务已设计或实现。

## 1. 十一项门禁矩阵

| 门禁 | 实施前基线 | 进入业务设计前的通过条件 |
|---|---|---|
| 事实 | 部分满足 | 每项结论可回到代码/测试/文档或版本化外部主源；目标不冒充实现 |
| 目标 | 部分满足 | 每个新增机制关联已复现问题和明确非目标，不为假想模块预埋 |
| 边界 | 部分满足 | Kernel、coordinator、Supervisor、HTTP、health owner 和依赖方向明确且无穿透 |
| 装配 | 部分满足 | Loader、candidate、constructor、runner、listener 和 cleanup 均从唯一 root 可追溯 |
| 生命周期 | 不满足 | start/ready/run/fail/drain/stop/wait/timeout 全部有事件序列证据 |
| 一致性 | 部分满足 | 全 owner 同一 candidate；无部分 commit、silent fallback 或无记录 degraded |
| 错误 | 部分满足 | primary/runtime/cancel/timeout/cleanup 全链保留，阶段和 owner 可识别 |
| 治理 | 不满足 | package graph、注册校验、生命周期/状态测试进入持续检查 |
| 演进 | 方向满足 | 单轨迁移、兼容影响、删除内容和失败回滚边界已验证 |
| 复杂度 | 推荐路线满足 | 不引入第二容器或无需求动态机制；每个抽象有删除/简化评估 |
| 业务延伸 | 不满足 | 以下全部基础 AC 通过且真实用例已确认 |

## 2. CLI、默认配置、Config 与装配验收

| ID | 验收项 | 必需证据 | 实施前基线 |
|---|---|---|---|
| `AC-CLI-001` | Bootstrap/ApplicationCommand/Service 模式在资源构造前显式选择；help/version/parse/config init 不构造或启动无关资源 | composition spy、黑盒进程测试 | 未实现 |
| `AC-CLI-002` | 完整 command path/name/alias/group/flag/shorthand 冲突在执行副作用前失败 | 完整树冲突表驱动测试 | 不完整 |
| `AC-CLI-003` | stdin/out/err、context、positional policy、missing/zero flag 和 side-effect class 语义明确 | I/O、nil、取消、flag matrix | 不完整 |
| `AC-CLI-004` | help/version/usage/command/config/cancel 分别走 0/1/2/3/130 的真实路径，cause 保留且只输出一次 | `runMain` 黑盒与 errors.Is/As | 不完整 |
| `AC-DEF-001` | 每个 section 的 defaults 与运行期同一 strict binder/semantic validator 回环一致 | generate/parse/bind/validate round-trip | 未实现 |
| `AC-DEF-002` | no-overwrite、显式 force、目标格式/绝对路径、0600/0700 和平台发布保证明确 | 文件系统/权限/平台测试 | 当前局部满足 |
| `AC-DEF-003` | 取消、短写、Sync/Close/publish/cleanup 失败不留错误目标/temp，主错误与清理错误保留 | fault injection、并发目标测试 | 当前局部满足 |
| `AC-DEF-004` | Secret/Token/password/key/DSN 使用安全空值或不可运行 placeholder，绝不读取真实环境凭据写入模板 | 生成物扫描、sensitivity tests | 当前局部满足 |
| `AC-CFG-001` | Source 名称非空唯一、顺序/优先级确定、canonical value domain、取消与 merge type conflict 明确 | Source 构造/覆盖/取消/值域测试 | 不完整 |
| `AC-CFG-002` | unknown/duplicate/type/required/deprecated 字段在资源副作用前严格处理 | YAML/JSON/Env strict negative tests | 未实现 |
| `AC-CFG-003` | 每个字段 missing/zero/empty/null/disabled/default 由 owner 明确并测试 | 逐 section field matrix | 不完整 |
| `AC-CFG-004` | Snapshot 完整深拷贝、canonical digest、稳定 provenance、owner sensitivity 脱敏 | mutation/digest/provenance/redaction tests | 不完整 |
| `AC-CFG-005` | Start/Reload 每个候选只调用 Loader 一次，具有稳定 identity/digest | Loader spy、并发变更测试 | 未实现 |
| `AC-CFG-006` | Kernel 与 application owners 解码同一不可变 candidate | 组合测试、identity/digest 断言 | 未实现 |
| `AC-CFG-007` | 所有 immutable/RestartRequired 预检先于 Kernel 和外部副作用 | 失败注入与事件序列测试 | 未实现 |
| `AC-CFG-008` | 未知/application section 变化不会被 Kernel 单独提交 digest | snapshot/reload 回归测试 | 未实现 |
| `AC-CMP-001` | 依赖、配置、constructor、runner、resource owner 可从唯一 root 定位 | 组合快照与代码审阅 | 部分实现 |
| `AC-CMP-002` | Kernel Plan 只含底层资源，无业务对象/运行时 Resolver/扫描 | Plan 测试、符号与 import 门禁 | 当前 Kernel 满足 |
| `AC-CMP-003` | component/section/command/participant/runner 注册使用同源规范并在冻结前拒绝 nil/空/重复/路径重叠 | registry matrix | 不完整 |

## 3. 监督与生命周期验收

| ID | 验收项 | 必需证据 | 实施前基线 |
|---|---|---|---|
| `AC-SUP-001` | owner/runner ID 非空唯一、顺序确定，nil 注册失败 | 构造失败测试 | 未实现 |
| `AC-SUP-002` | 启动失败反序清理并保留主错误和全部 cleanup error | 事件序列与 errors.Is/As | 局部满足 |
| `AC-SUP-003` | Service runner error 或 nil 非预期完成均触发取消和失败退出 | 两类 runner 测试 | 未实现 |
| `AC-SUP-004` | signal/runtime failure 后先撤销 ready，再 cancel、反序 StopAndWait | 无 sleep 的确定性序列测试 | 未实现 |
| `AC-SUP-005` | shutdown deadline 覆盖 Stop 与 Wait；不合作 runner 可定位且不会造成无限等待 | fake runner 超时测试 | 未实现 |
| `AC-SUP-006` | one-shot CLI 的 nil 完成与 Service runner 的 nil 完成语义明确分离 | 模式表驱动测试 | 未实现 |
| `AC-SUP-007` | Kernel/owner 停止错误不会阻止后续 owner 尝试清理，全部错误可识别 | 多错误注入测试 | 部分满足 |

## 4. HTTP 与状态验收

| ID | 验收项 | 必需证据 | 实施前基线 |
|---|---|---|---|
| `AC-HTTP-001` | listener 预绑定；端口占用/非法地址在 Start 同步失败 | 真实 loopback 端口测试 | 未实现 |
| `AC-HTTP-002` | Serve 是受监督 runner，异常退出触发 process failure | listener/Serve 失败注入 | 未实现 |
| `AC-HTTP-003` | Shutdown/Close/Wait 在期限内完成并等待活跃请求 | 阻塞请求、取消、超时测试 | 未实现 |
| `AC-HTTP-004` | 不存在“先 Wait Serve、后调用 Shutdown”的生命周期互锁 | 事件序列/超时回归测试 | 未实现 |
| `AC-STATE-001` | state 至少区分 starting、ready/running、draining、stopped、failed/degraded | 并发安全状态转换测试 | 未实现 |
| `AC-STATE-002` | ready 在全部必需单元运行后才 true，drain/failure 时先 false | probe 与事件序列测试 | 未实现 |
| `AC-STATE-003` | diagnostics 暴露 generation/digest/last reload-cleanup/owner，且脱敏 | 快照、并发与敏感信息测试 | 未实现 |
| `AC-STATE-004` | Kernel candidate Ready 与 process readiness 在接口/文档中不混淆 | API/文档/测试审阅 | 未实现 |

## 5. Reload、终止与错误验收

| ID | 验收项 | 必需证据 | 实施前基线 |
|---|---|---|---|
| `AC-REL-001` | candidate/prepare/drain 失败保持旧 generation 和旧 snapshot | Kernel 现有测试 + coordinator 组合测试 | Kernel 内满足 |
| `AC-REL-002` | reload drain 超时可 Resume；process termination drain 超时不恢复 serving/ready | 两种路径对照测试 | 未实现 |
| `AC-REL-003` | committed cleanup failure 保持新代、进入 degraded/restart-required 并阻断后续 reload | 状态与二次 reload 测试 | 未实现 |
| `AC-REL-004` | 每代资源只有唯一 owner，成功/失败路径不重复 Close 或丢失责任 | owner 表、计数与 race 测试 | Kernel 内部分满足 |
| `AC-ERR-001` | primary/runtime/cancel/timeout/drain/stop/cleanup error 保留 cause、phase、owner | errors.Is/As 与 join 测试 | 部分满足 |
| `AC-ERR-002` | 只有 process/presentation 策略边界记录一次，错误/诊断不泄密 | 日志 capture 与 redaction 测试 | 未实现 |

## 6. 治理与演进验收

| ID | 验收项 | 必需证据 | 实施前基线 |
|---|---|---|---|
| `AC-GOV-001` | 基于解析 package graph 验证禁止依赖和唯一 composition 边界 | 违规/合法 fixtures | 未实现 |
| `AC-GOV-002` | 生产与测试共用 ID/path/command 规范化和冲突规则 | 表驱动测试 | 未实现 |
| `AC-GOV-003` | 无全局 Registry、隐藏 fallback、fire-and-forget goroutine 或第二资源客户端 | 静态门禁与调用方搜索 | 部分满足 |
| `AC-GOV-004` | 生命周期测试不使用 sleep/环境 bypass，race 与泄漏检查按风险执行 | 测试审阅与命令证据 | 未实现 |
| `AC-EVO-001` | 新入口单轨替换旧入口，兼容影响和删除搜索完成 | 完整 Diff、旧符号搜索 | 待实施 |
| `AC-EVO-002` | 当前主题文档只描述实现，012 保留历史证据且不成为第二规范 | 链接/状态/术语检查 | 待实施 |

## 7. 业务延伸门禁

`AC-BIZ-GATE-001`：**已通过**。上述基础组已由 R021 的代码、测试、运行与治理证据关闭，允许在后续独立方案中恢复业务模块详细设计。

之后仍需 `AC-BIZ-GATE-002`：**阻塞**。首个真实用例的 actor、业务不变量、数据/事务 owner、必要入站边界和验收数据尚未得到独立确认。不得用假业务、空 CRUD、内存 Repository 或 TODO 解锁。

## 8. 方案阶段文档验收（历史）

方案阶段只允许确认 R001-R020 关系与文档质量，当时所有实现 AC 仍为未执行、不完整或局部事实。该历史限制不再描述 Round 6 实施结果；当前证据见 R021。
