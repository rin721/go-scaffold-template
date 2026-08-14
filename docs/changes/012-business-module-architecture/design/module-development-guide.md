# 新模块开发黄金路径

> 当前状态：**阻塞，不可执行**。只有基础闭环全部通过、权威文档同步、`AC-BIZ-GATE-001` 解除且首个真实用例获得独立确认后，才可使用本指南。本文不再作为当前实施顺序。

## 1. 使用前提

本指南描述目标路径，当前尚未实现其公共 contribution 与 application composition。只有 012 获得确认、基础任务完成并且首个真实业务用例明确后，才能按步骤执行。不得使用虚构 `User`、`Order` 或空 CRUD 作为架构验收。

## 2. 第一步：写清真实用例

在写目录或接口前记录：

- 业务能力名称与所有者；
- actor、触发条件、成功结果和业务不变量；
- 输入缺失、重复、冲突、取消和依赖失败的语义；
- 是否需要 HTTP、Application CLI 或二者；
- 数据所有权、事务原子性和一致性范围；
- 是否有可量化理由使用 Cache；
- 需要公开的错误 reason 和本地化消息；
- 验收示例与明确非目标。

若不能回答这些问题，保持在需求阶段，不预建模块骨架。

## 3. 第二步：确定边界与模型

1. 以业务能力命名 `internal/module/<capability>`；这里的 Module 是应用组合根选择的纵向模块，不是 Go module 或 Kernel Component。
2. 只有存在独立不变量时建立 `domain`；定义实体/值对象及其构造规则。
3. 在 `application` 定义用例 Command/Query/Result 和最小 Repository/跨模块 port。
4. 为输入、缺失、冲突和依赖错误定义稳定分类；不写本地化文本。
5. 画出允许 import 和跨模块调用，提前排除循环。

完成条件：Service 可仅用 fake port、固定 Clock/ID 在内存中进行单元测试，不导入 Kernel 或协议/数据库包。

## 4. 第三步：实现 Application Service

- 构造函数接收每个明确依赖，创建时校验必需依赖非空。
- 每个方法接收 `context.Context` 并及时传播取消。
- 业务规则和用例编排在这里，协议解析与数据库查询细节不在这里。
- 多 Repository 原子性由用例专用 UnitOfWork port 表达。
- 失败保留 cause 和稳定分类；不在此重复记录最终日志。

优先完成成功、输入错误、业务冲突、依赖失败、超时和取消测试，再接 Adapter。

## 5. 第四步：实现基础设施 Adapter

### Database

1. 在 `adapter/database` 定义 persistence Record 和显式转换。
2. 实现 application-owned Repository port。
3. 只保存稳定 Database Access；每个操作在 `Use`/`WithinTx` 回调内完成。
4. 用合约测试验证 SQL/ORM 错误转换、未找到、取消、回滚和 borrowed object 不逃逸。

### Cache（有真实必要时）

1. 确认 key、TTL、版本与失效所有者。
2. 以 Decorator/显式协作者接入，不改变 Service 接口语义。
3. 明确 unavailable 时失败还是可观测降级。
4. 把 typed Client Close 放入模块 owner 生命周期。

没有证据时跳过 Cache，不创建占位 Adapter。

## 6. 第五步：实现入站 Adapter

### HTTP

- 定义请求/响应 DTO 及转换。
- Handler 只解析/校验/提取可信上下文、调用 Service、映射响应。
- 定义模块路由和策略 Middleware contribution。
- 为错误映射定义 namespaced message ID，并显式接入 I18n 资源。

### CLI

- 定义参数与输出 View，调用同一 Service。
- 将命令明确分类为 Bootstrap 或 Application；业务命令通常是 Application。
- 不通过 HTTP 回环或调用 Handler。

只实现真实验收需要的入口，不为了目录完整同时制造 HTTP 与 CLI。

## 7. 第六步：模块局部装配

模块构造函数接收：

- 模块实际需要的最小 Capability/项目契约；
- 配置协调者已经解码并校验的不可变模块 Config；
- composition root 显式传入的跨模块 port。

它返回已绑定的 Service/Route/Command/Participant/Cleanup contribution。构造过程纯内存、确定、无 goroutine、无资源探测。重复 route/command/module ID 由集中 validator 在监听前拒绝。

## 8. 第七步：接入 composition 与 Host

1. 在唯一 composition root 显式选择模块和实现。
2. 把 Kernel stable facade 适配成最小业务依赖。
3. 连接跨模块 port，确认无循环。
4. 合并并验证 contribution。
5. 把模块 Participant 放在 Kernel 后、HTTP 前。
6. 将 HTTP route 安装到唯一 Router；Application CLI 作为 one-shot 受管运行。

应用模块不得自行修改 `cmd/app`、创建 Server 或追加全局 Registry。

## 9. 第八步：验证与文档同步

最低证据集：

- Domain/Application 单元测试；
- Repository/Cache Adapter 合约测试；
- HTTP/CLI 边界测试；
- route/command 冲突与 import 架构测试；
- 启动、失败回滚、取消、停止和资源泄漏测试；
- 真实垂直切片验收；
- 当前主题文档、配置说明和运行说明同步；
- 搜索确认没有旧入口、旧配置、重复实现或失效说明。

实施交付必须区分已通过、未执行和被外部环境阻断的验证；不能用“构建成功”代替产品验收。

## 10. Review 清单

- [ ] 模块名表达业务能力，目录没有空层或横向杂物包。
- [ ] Service 不依赖 Kernel、协议或第三方基础设施。
- [ ] Repository port 由使用方定义，borrowed resource 没有逃逸。
- [ ] HTTP/CLI 只做边界转换并复用同一 Service。
- [ ] 错误链、取消、超时、清理错误和敏感信息规则已验证。
- [ ] 配置来自单一快照，影响对象图的变化返回 `RestartRequired`。
- [ ] 每个 listener、goroutine、client 和 cleanup 有唯一 owner。
- [ ] 新旧实现没有无理由双轨，文档没有把目标写成现状。
