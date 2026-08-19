# 配置说明

本文是应用配置来源、生成方式、环境变量覆盖和配置节 ownership 的当前权威说明。本地首次启动见 [本地启动指南](../getting-started/local-development.md)。

## 配置来源

默认配置文件是仓库根目录的 `config.yaml`。应用按声明顺序合并配置来源：

```text
FileSource -> EnvSource
```

因此环境变量覆盖文件。环境变量使用 `APP_` 前缀，嵌套字段用双下划线分隔，例如：

```powershell
$env:APP_DATABASE__DSN = '.data/app.db'
$env:APP_LOGGER__LEVEL = 'debug'
$env:APP_HTTP__ADDR = '127.0.0.1:8081'
```

配置解析是 strict 的：未知配置节、未知字段、重复逻辑路径、大小写别名和错误类型都会在创建资源或绑定 listener 前失败。

## 生成配置与示例文件

首次本地开发推荐使用：

```powershell
go run ./cmd/app config init
```

该命令聚合所有配置 owner 的默认值，并用同一套 binder 与 validator 做回环校验。默认目标已存在时会拒绝覆盖。

需要生成到其他路径：

```powershell
go run ./cmd/app config init --output .data/generated-config.yaml
```

`config.example.yaml` 是带注释的字段参考，不是第二套启动权威。可以手工复制它，但复制后仍要执行 `db migrate up` 再启动 Service。

## 配置节 ownership

| 配置节 | Owner | 主要用途 |
| --- | --- | --- |
| `logger` | Kernel Logger App | 进程日志级别、编码、输出路径 |
| `database` | Kernel Database App | 数据库 driver、DSN、连接池和 ping timeout |
| `migration` | Migration 模块 | one-shot migration 锁等待和操作期限 |
| `cache` | Kernel Cache App | 缓存 backend 与 Redis 参数 |
| `i18n` | Kernel I18n App | 默认语言、消息文件（统一维护于 `./locales`）和缺失翻译策略 |
| `storage` | Kernel Storage App | 本地、S3、MinIO 对象存储配置 |
| `todo` | Todo 模块 | Todo 业务约束 |
| `auth` | Auth 模块 | development-anonymous 或 JWT 鉴权配置 |
| `http` | Kernel HTTP composition | 业务 HTTP listener 与请求治理 |
| `management` | Ops 模块 | management listener、readiness、metrics access |
| `observability` | Kernel Observability composition | service name、tracing exporter 与采样参数 |

Bootstrap CLI、migration one-shot、Todo CLI 和长期 Service 都必须识别同一套官方应用配置节。某个运行模式可以只创建自己需要的资源，但不能把其他官方配置节当作未知字段拒绝。

## Tracing

`observability.tracing.enabled` 默认为 `false`。启用后必须提供合法 endpoint、采样和批处理参数。生产环境优先使用安全传输；HTTP endpoint 只适合明确受控的本地或测试环境，并需要显式配置 `insecure: true`。

## 密钥与环境差异

密码、Token、Access Key、完整生产 DSN 和私有证书不要写入配置文件或提交历史。使用环境变量注入敏感值，并让部署系统负责密钥来源。配置错误和日志也不得输出完整 DSN 或凭据。

## Reload 行为

长期 Service 监听配置文件变化，并把文件与进程启动时继承的环境变量重新合并成候选配置。被环境变量覆盖的字段即使文件发生变化，也不会改变最终有效值。reload 的底层机制见 [Kernel 运行与配置](../../internal/kernel/README.md)。
