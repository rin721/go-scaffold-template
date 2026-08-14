# GOV：治理、文档与验证任务

## GOV-F-001：当前边界与唯一装配门禁

- 工作量：M。
- 依赖：FND-004、FND-006、FND-008、FND-009 确定真实路径。
- 状态：已完成（实现）。
- 完成条件：基于解析后的 package graph 检查 Kernel/composition/application/http owner 的允许/禁止方向；唯一资源构造位置可验证；每条规则有违规和合法 fixture，不靠 grep/宽泛正则。

## GOV-F-002：注册与生命周期门禁

- 工作量：M。
- 依赖：FND-002、FND-003、FND-005、FND-008、FND-009、FND-010。
- 状态：已完成（实现）。
- 完成条件：空/重复 command/section/owner/runner、CLI path/flag conflict、unknown/duplicate config、default round-trip、端口占用、startup failure、runner error/nil、signal、不合作 runner、terminal drain、committed cleanup degraded 和多错误合并均有确定性测试；生产与测试共用规范化规则。

## GOV-F-003：基础闭环运行与权威文档

- 工作量：M-L。
- 依赖：FND 全部完成。
- 状态：已完成（实现）。
- 完成条件：
  - 默认 Service mode 的 listener、ready、reload、drain、stop 有可复核运行证据；
  - Bootstrap help/version/config init 的 no-resource、I/O、退出码和默认生成物有可复核证据；
  - 根入口到配置、运行、生命周期、诊断和验证文档可达；
  - 权威文档只描述真实行为，012 保留历史证据且明确已实施/未实施；
  - 旧入口、旧配置、旧测试和冲突说明删除；
  - 未运行的远程/Docker/产品门禁明确标注。

## GOV-F-004：基础业务延伸门禁审计

- 工作量：M。
- 依赖：GOV-F-001..003。
- 状态：已完成（审计）；`AC-BIZ-GATE-001` 基础条件通过，`AC-BIZ-GATE-002` 继续阻塞。
- 完成条件：
  - acceptance matrix 的基础 AC 每项有证据或经确认的不适用理由；
  - 十一门禁重新评估，事实/目标/边界/装配/生命周期/一致性/错误/治理/演进/复杂度均通过；
  - `AC-BIZ-GATE-001` 明确解锁或继续阻塞，不模糊处理；
  - 完整测试、静态检查、race/集成、Diff、旧符号和 Git 范围审阅完成。

## GOV-B：业务门禁（当前阻塞）

基础闭环和真实用例确认后，再细化：

- 业务 domain/application 不导入 Kernel、HTTP/CLI、ORM/Cache 第三方包；
- 模块不穿透其他模块 Adapter/Record/Repository；
- route/command/业务 contribution 冲突；
- Service/Adapter/入站边界契约与真实垂直切片验收。

禁止在业务包尚不存在时为假想目录写永久正则或生成器。

## 检查设计原则

- 检查真实 import graph 和生产规范化函数，不把目录名或文本搜索当全部证明。
- 失败测试必须能证明规则实际拦截，合法样例防止过度限制。
- lifecycle 测试用 channel/barrier/受控 fake，不用 sleep、环境变量 bypass 或隐藏 fallback。
- Health 通过不替代 lifecycle/ownership 证据，build 通过不替代运行/产品验收。

## 研究刷新

R021 已替代 R017/R020，记录实现快照和门禁证据；历史记录未被改写。业务首个切片完成后再复核 R002-R009 的业务适用性。研究更新必须同步当前主题文档，不能让 012 成为第二套现行规范。
