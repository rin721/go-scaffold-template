# FND：基础装配任务

## 执行门禁

本文件所有任务均为 **待确认**。只有用户在本方案报告后明确确认 012，且任务的公共接口/迁移未发生实质变化，才允许实施。每项工作量为相对估算：S 不超过一个局部单元，M 跨少量包，L 触及核心启动/生命周期并需要系统测试。

## FND-001：单一配置快照协调

- 工作量：L。
- 依赖：无。
- 目标：让 application config coordinator 成为 Loader 唯一调用者，同一不可变候选同时供应 Kernel 和业务/HTTP 解码。
- 文件影响候选：`cmd/app`、`internal/kernel` 配置入口及相关测试；具体路径以实施前代码为准。
- 完成条件：
  - 启动只调用 Loader 一次；
  - Kernel 能从显式候选启动/reload，不再私自二次读取；
  - application-owned 配置变化在 Kernel candidate transaction 前返回 `RestartRequired`；
  - 旧 Generation 在失败时保持可用，错误链完整；
  - 当前配置文档同步。
- 风险：Kernel API 与 reload 事务核心变化；若候选所有权或公共接口与设计不同，返回待确认。

## FND-002：Typed contribution 与集中校验

- 工作量：M。
- 依赖：FND-001 的配置所有权方向已稳定。
- 目标：定义最小 Module/Route/Command/Participant/Cleanup 完成品契约和 validator，不建立 Provider/Container。
- 完成条件：
  - Module ID、method+normalized path、CLI 完整路径/alias、Participant 名称重复均在监听前失败；
  - contribution 顺序确定且测试不依赖 map/group 随机顺序；
  - API 不含 `map[string]any`、反射、字符串依赖查找或运行时 constructor；
  - 模块可脱离 composition 独立构造测试。
- 风险：契约过早泛化；必须以 HTTP participant 和首个真实模块的最小需求收敛字段。

## FND-003：HTTP Server Participant

- 工作量：L。
- 依赖：FND-001；与 FND-002 的 Route 安装契约协调。
- 目标：让唯一 HTTP Server 在 Host 下同步预绑定 listener、受管 serve、报告异常退出并优雅停止。
- 文件影响候选：`pkg/httpx`、application composition/Host 接入与测试。
- 完成条件：
  - 端口冲突在 `Start` 同步返回；
  - serve goroutine 有 owner、取消、done 与异常上报；
  - Stop 有超时、等待退出并合并 shutdown/close 错误；
  - HTTP 最后启动、最先停止；
  - RequestID/AccessLog 改用注入 ID/Clock，删除隐藏 fallback；
  - 外部消费者若存在，其迁移边界已确认，否则直接单轨替换。
- 风险：监听器 API、Supervisor 任务模型和兼容影响需先代码取证。

## FND-004：Application composition 与运行模式

- 工作量：L。
- 依赖：FND-001、FND-002、FND-003。
- 目标：建立唯一 composition root，显式构造 Kernel、模块、Router、Host，并使用 enum 区分 Service/Bootstrap CLI/Application CLI。
- 完成条件：
  - 构造无 I/O、无 goroutine、无扫描/`init` Registry；
  - Kernel Plan 不包含普通业务对象；
  - Participant 顺序和失败回滚有测试；
  - 未知命令/help 不无谓启动资源；
  - 旧启动入口完成迁移并删除，无长期双轨。
- 风险：入口重构范围大；一旦运行模式或公共 CLI 兼容要求变化需重新确认。

## FND-005：Bootstrap 与 Application CLI

- 工作量：M。
- 依赖：FND-004。
- 目标：保持当前 `config init` 的 Bootstrap 语义，建立需要业务 Capability 的 one-shot Application Command 路径。
- 完成条件：
  - Bootstrap 命令不构造/启动 Kernel Database、Cache 或 HTTP；
  - Application 命令在 Kernel/必要模块就绪后运行同一 Service，完成后反向停止；
  - 参数错误、取消、业务失败和输出映射可测试；
  - Command 冲突集中校验。
- 风险：首个真实业务未必需要 CLI；若没有需求，只实现运行模式基础，不制造占位命令。

## 基础批次验证

- `go test`、race/lifecycle 定向测试和仓库已有静态检查全部通过；具体命令以当时工具链为准。
- 端口占用、构造失败、Module Start 失败、serve 异常、Stop 错误和 reload restart-required 均有失败路径证据。
- `git diff --check`、完整 Diff 审阅、旧符号/路径/配置搜索通过。
- 任何未执行门禁必须如实标为未验证，不能用其他检查替代。
