# FND：底层闭环实施任务

## 执行门禁

全部任务均为 **待确认**。用户本轮只要求研究和文档，以下文件影响只是方案候选；实施前必须复核真实调用方和 Git 状态。工作量为相对估算：M 为局部跨包，L 触及入口/生命周期核心。

## FND-001：生命周期状态与错误语义

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

- 工作量：L。
- 依赖：FND-001；可与 FND-002 设计并行，改动入口时串行。
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

- 工作量：L。
- 依赖：FND-002、FND-003、FND-004、FND-005。
- 目标：建立唯一进程 composition，连接 Kernel、Watcher、HTTP owner、Supervisor 和模式；不创建业务模块 SDK。
- 完成条件：
  - 构造纯内存且依赖/owner 可追踪，无扫描、Registry、Resolver、`any`；
  - Service/Bootstrap CLI/未来 Application CLI 的完成语义显式区分；
  - help/未知命令/Bootstrap 不无谓启动 Kernel/HTTP；
  - 新旧入口单轨迁移、旧路径删除；
  - 当前默认 Service mode 有可复核运行/失败/停止证据。
- 风险：没有真实 Application CLI 时不实现占位命令。

## FND-007：Readiness、Health 与诊断接入

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

- 逐项关闭 `AC-CFG-*`、`AC-CMP-*`、`AC-SUP-*`、`AC-HTTP-*`、`AC-STATE-*`、`AC-REL-*`、`AC-ERR-*`。
- 定向 lifecycle 测试、race、loopback HTTP、现有 Kernel/Host/Watch 回归和完整 Go 测试按实际改动执行。
- `git diff --check`、完整 Diff、旧符号/入口/配置/依赖搜索通过；未执行项如实记录。
- FND 通过仍不能进入业务模块；必须再完成 GOV-F 和权威文档同步。
