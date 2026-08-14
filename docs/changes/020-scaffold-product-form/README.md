# 020：复制型脚手架产品形态

## 当前状态

- 任务类型：用户决策后的方案调整与待确认隔离验证。
- 研究门禁：已通过，证据为 [R001 当前分发边界](research/R001-current-distribution-boundary/report.md) 与 [R002 Go 分发和版本语义](research/R002-go-distribution-versioning/report.md)。
- 产品形态决策：用户于 2026-08-15 明确选择“复制脚手架源码”，不由脚手架创建项目，不采用 generator、library-only 或 Runtime + generator 形态。
- 计划状态：已按该决策重写，等待后续明确确认；本轮只修改文档。
- 代码快照：`main@1b60d16b6807313ee33d60b9a3d1659bf16abac1`。
- 来源：[019 的 `FORM-001`](../019-http-api-maturity-gap-assessment/tasks.md)。

## 一句话结论

`go-scaffold2` 本身就是一份版本化、可运行、可复制的完整服务源码基线。使用者复制某个发布快照后，一次性替换项目身份并建立自己的 Git 历史；`pkg`、`internal/kernel/app`、底层 composition、业务模块和交付文件全部归新项目所有，不再依赖源脚手架，也不接受后续 generator 覆盖。

```text
go-scaffold2 release/tag
          │ 复制源码快照，不运行 scaffold create
          ▼
新项目工作区
  ├─ 一次性迁移 module / app / env prefix 等身份
  ├─ 保留或删除 Todo 示例
  ├─ 完整验证
  └─ 建立独立 Git 历史，之后自行演进
```

## 明确排除

- 不开发 `scaffold new`、`init`、模板渲染器或项目生成 DSL。
- 不把 Kernel/Plan/Host 搬成供外部项目依赖的公共 Runtime module。
- 不把上游 Git merge、自动重生成或文件覆盖作为升级机制。
- 不维护“复制模式”和“generator 模式”两套当前入口。

## 仍需验证的问题

1. 一个干净源码快照能否排除 `.git`、运行数据和本机状态后完整复制？
2. module path、应用名、可执行名、环境变量前缀和文档身份是否能一次性迁移且无残留？
3. `pkg -> internal/kernel/app -> internal/kernel/composition -> internal/composition` 是否能在新 module 中原样编译和运行？
4. Todo 示例能否被明确保留，或按清单完整移除而不破坏底座？
5. 如何记录复制来源，并诚实发布后续安全修复和人工迁移说明？

## 阅读顺序

1. [需求与验收标准](requirements.md)
2. [复制、身份迁移与版本设计](design.md)
3. [待确认验证任务](tasks.md)
4. [研究档案](research/README.md)

## 当前行为边界

020 尚未改变项目的复制、重命名或发布方式。任何临时副本、身份替换、Todo 移除、构建、测试或 ADR 结果更新，都必须在本计划报告之后获得用户明确确认。
