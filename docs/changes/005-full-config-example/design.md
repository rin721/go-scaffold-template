# 开发设计：全量配置示例

## 1. 示例结构

根目录新增 `config.example.yaml`，使复制命令保持简单，并避免与已忽略的本地 `config.yaml` 混淆。活动配置固定为：

- Logger：`environment: development`、`level: info`、stdout/stderr。
- Database：`engine: gorm`、`driver: postgres`、当前连接池和 Ping 默认值。
- DSN：文件中保持空字符串，运行时由 `APP_DATABASE__DSN` 覆盖。

`production`、其他日志级别、`console/json`、显式 Caller/Stacktrace、文件输出、`sql` 和 `mysql` 都作为相邻注释候选出现。示例不复制成多个互相漂移的 Profile。

## 2. 派生字段和安全边界

Logger 的 encoding、addCaller 和 addStacktrace 默认保持注释，使运行时继续按 environment 推导；注释说明 development 与 production 的实际默认值。Database DSN 只给 PostgreSQL、MySQL 占位格式和 PowerShell 环境变量命令，不出现真实账号、密码、主机或数据库名。

环境变量继续使用应用入口已经定义的 `APP_` 前缀和双下划线路径。示例文件只是人工复制入口，不加入 Loader，也不改变 FileSource、EnvSource 或后加载来源覆盖先加载来源的语义。

## 3. 文档与验证

根 README 把带注释示例作为推荐起点，同时保留 `config init` 作为生成最小骨架的入口，并明确 `--force` 会覆盖已有目标。验证只检查 YAML、字段/枚举一致性、链接、敏感信息和 Diff；不连接外部 Database，也不声明真实服务启动通过。
