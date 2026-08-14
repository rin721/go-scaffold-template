# 任务：复制型脚手架产品形态

## 1. 门禁状态

- 020 研究门禁：已通过。
- 产品形态：用户已确认 copy-owned source scaffold，旧的 generator/library/组合候选失效。
- 调整后计划状态：用户已于 2026-08-15 明确确认；隔离复制验证已完成。
- 当前授权边界：验证副本保持 Git 忽略；只提交 020/019 与必要导航，不修改生产源码。

## 2. 已完成任务

| ID | 工作量 | 依赖 | 内容 | 完成条件 | 状态 |
| --- | --- | --- | --- | --- | --- |
| `RES-001` | M | `019/FORM-001` | 核验当前分发、身份、版本与外部消费边界 | R001 可复核且区分事实与推断 | 已完成 |
| `RES-002` | M | 无 | 研究 Go `internal`、module、模板复制和版本语义 | R002 只使用主源并说明适用边界 | 已完成 |
| `DEC-001` | S | 用户决定 | 选择完整源码复制，排除 generator/library/组合模式 | 当前方案只有 copy-owned 一条权威路径 | 已完成 |
| `PLAN-001` | M | `DEC-001` | 重写复制、身份、所有权、示例和升级方案 | requirements/design/tasks 与用户决策一致 | 已完成 |

上一版 `PROBE-001`、`PROBE-002`、`PROBE-003` 未获得确认、未执行，现已由下列单轨任务替换，不保留兼容任务。

## 3. 已确认并完成的验证任务

| ID | 工作量 | 依赖 | 内容 | 完成条件 | 状态 |
| --- | --- | --- | --- | --- | --- |
| `COPY-001` | M | 用户确认 | 从固定 commit 将允许的 tracked baseline 复制到忽略目录，验证包含/排除清单 | 不复制 `.git`、运行数据、凭据、缓存或本机状态；副本不回链源工作区 | 已完成 |
| `IDENTITY-001` | M | `COPY-001` | 在副本中按语义清单迁移 module/app/binary/config/env/docs/test identity | 目标身份一致；错误的旧运行身份和 Go import 归零 | 已完成 |
| `TODO-001` | M | `IDENTITY-001` | 验证保留 Todo 的完整复制基线 | build/test/vet/config init 和装配链验证通过 | 已完成 |
| `TODO-002` | L | `COPY-001` | 在第二副本中验证 Todo 完整移除边界 | 无失效配置、migration、route、CLI、composition、test、doc；阻塞则形成精确后续任务 | 已完成 |
| `POLICY-001` | S | `COPY-001` | 验证 baseline provenance、无自动升级和安全修复迁移信息模型 | 能判断副本来源和受影响版本，不形成运行期依赖 | 已完成 |
| `PORTABLE-001` | M | `IDENTITY-001` | 核验 Windows 流程，并执行可用的 Linux/WSL 等价检查 | 两平台结果分别记录；不可用平台不伪称通过 | 部分完成：Windows 通过，WSL 无发行版 |
| `ADR-001` | M | 上述验证 | 固化 copy-owned 单轨决策、边界、版本与剩余实施任务 | 020/019/权威文档同步，不夹带正式源码实现 | 已完成 |

## 4. 确认后的允许范围

- 只允许在 Git 忽略的 `tmp/scaffold-copy-validation/` 创建副本、迁移身份、删除副本中的 Todo 和保存非敏感验证摘要。
- 允许在副本内执行 `go mod tidy`、build、test、vet、`config init`、身份/边界扫描和临时 Git 状态检查。
- 允许更新 020、019 和必要文档导航，记录验证事实与 ADR 结果。
- 不允许修改当前仓库的 `cmd/`、`internal/`、`pkg/`、根配置、依赖、脚本或 CI。
- 不允许新增 generator、公开 Runtime、tag、push、长期进程或外部写入。

## 5. 逐轮证据

| 轮次 | 日期 | 完成任务 | 证据 | Commit | 剩余风险 |
| --- | --- | --- | --- | --- | --- |
| 1 | 2026-08-15 | `RES-001`、`RES-002`、旧 `PLAN-001` | HEAD `af7fdad`；仓库身份/边界/tag 清单；Go/GitHub 官方资料 | `1b60d16` | 原计划尚未验证，随后被用户产品决策替换 |
| 2 | 2026-08-15 | `DEC-001`、新 `PLAN-001` | 用户明确要求复制脚手架而非脚手架创建；当前装配链复核；计划单轨重写与文档验证 | 本次纯文档提交 | 尚未真实复制、迁移身份、移除 Todo 或验证平台可移植性 |
| 3 | 2026-08-15 | `COPY-001`、`IDENTITY-001`、`TODO-001`、`TODO-002`、`POLICY-001`、`PORTABLE-001`、`ADR-001` | [R003](research/R003-isolated-copy-validation/report.md)；两个独立 Git/module；完整 Go 门禁、config init、残留与边界扫描 | 本次文档提交 | Linux 未验证；正式指南/release/公告模板未实现；021 尚待确认 |

## 6. 完成边界

020 的产品形态决策和隔离验证已经完成。临时副本不提交，生产源码不变。021 的 repository identity 迁移、正式复制指南、release baseline、安全公告模板和 Linux CI 都是新的状态变更，必须在各自计划确认后实施。
