# 任务：业务模块统一契约与 binding 对齐（033）

## 1. 门禁状态

- 研究门禁：已通过，证据为 `R001`、`R002`。
- 当前计划状态：待确认。
- 当前授权：用户仅请求研究并提交修复方案；未授权实施。实施必须等待用户在计划报告后的后续消息中明确确认 033 当前方案。
- 实施前提：尚未满足；命中 design 第 9 节触发器后恢复待确认。
- 外部副作用：确认后的实施只修改仓库内源码、测试、配置示例与文档目录；不 push、不 tag、不 release、不部署、不写外部数据库。

## 2. 任务清单

| ID | 工作量 | 依赖 | 任务 | 完成条件 | 状态 |
| --- | --- | --- | --- | --- | --- |
| `RES-001` | L | 无 | 梳理业务模块统一契约清单与现状缺口（HTTP/config/cli/migration/i18n/middleware） | R001 区分事实、推断与缺口清单 | 已完成 |
| `RES-002` | L | `RES-001` | 设计统一 binding 契约与 i18n 接入（业务模块自有语言资源 + binding + 聚合） | R002 给出清单、i18n 设计、Ops/Auth/Migration 对齐、门禁 | 已完成 |
| `PLAN-001` | L | `RES-001..002` | 冻结 033 需求、设计、文件影响、风险与验证 | README、requirements、design、tasks 完整且相互引用 | 已完成（待确认） |
| `DOC-001` | M | 用户确认 | 先把统一 binding 契约清单与 i18n 接入固化到模块开发指南（声明位置/接入方式/维护位置） | 模块开发指南单一声明各 binding 契约 | 待确认 |
| `I18N-001` | M | `DOC-001` | 落地业务模块 i18n binding：模块内语言资源 + `binding/i18n`（catalog/MessageFiles/fs.FS），Todo 先行，按需扩展 Auth/Ops/Migration | 业务模块提供自有语言资源+binding，经聚合注入 Translator | 待确认 |
| `ALIGN-001` | M | `DOC-001`、`I18N-001` | 对齐 Ops management HTTP 形态（收敛手写 ServeMux 或文档明确独立 management 边界）、Auth/Migration 文档形态说明 | Ops 不再旧式手写路由直接形态或明确边界；Auth/Migration 文档清晰 | 待确认 |
| `GOV-001` | M | `ALIGN-001` | 扩展门禁：HTTP binding→ModuleContract 注册；i18n binding→语言资源来源；防旧式手写路由；保留 kernel/app vs pkg 边界门禁 | 可执行门禁 + fixture 通过 | 待确认 |
| `SYNC-001` | M | 全部实施任务 | 同步模块 README、配置说明、变更索引；删除旧式路径残留 | 权威文档单轨；无旧式手写路由残留 | 待确认 |
| `VER-001` | L | 全部实施任务 | 执行单元/集成/完整 gate 与文档审阅 | requirements 第 7 节全部有直接证据；无旧式残留 | 待确认 |

## 3. 实施顺序

```text
DOC-001 -> I18N-001 -> ALIGN-001
  -> GOV-001 -> SYNC-001 -> VER-001
```

契约清单文档先行，作为实现依据；i18n binding 先 Todo 后按需扩展；门禁与对齐必须同轮完成，避免长期红灯。

## 4. 计划报告后的确认要求

1. 用户阅读研究结论、需求、设计与任务后，在后续消息中明确确认「033 当前方案」；
2. 确认后按任务 ID 顺序实施；范围外的新事实退回研究阶段；
3. 实施中的每个检查点提交只包含 033 范围文件；不 push、不 tag、不 release。

## 5. 停止条件

- 用户未确认当前计划；
- 命中 design 第 9 节重新确认触发器；
- 工作区出现无法可靠分离的用户修改；
- 测试、race、vet、build、tidy、生成或门禁失败且无法在确认范围内修复；
- 为继续工作必须保留旧式手写路由、双套 i18n 路径或兼容 wrapper。

停止时保留研究、计划与已确认任务范围，不以占位实现或降低门禁冒充完成。

## 6. 建议实施提交信息（确认后）

```text
refactor(module): align business modules to unified binding and i18n contracts
```
