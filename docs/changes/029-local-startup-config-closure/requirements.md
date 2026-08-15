# 需求：本地启动与配置闭环

## 1. 目标

让新用户从仓库根目录按一条明确路径完成：生成配置、迁移本地 SQLite、启动 Service、验证 ready、停止并再次启动；同一生成配置必须被 Migration CLI、Todo CLI 与 Service 一致识别。

## 2. 事实依据

- `R001`：generator/consumer binding 漂移和当前文档冲突。
- 当前配置严格语义、migration owner 和 Service generation 不因本任务放宽。

## 3. 范围

### 3.1 代码

- 收口 application-owned 配置 binding 的唯一构造位置。
- Bootstrap、Service、Migration CLI、Todo CLI 使用同一集合。
- 保留底层 `ConfigurationBindings` 对 Logger/Database/Cache/I18n/Storage/HTTP 的唯一聚合。
- 不改变配置 key、默认值、格式、环境变量前缀或 CLI 命令名。

### 3.2 文档

- 根 README：五分钟启动、成功判据、最小文档地图。
- 新增本地开发 authority：首次启动、日常启动、停止、ready 检查、常见错误。
- 新增配置 authority：文件/env 来源与优先级、生成器与示例关系、完整 section ownership、严格校验、敏感信息。
- `config.example.yaml`、`docs/README.md`、migration 运维文档和相关模块 README 只链接 authority，不复制冲突步骤。
- `docs/changes` 继续作为历史证据，不进入正常操作路径。

### 3.3 测试

- 真实验证 generated config -> migration status/up -> Service ready -> graceful stop。
- 使用测试临时目录、临时 SQLite、loopback 临时端口和隔离环境变量前缀。
- 验证完整配置被 Migration 接受，真正未知 root 仍失败。
- 验证 tracing 默认关闭；启用时 endpoint 规则有明确错误与文档。

## 4. 非目标

- 不引入配置中心、profile、interactive wizard 或自动修复配置。
- 不让 Service 自动执行 migration。
- 不放宽未知 section 或 owner 严格校验。
- 不增加 PostgreSQL/MySQL/Redis/S3/OTLP 本地依赖。
- 不修改 Database schema、migration SQL、OpenAPI、HTTP 路由或业务语义。
- 不删除历史变更记录。

## 5. 约束

- 文档与注释以中文为主。
- `config init --force` 不作为日常推荐；已有配置应编辑或生成到另一路径比较。
- 默认本地路径不得要求凭据或远端服务。
- 测试必须释放端口、文件句柄和 goroutine，不使用固定端口。
- 只有用户确认 029 当前计划后才能修改非文档文件或运行会改变状态的命令。

## 6. 验收标准

1. 全新临时目录中，应用生成的默认配置无需删除合法 section 即可执行 migration。
2. `db migrate up` 后 Service 到达 ready，业务与 management listener 可访问，取消后端口和 SQLite 文件句柄释放。
3. Migration、Todo CLI 与 Service 对合法顶层 section 的认识来自同一 application-owned binding 集合。
4. `unknown config section` 只针对真正未知 root；回归测试覆盖 `management`、`observability` 和非法 root。
5. 根 README 首屏给出唯一三步本地路径，不要求先理解 Kernel/Application Generation。
6. 当前文档明确：`config init` 是首次启动推荐方案，`config.example.yaml` 是字段参考，`status` 是检查，`up` 是首次启动必要动作。
7. 排障表覆盖缺失配置、已有目标、unknown section、migration 未完成、trace endpoint、端口占用，并能区分用户配置与产品缺陷。
8. 当前使用、配置、架构、运维和历史证据的关系在 `docs/README.md` 中只定义一次。
9. 定向测试、完整 test/race/vet/build/tidy、Markdown 链接、命令残留和 `git diff --check` 通过；范围外既有 observability flaky test 单独如实记录，不用删除测试掩盖。
