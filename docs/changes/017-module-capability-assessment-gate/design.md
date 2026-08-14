# 设计：应用模块能力评估门禁

## 1. 设计结论

本方案不改变 Kernel，也不在 `AGENTS.md` 复制项目架构。采用两级治理：

```text
AGENTS.md
  通用触发语境 + 必须遵循项目文档 + 缺口时停止推进
        ↓
README.md -> docs/README.md
        ↓
docs/development/application-module-development.md
  当前项目的能力评估、模块步骤、生命周期适配与升级规则
        ├─ internal/module/README.md      包职责与依赖方向
        ├─ pkg/README.md                  当前与暂缓 Capability 清单
        └─ internal/kernel/app/README.md  组件形态、API 与接入步骤
```

`docs/development/application-module-development.md` 是“如何新增应用模块”的唯一当前权威；三个包级 README 只拥有各自代码边界，不复制开发流程全文。设计由 [R001](research/R001-current-module-capability-assessment/report.md) 和需求 `AGT-001`、`NAV-001`、`CAP-001`、`LIFE-001`、`GAP-001` 支撑。

## 2. `AGENTS.md` 的通用边界

在研究阶段增加一条短规则，目标语义如下：

> 当任务新增业务或应用模块、通用能力或外部系统接入时，必须先遵循项目文档识别现有能力、依赖边界、资源所有权、生命周期与基础契约适配性；若项目没有可验证的接入路径，或现有契约无法表达真实需求，应停留在研究和计划阶段，先补齐项目级设计并获得相应确认。

最终措辞可以压缩，但必须保留：

- 触发条件；
- 按项目文档推进；
- 能力、资源、生命周期和契约适配四类问题；
- 缺少路径或契约不适用时停止实施。

不得写入：

- `internal/module`、`internal/kernel/app` 等项目目录；
- `app.Value`、`ManagedConfigured`、`KernelInstanceSwap` 等当前 API；
- NATS、Kafka、SMTP、Elasticsearch 等具体技术；
- 本项目检查表的完整字段或任务 ID。

这样 `AGENTS.md` 提供可跨项目复用的判断语境，具体执行随项目文档演进，不需要每次架构变化都重写全局规则。

## 3. 当前项目权威入口

新增 `docs/development/application-module-development.md`，内容按开发者实际动作组织：

1. 明确真实用例和外部副作用；
2. 盘点并分类所需能力；
3. 判断模块边界和依赖方向；
4. 判断资源 owner、运行单元和配置 owner；
5. 对照 Kernel App 形态选择治理方式；
6. 判断当前契约是否适用；
7. 形成研究结论和任务影响；
8. 获得确认后才实现模块。

导航改动：

- 根 README 的包入口增加“应用模块开发指南”；
- `docs/README.md` 的当前主题增加同一链接；
- `internal/module/README.md` 在“新增模块”处链接指南，并保留简短依赖方向；
- `internal/kernel/app/README.md` 在组件形态前增加反向链接：只有项目指南判定为进程选择的底层 Capability 后，才进入该组件接入步骤；
- `docs/research/README.md` 引用指南的必答评估模板。

## 4. 能力评估表

指南要求每个新模块填写以下表格。字段必须出现；不适用时写明原因。

| 维度 | 必答问题 | 输出 |
| --- | --- | --- |
| 用例 | 哪个 actor 触发什么行为，有哪些外部副作用 | 用例与非目标 |
| 现有能力 | `pkg` 和 composition 已有哪些能力可复用 | 能力、入口、证据 |
| 新能力 | 是否新增外部系统、SDK、协议或跨模块能力 | 有/无及理由 |
| 归属 | 是模块 Adapter、通用 `pkg` Capability 还是进程选择的底层能力 | 唯一 owner |
| 资源 | 是否有连接、Client、listener、订阅、goroutine、缓存或派生句柄 | 创建和释放责任 |
| 运行 | 是否需要 Start、Ready、Health、Run、Stop、Wait | owner 和顺序 |
| 配置 | 是否有配置、Secret、Defaults、严格校验和变化分类 | section owner |
| 出口 | 普通接口、稳定 facade 还是 Lease Access | 最小调用契约 |
| Reload | 是否能并存、排空、回滚，还是只能重启 | 当前策略 |
| 契约适配 | 当前项目契约能否表达全部保证 | 适用/不适用 |
| 失败 | 取消、超时、重试、幂等、背压、降级和清理错误 | 可识别语义 |
| 影响 | composition、Host、HTTP、CLI、配置、迁移和测试 | 文件与验收清单 |

“无新能力”不是空表：必须列出复用的能力、核对位置和为什么不需要新增资源治理。

## 5. 决策流程

