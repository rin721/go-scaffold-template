# 迁移、风险、决策与未决项

## 1. 决策记录

下列决策均为 **012 待确认方案**，尚未成为当前实现：

| ID | 决策 | 理由 | 被拒绝的当前替代 |
|---|---|---|---|
| D-012-01 | Kernel 资源平面与业务对象图平面分离 | 保留现有配置/生命周期价值，避免普通对象进入运行时容器 | 所有业务对象塞入 Kernel Plan |
| D-012-02 | 唯一 composition root 手工装配 | 依赖、顺序和 owner 可直接从代码审查 | runtime DI、扫描、`init` Registry、Service Locator |
| D-012-03 | 按业务能力纵向组织 | 业务所有权和变更局部性清晰 | 全局 handlers/services/models/repositories |
| D-012-04 | 使用方定义 port，Adapter 向内 | 稳定业务规则不依赖易变技术 | 业务直接依赖 GORM/Redis/HTTP 类型 |
| D-012-05 | typed contribution 只包含完成品 | 支持集中验证但不演变为 DI 语言 | Provider 列表、`map[string]any`、字符串查找 |
| D-012-06 | 启动共享单一不可变配置快照 | 防止 Kernel 与业务图撕裂读取 | 两边各自调用 Loader |
| D-012-07 | 首版业务图/路由/命令配置变化需重启 | 保持重载事务边界可证明 | 业务对象热重建和动态路由 |
| D-012-08 | HTTP/CLI 复用 Service | 协议与业务规则分离 | CLI 调 Handler 或 HTTP 回环 |
| D-012-09 | 事务由用例显式定义 | 保持入口无关和错误语义 | HTTP Middleware/context 隐式事务 |
| D-012-10 | 当前不建设动态插件/分布式运行时 | 没有真实隔离、第三方扩展或远程需求 | Encore/Dapr/go-plugin 式运行模型 |

如后续改变公共接口、依赖、模块边界、配置迁移或运行副作用，应新增/更新任务方案；难以逆转的长期选择需进入 ADR，而不是静默改写本表。

## 2. 分阶段迁移

### Phase A：基础装配能力

- 把 Loader 所有权上移为单一 config coordinator，并让 Kernel 接受外部候选快照。
- 定义最小 typed contribution 与冲突 validator。
- 建立 application composition 和显式运行模式。
- 改造/封装 HTTP Server 为可同步报告监听失败的 Participant。
- 保持现有 Bootstrap `config init` 不打开资源。

此阶段只建立真实垂直切片必需的薄能力，不能预先引入 Module SDK、代码生成器或插件系统。

### Phase B：首个真实垂直切片

- 根据确认的业务需求建立第一个纵向模块。
- 实现 Domain/Application、Repository Adapter 和一个必要入站入口。
- 只在验收确有需要时加入 Cache、CLI 或跨模块协作。
- 用真实 Database/HTTP 或明确可接受的外部消费者完成端到端证据。

### Phase C：治理与推广

- 把首个切片中经验证的规则固化为 import、冲突和生命周期测试。
- 同步根运行说明、架构与模块开发主题文档。
- 第二个真实模块证明路径可复用后，再评估生成工具；不得由第一份设计直接推导生成器需求。

## 3. 主要风险与缓解

| 风险 | 影响 | 缓解与证据 |
|---|---|---|
| config coordinator 改变 Kernel API | 触及启动与 reload 核心语义 | 小步设计、候选事务测试、旧 Generation 保留测试 |
| contribution 演变为隐藏 DI | 依赖和 owner 重新隐式化 | 只允许完成品、禁止 Provider/反射/`any`、API review |
| composition root 过大 | 连接逻辑难维护 | 模块局部纯装配、root 只选实现/合并/验证 |
| HTTP listener 改造引入竞态 | Start 假成功或 Stop 泄漏 | 预绑定、done channel、异常退出和 race/lifecycle 测试 |
| Module Service 形成巨型接口 | 跨模块耦合与测试成本上升 | 调用方定义窄 port，按用例拆分方法 |
| Domain/DTO/Record 转换样板增加 | 开发者绕过边界 | 提供明确示例与合约测试；第二模块后再评估生成 |
| Cache 降级掩盖错误 | 返回陈旧/错误业务结果 | 每个用例明确语义、可观测降级、故障测试 |
| I18n 资源聚合方式未定 | 模块消息无法可靠加载/reload | 首模块前确认显式文件或 embed 方案 |
| 设计覆盖过多未来场景 | 过度工程化 | 只实施已确认任务和真实入口，非目标保持删除态 |
| 文档领先实现 | 使用者误判现状 | 所有目标文件标待确认，实施后同步权威文档 |

## 4. 当前未决项

以下问题会实质改变实施，不得由 Agent 自行假设：

1. 首个真实业务能力、用例、actor 与验收数据是什么？
2. 首个切片需要 HTTP、Application CLI，还是二者？
3. 首个用例的数据所有权、表/外部服务和事务边界是什么？
4. 是否有量化证据需要 Cache；不可用时允许怎样的业务行为？
5. application/module 配置节和 schema 由谁拥有，哪些字段影响对象图？
6. I18n 消息资源继续使用显式文件路径，还是引入可审计 embed 聚合？
7. HTTP 公共错误 schema、状态码和版本策略是什么？
8. 首个切片是否需要 trace/metrics；对应 SLO 和 exporter 生命周期是什么？
9. 当前 RequestID fallback、DefaultErrorHandler 和阻塞 Server API 是直接替换还是存在外部消费者迁移需求？

## 5. 明确拒绝项

当前方案不保留以下“以后也许有用”的兼容层：

- 业务运行时 Container/Resolver；
- 自动扫描模块、路由、命令或 I18n 文件；
- 全局 `ServiceContext`/Capabilities 注入到每个 Handler；
- 模块共享数据库 Record/Repository；
- 通用事件转发绕过同步依赖；
- 进程外插件协议或 sidecar；
- 新旧启动路径长期双轨；
- 空业务模块、占位 Handler、TODO Repository。

真实需求若出现，必须带证据重新进入方案/ADR，而不是预埋关闭的 Feature Flag。

## 6. 回滚与兼容原则

实施应在一个确认任务内完成调用方迁移并删除失效入口，不留下 `legacy`、`old` 或静默 fallback。若尚无外部消费者，优先直接单轨替换。若发现真实外部兼容要求，应暂停并补充兼容范围、责任人、截止条件、观测指标和删除计划，再重新确认。

数据库数据迁移、外部协议兼容和用户数据不因本架构任务自动获得修改授权；它们必须由首个真实业务任务单独设计。
