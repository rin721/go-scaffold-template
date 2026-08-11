# 产品需求：全量配置示例

## 背景与当前事实

`config init` 生成可解析的最小配置骨架，但 Database 的 Engine、Driver 和 DSN 必须由使用者明确提供。生成文件只包含默认契约，不列出所有合法选项，也不会说明 Logger 中由 environment 推导的可选字段，使用者容易直接启动空配置并在 Database 校验阶段失败。

## 目标

- 提供一份可复制为 `config.yaml` 的全量 YAML 示例。
- 覆盖当前 composition 实际登记的 Logger、Database 全部 typed Config 字段。
- 默认选择适合本地开发的 `development + gorm + postgres`。
- 互斥选项紧邻当前值以中文注释保留，使用者可以手动切换。
- DSN 只提供无真实凭据的环境变量模板，不把秘密写入文件。

## 验收标准

- 示例是合法 YAML，顶层只有当前已组合的 `logger` 和 `database` 配置段。
- Logger 列出 environment、level、encoding、outputPaths、errorOutputPaths、addCaller 和 addStacktrace，并准确说明环境派生语义。
- Database 列出 engine、driver、dsn、全部 pool 字段和 pingTimeout；合法 Engine、Driver 选项与代码一致。
- README 说明复制示例、选择方案、设置 DSN、启动应用以及 `config init --force` 的覆盖风险。
- 本地被忽略的 `config.yaml` 不被修改，示例不包含真实凭据或内部地址。

## 非目标

- 不改变 `config init` 的默认输出或覆盖策略。
- 不新增配置字段、Profile、字符串插值或第二个运行时配置来源。
- 不连接真实 Database，不修改 Go 源码、依赖或测试实现。