```text
真实用例与外部副作用
  -> 盘点当前 Capability
     -> 已存在：通过最小接口注入模块
     -> 模块专属且无进程共享：保留在模块 Adapter/Participant
     -> 新的可复用底层能力：定义项目契约并进入 Kernel App 形态判断
     -> 证据不足：继续研究，不建占位目录或接口

Kernel App 形态判断
  -> 固定且无资源生命周期：Value
  -> 固定但需 Start/Stop：ManagedFixed + NoReload
  -> 配置化且候选可安全并存：ManagedConfigured + Lease + KernelInstanceSwap
  -> 配置变化不能安全热换：ManagedConfigured + RestartRequired
  -> 明确替换既有稳定 target：Replacement
  -> 需要当前未支持的 Handoff/Native Reload/观察期/复杂原子依赖：停止接入，建立独立研究或 ADR
```

上述当前项目枚举只存在于项目指南和 Kernel App 文档，不进入 `AGENTS.md`。

## 6. 模块与 Kernel 的所有权分界

新能力不等于所有代码都进入 Kernel：

- 共享底层连接、连接池或稳定能力出口可以由 Kernel App 管理；
- 模块业务 Service、Handler、Repository 和 Model 继续由 application composition root 普通构造；
- 模块特有 migration、消费者循环、索引任务或清理单元通过有 owner 的 contribution/Participant 进入 Host；
- `internal/composition` 连接 Kernel Capability、模块 Config、跨模块 port 和模块 contribution；
- 业务核心不得导入 Kernel 或完整 `Capabilities`。

研究必须分别回答“底层资源 owner”和“模块运行 owner”，不能因为两者共同服务一个功能就合并成万能组件。

## 7. 契约缺口升级规则

出现以下任一事实时，不得直接编写新能力实现：

- 新旧资源不能并存，但需求要求不中断切换；
- 需要排他资源所有权交接或消费者组 Handoff；
- 需要资源自身 Native Atomic Reload；
- 需要提交后的观察期和自动回切；
- 需要当前前向 typed Input 无法表达的复杂资源依赖或多资源原子性；
- 无法确定借用对象、派生句柄或后台任务的排空边界；
- 失败、重试、幂等、补偿或一致性保证尚未定义。

处理流程：

1. 在当前模块研究中标记具体缺口和受影响用例；
2. 比较 `RestartRequired`、模块级受管运行单元和扩展 Kernel 契约三条路径；
3. 若仍需扩展公共生命周期或依赖模型，建立独立 `docs/changes/<seq>`；
4. 难以逆转的公共语义使用 ADR；
5. 在新方案确认前，原模块任务保持待确认，不添加空 Hook、兼容层或隐藏回退。

## 8. 示例设计要求

指南提供消息、邮件和搜索三类简短例子，每个例子都分成两层：

- 底层 Capability：项目契约、共享 Client/连接、配置、健康和资源释放；
- 模块语义：消费者/投递/索引用例、幂等、一致性、重试、排空和验收。

例子只展示问题和所有权拆分，不冻结产品或公共 API。后续真实任务必须重新研究。

## 9. 文件影响

计划实施只修改以下文档：

- `AGENTS.md`
- `README.md`
- `docs/README.md`
- `docs/research/README.md`
- `docs/development/application-module-development.md`（新增）
- `internal/module/README.md`
- `internal/kernel/app/README.md`
- `docs/changes/017-module-capability-assessment-gate/**`
- `docs/changes/README.md`

不修改 Go、配置、脚本、依赖、生成物或测试实现。

## 10. 失败语义与一致性

- 如果指南与当前代码或 Kernel App 文档冲突，以代码事实为准，返回研究更新方案；不在多个位置保留不同版本。
- 如果链接或主题 owner 不明确，停止发布，先确定唯一权威文件。
- 如果实施中发现必须改变 Kernel API、生命周期语义或自动门禁代码，本任务退出纯文档范围，更新需求/设计并重新确认。

## 11. 验证方案

- 检查 `AGENTS.md -> README.md -> docs/README.md -> 应用模块开发指南` 导航闭合。
- 检查指南到模块、能力清单和 Kernel App 文档的相对链接有效。
- 搜索确认 `AGENTS.md` 没有项目专属 API、目录和具体产品清单。
- 搜索确认只有项目指南拥有完整能力评估表，其他入口不复制正文。
- 核对指南中的当前组件形态与 `internal/kernel/app/README.md` 一致。
- 核对消息、邮件、搜索均未被描述为已实现或已选型。
- 运行 `git diff --check`。
- 本任务不运行或宣称 Go 测试证明文档流程；若未修改 Go，测试不是必要证据。
