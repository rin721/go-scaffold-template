# 需求：脚手架产品形态与升级模型

## 1. 目标

基于 [R001](research/R001-current-distribution-boundary/report.md) 和 [R002](research/R002-go-distribution-versioning/report.md)，用外部消费者实验证据决定 `go-scaffold2` 的产品形态，并冻结创建、所有权、版本和升级语义。

## 2. 消费者与使用场景

主要消费者是创建新 Go HTTP API 服务的平台/业务开发者，而不是为本仓库贡献 Kernel 的维护者。必须覆盖四个连续场景：

1. 用目标 module path、应用名、环境变量前缀和示例选择创建新服务；
2. 在应用拥有区域新增真实业务代码；
3. 升级脚手架 Runtime 的兼容版本；
4. 获取模板结构改进，同时保留用户代码并得到可审阅冲突。

## 3. 功能需求

| ID | 需求 |
| --- | --- |
| `FORM-001` | 比较 template、generator、library 和组合模式，并以可重复实验和 ADR 单轨选择一种当前形态。 |
| `IDENTITY-001` | 创建输入必须显式包含 module path、应用标识、可执行文件名、配置文件名和环境变量前缀；校验失败不得生成半成品。 |
| `OWNERSHIP-001` | 每个输出文件必须归类为 Runtime 依赖、generator 拥有或 application 拥有；不得根据文件是否改过来猜所有权。 |
| `EXAMPLE-001` | Todo 必须被定义为显式示例选择；产品决策需说明无 Todo 时的最小服务形态，不能把示例业务伪装成框架核心。 |
| `RUNTIME-001` | 若选择公共 Runtime，其 API 必须窄、第三方类型不泄漏、外部消费者不得导入源仓库 `internal/**`。 |
| `VERSION-001` | 必须区分 Runtime module version、generator version 和 template schema version，并定义各自兼容含义。 |
| `GENERATE-001` | 相同版本与相同输入必须得到语义一致的输出；生成失败必须可诊断且不留下被当成成功项目的部分结果。 |
| `UPGRADE-001` | Runtime 升级必须可用 Go module 标准流程验证；模板演进必须产生可审阅迁移或明确声明不自动升级。 |
| `CONFLICT-001` | 任何升级或重生成都不得静默覆盖 application-owned 文件；冲突必须停止并列出精确目标。 |
| `RELEASE-001` | 采用 library/组合模式时必须定义 v0 到 v1 的兼容门禁和 tag 规则；没有 tag 时不得声称可版本化消费。 |
| `VERIFY-001` | 决策必须由独立 Go module 完成创建、编译/测试、定制、升级和无越界导入验证。 |
| `PORTABLE-001` | 生成结果必须在 Windows 与 Linux 路径/换行差异下保持语义一致；未执行的平台必须明确记录。 |

## 4. 质量与治理约束

- 第三方工具只能位于可替换 Adapter/命令边界，不能决定项目公共契约。
- 生产仓库不能保留两种现行创建方式或“旧 generator”兼容层；实验失败产物只存在于 Git 忽略目录。
- 不把 `map[string]any` 或自由文本替换作为核心模板 schema。
- 用户代码与生成代码的边界必须由 manifest/元数据表达，并可被测试。
- 错误必须保留目标文件、阶段和原因，但不得泄露本机绝对路径到生成产物。

## 5. 非目标

- 本任务不实施正式 generator、公共 Runtime、release pipeline 或 package 搬迁。
- 不同时决定 OpenAPI authority、认证、观测、管理面和部署模型。
- 不承诺从任意历史 fork 自动升级，也不以 Git subtree/merge 作为默认产品协议。
- 不发布 Git tag、不推送远端、不启动服务或外部基础设施。

## 6. 验收标准

1. 四种形态在同一指标下比较，结论包含被拒绝方案和触发重新评估的条件。
2. 隔离消费者使用目标身份完成干净创建，生成产物没有错误的源项目身份残留。
3. 外部 module 不导入 `github.com/rin721/go-scaffold2/internal/**`。
4. 至少模拟一次 Runtime 兼容升级和一次模板 schema 演进；用户 sentinel 文件不被覆盖。
5. Todo/none 选择的可行性得到证据；若当前代码阻塞，明确记录需要后续实施的最小缺口。
6. ADR 明确当前单轨产品形态、公共/生成/应用边界、三种版本语义和失败条件。
7. 所有正式源码变化仍由 ADR 后的新变更计划承接；020 不把原型当实现交付。
