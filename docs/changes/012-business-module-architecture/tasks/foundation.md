# FND：底层闭环实施任务

## 执行门禁

用户已在方案报告后的独立消息中确认实施。FND-001..010 均已完成并由 R021 汇总证据；本文件保留稳定任务 ID、验收条件和风险边界。工作量为相对估算：M 为局部跨包，L 触及入口/生命周期核心。

推荐实施阶段不是按 ID 数字排序：先执行 FND-008/009/010 的 CLI/Config 契约链，同时冻结 FND-001 状态语义；再执行 FND-002/003 与 FND-004/005 两条依赖链，最后由 FND-006/007 汇合。既有 ID 保持稳定，不因优先级校正重编号。

## FND-008：CLI registry 与 Bootstrap mode

- 状态：已完成（实现）。
- 工作量：L。
- 依赖：无。
- 目标：在资源构造前冻结完整 command registry，并显式选择 Bootstrap/Service；不创建无真实需求的 ApplicationCommand。
- 候选影响：`cmd/app`、`pkg/cli`、`internal/kernel/cli`、composition seam 与定向测试。
- 完成条件：
  - command path/name/alias/group/flag/shorthand 的 nil/空/重复/冲突在副作用前失败；
  - positional policy、context、stdin/out/err、missing/zero flag、mode 和 side-effect class 语义明确；
  - help/version/parse/config init 只组装 Bootstrap 需要的 registry/default manager，不构造或启动 DB/Cache/Storage/HTTP；
  - 0/1/2/3/130 退出路径由黑盒测试覆盖，错误只在进程边界输出一次；
  - Cobra 保持内部 Adapter，不形成第二套项目命令契约。
- 风险：当前 `len(args)` 分支和 `composition.Compose` 签名会受影响；公共 API 变化需在实施前确认。

## FND-009：严格 Section Config 与 Snapshot

- 状态：已完成（实现）。
- 工作量：L。
- 依赖：无。
- 目标：建立 section owner 的 path/default/bind/validate/change/sensitivity 最小 registration，收敛 Source 与 Snapshot 契约。
- 候选影响：`internal/kernel/config`、各 component Config/Stage、composition registration 与测试；具体 API 实施前确定。
- 完成条件：
  - Source 非空唯一、顺序/merge/type conflict/canonical value domain/cancel 可验证；
  - unknown/duplicate/type/decode hook/deprecated 在资源副作用前严格失败；
  - 每个字段 missing/zero/empty/null/disabled/default 由 owner 测试明确；
  - Snapshot 完整深拷贝、digest/provenance 确定、owner sensitivity 优先脱敏；
  - 不引入巨型 Config、无类型 Map 公共 API、动态 schema runtime 或隐藏 default。
- 风险：strict binding 可能暴露既有宽松配置；先盘点真实配置和调用方，有期限迁移才允许短期兼容。

## FND-010：默认配置同契约回环

- 状态：已完成（实现）。
- 工作量：M。
- 依赖：FND-008、FND-009。
- 目标：让生成、解析和运行期绑定复用同一 section contract，并收紧安全发布承诺。
- 候选影响：`internal/kernel/config/default*`、默认配置 CLI、各 owner defaults 与测试。
- 完成条件：
  - 全部 defaults 聚合后执行同一 strict binder/semantic validator，再 encode/parse/bind round-trip；
  - 无任何资源 probe/connection/listener；Secret/Token/password/key/DSN 只用安全空值或不可运行 placeholder；
  - no-overwrite、explicit force、0600/0700、取消、短写、Sync/Close/publish/cleanup 与并发目标测试闭合；
  - 平台原子性和 crash durability 只声明真实验证范围；失败结果说明目标是否可能已发布；
  - 生成结果包含绝对路径、format、replace 与 section IDs 的可验证信息。
- 风险：不同 OS/filesystem 的 publish 保证不同；未确认 force concurrency 和目录 fsync 前不扩大承诺。

## FND-001：生命周期状态与错误语义

- 状态：已完成（实现）。
- 工作量：M。
- 依赖：无。
- 目标：定义最小进程 state、termination cause、ready/degraded、owner/phase 和 diagnostics snapshot，不建立通用编排框架。
- 候选影响：application/Host 内部边界、项目自有错误/诊断类型及测试；具体路径实施前确定。
- 完成条件：
  - starting、ready/running、draining、stopped、failed/degraded 转换可测试且并发安全；
  - Kernel candidate Ready 与 process readiness 分离；
  - primary/runtime/stop/wait/cleanup error 保留 owner、phase 和 cause；
  - diagnostics 只输出安全 ID/digest/classification。
- 风险：状态机过度泛化；只保留 FND 真实消费者需要的状态。

## FND-002：Supervisor 运行监督闭环

- 状态：已完成（实现）。
- 工作量：L。
- 依赖：FND-001。
- 目标：区分 startup owner、blocking runner 和 one-shot；修复 Wait-before-Stop、runtime failure、nil 提前完成和不合作 runner 问题。
- 候选影响：`pkg/supervisor`、`internal/kernel/host.go` 及定向测试。
- 完成条件：
  - nil/空名/重复名在启动前失败，顺序确定；
  - Service runner error 或非预期 nil 完成触发 ready false 和全局取消；
  - shutdown deadline 覆盖 cancel、反序 Stop 与 Wait，不先无限等待；
  - 不合作 runner 超时可定位，后续可安全 cleanup 仍尝试；
  - 全部错误合并，事件序列测试不用 sleep。
