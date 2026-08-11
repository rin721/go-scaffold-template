# 实施任务：默认配置契约与可选 CLI 能力

## 使用规则

- 本清单是后续实现的唯一任务账本。需求语义以 [requirements.md](requirements.md) 为准，技术接口以 [design.md](design.md) 为准。
- 每个任务使用稳定 ID 和工作量点数。点数只用于记录完成量，不代表工时。
- 只有代码、测试、文档和该任务验收条件同时完成后，才能把任务从 `[ ]` 改为 `[x]`。
- 每轮 Agent 工作结束时必须追加一条执行记录，列出实际完成的任务 ID、本轮点数、累计点数、验证证据、Commit 和剩余风险。
- 任务需要拆分时新增子任务 ID，不重写已经完成的记录；架构决策变化必须先更新 requirements/design，再调整任务。

## 进度总览

- 设计文档：`1 / 1` 点。
- 实现任务：`0 / 31` 点。
- 总进度：`1 / 32` 点。

## 任务清单

### 文档基线

- [x] **DOC-001（1 点）** 建立需求、开发设计、Agent 任务账本和根 README 入口。
  - 验收：明确标注尚未实现；产品、接口、文件改动、测试和非目标相互一致。

### 契约与登记

- [ ] **CFG-001（4 点）** 实现有序默认配置 Value/Object 模型、构造函数和结构校验。
  - 依赖：DOC-001。
  - 验收：支持 object、list、string、bool、number、duration、null；重复字段和非法值有稳定错误链。
- [ ] **CFG-002（3 点）** 将 DefaultContract 纳入 Definition，并把 Register 单轨迁移为 Registration 返回值。
  - 依赖：CFG-001。
  - 验收：缺少契约或登记失败时返回零值 Registration；旧返回方式引用归零。

### 配置聚合与文件事务

- [ ] **CFG-003（4 点）** 实现 DefaultManager、Binding 路径校验和 Continue/Abort 聚合事务。
  - 依赖：CFG-001、CFG-002。
  - 验收：顺序稳定；Abort/错误不产生部分文档；`AbortedError` 同时保留分类与原因链。
- [ ] **CFG-004（3 点）** 实现有序 YAML/JSON 编码和 Golden 测试。
  - 依赖：CFG-003。
  - 验收：两空格缩进、duration 字符串、字段顺序和单一结尾换行全部固定。
- [ ] **CFG-005（4 点）** 实现安全文件写入、默认拒绝覆盖和 Force 跨平台原子替换。
  - 依赖：CFG-004。
  - 验收：父目录、权限、临时文件、Sync/Close、Unix rename、Windows MoveFileEx 和清理错误全部有测试或平台证据。

### 能力契约与 CLI

- [ ] **CAP-001（3 点）** 为 Database 实现并绑定默认配置契约。
  - 依赖：CFG-002。
  - 验收：字段和值来自 Database 自有策略；连接池/超时不复制魔法数字；YAML/JSON 结果一致。
- [ ] **CLI-001（3 点）** 实现 Kernel CLI Contract 和可选 App 组合。
  - 依赖：CFG-003。
  - 验收：复用 `pkg/cli`；nil/错误/重复命令失败时不返回部分 App；Kernel 核心不依赖 Cobra。
- [ ] **CLI-002（3 点）** 实现 `config init` 命令并调用 DefaultManager。
  - 依赖：CFG-005、CLI-001。
  - 验收：命令、flag、位置参数、stdout、错误链和退出码测试完整。

### Composition 与收口

- [ ] **CMP-001（2 点）** 迁移 Composition Options、Capabilities 和按能力拆分的绑定顺序。
  - 依赖：CAP-001、CLI-002。
  - 验收：CLI nil 语义明确；失败返回零值 Capabilities；旧 Compose 签名归零。
- [ ] **DOC-002（1 点）** 把实现后的真实用法同步到根说明、Kernel/Capability/CLI 文档。
  - 依赖：CMP-001。
  - 验收：当前实现与目标设计不再混淆；示例可编译或由测试覆盖。

## 最终验证任务

- [ ] **VER-001（1 点）** 执行并记录完整门禁。
  - 依赖：CFG-001 至 DOC-002 全部完成。
  - 必须通过：

```powershell
gofmt -w <本任务 Go 文件>
go test ./...
go test -race ./...
go vet ./...
git diff --check
```

  - 架构搜索必须确认以下旧入口或禁止模式归零：旧 `Compose(runtime)` 调用、缺少 Defaults 的生产 Definition、capability 内 `kernel.Register`、`init` 注册、反射发现和自动扫描。

## 逐轮执行记录

| 轮次 | 日期 | 完成任务 | 本轮点数 | 累计点数 | 验证 | Commit | 剩余风险 |
| --- | --- | --- | ---: | ---: | --- | --- | --- |
| 1 | 2026-08-11 | DOC-001 | 1 | 1 / 32 | 文档链接、目标/现状边界、Markdown 与 `git diff --check` | 当前文档基线提交 | 全部实现任务尚未开始。 |