- 风险：现有 Watch/Host 调用方兼容；发现外部消费者或公共接口实质变化时暂停并重新确认。

## FND-003：HTTP lifecycle 单轨接入

- 状态：已完成（实现）。
- 工作量：L。
- 依赖：FND-002。
- 目标：唯一 HTTP owner 预绑定 listener、运行阻塞 Serve、上报 runtime failure并 StopAndWait；不加入业务路由。
- 候选影响：`pkg/httpx`、application composition/Host seam 与测试。
- 完成条件：
  - 端口冲突/非法地址在 Start 同步返回；
  - 非正常 Serve exit 触发 process failure；
  - Shutdown/Close/Wait 有期限并等待活跃请求；
  - HTTP 最后 ready、终止时先停止接单；无 Wait-before-Stop 互锁；
  - RequestID/AccessLog 必需依赖显式注入，不使用隐藏 UUID/system-time fallback。
- 风险：旧 `Server.Start` 消费者和 hijacked connection；无真实需求不预建 WebSocket owner。

## FND-004：单一 immutable candidate

- 状态：已完成（实现）。
- 工作量：L。
- 依赖：FND-001、FND-009、FND-010；可与 FND-002 设计并行，改动入口时串行。
- 目标：coordinator 成为 Loader 唯一调用者，同一 candidate 供应 Kernel 与 application owners。
- 候选影响：`cmd/app`、Kernel Start/Reload 输入、配置协调与测试。
- 完成条件：
  - Start/Reload 一次 Load，candidate identity/digest 稳定；
  - 各 owner 独立 decode/validate，coordinator 不保存敏感大 Config；
  - application immutable 变化在 Kernel/外部副作用前返回 RestartRequired；
  - 未知/application section 变化不被 Kernel 单独提交；
  - 旧 Kernel 自行 Load 入口完成迁移并删除。
- 风险：Kernel API/测试影响面大；公共契约变化需重新确认。

## FND-005：Reload、terminal drain 与 cleanup degraded

- 状态：已完成（实现）。
- 工作量：L。
- 依赖：FND-001、FND-004。
- 目标：明确全应用候选提交、reload 可回滚排空、进程终止排空和提交后清理失败策略。
- 完成条件：
  - candidate/prepare/reload drain failure 保持旧 generation/candidate；
  - process termination 先 not-ready/拒绝新 work，drain timeout 不 Resume；
  - committed cleanup error 保持新代，标 degraded/restart-required 并阻断二次 reload；
  - 每代 owner 唯一，不重复 Close，不无记录丢失清理责任；
  - 不引入 NativeAtomic/Handoff/自动观测回滚占位机制。
- 风险：不同资源 Close 可重试性不同；默认重启而非盲重试。

## FND-006：Application composition 与运行模式

- 状态：已完成（实现）。
- 工作量：L。
- 依赖：FND-002、FND-003、FND-004、FND-005、FND-008、FND-010。
- 目标：建立唯一进程 composition，连接 Kernel、Watcher、HTTP owner、Supervisor 和模式；不创建业务模块 SDK。
- 完成条件：
  - 构造纯内存且依赖/owner 可追踪，无扫描、Registry、Resolver、`any`；
  - Service/Bootstrap CLI/未来 Application CLI 的完成语义显式区分；
  - help/未知命令/Bootstrap 不无谓启动 Kernel/HTTP；
  - 新旧入口单轨迁移、旧路径删除；
  - 当前默认 Service mode 有可复核运行/失败/停止证据。
- 风险：没有真实 Application CLI 时不实现占位命令。

## FND-007：Readiness、Health 与诊断接入

- 状态：已完成（实现）。
- 工作量：M-L。
- 依赖：FND-001、FND-003、FND-005、FND-006。
- 目标：将 lifecycle state、必需 checker 和安全 diagnostics 接入实际进程；复用 `pkg/health` 原语而非引入新框架。
- 完成条件：
  - ready 只在 Kernel/runner/listener 全部满足后 true，drain/failure 时先 false；
  - liveness/readiness/startup 语义不混淆，checker 有界且结果确定；
  - generation/digest/last reload-cleanup/degraded/owner 可诊断且脱敏；
  - 暴露方式有真实运维需求；若没有管理端点授权，至少提供 application seam 和测试证据，不虚构公网 API。
- 风险：管理端口/业务端口选择尚未确认，不能自行产生外部协议。

## 基础批次验证

- 逐项关闭 `AC-CLI-*`、`AC-DEF-*`、`AC-CFG-*`、`AC-CMP-*`、`AC-SUP-*`、`AC-HTTP-*`、`AC-STATE-*`、`AC-REL-*`、`AC-ERR-*`。
- 定向 lifecycle 测试、race、loopback HTTP、现有 Kernel/Host/Watch 回归和完整 Go 测试按实际改动执行。
- `git diff --check`、完整 Diff、旧符号/入口/配置/依赖搜索通过；未执行项如实记录。
- FND 与 GOV-F 已通过；仍不能进入业务模块，必须先确认真实用例并重新提交业务方案。
