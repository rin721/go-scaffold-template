# logger

`pkg/logger` 是项目内通用日志库封装。它使用 `go.uber.org/zap` 作为内部实现，但业务代码只依赖本包暴露的 `Logger`、`Config`、`Field` 和字段构造函数，不直接接触 zap 类型。创建方通过 `Resource` 独占 Sync、Close 和文件 sink，业务调用方不能关闭共享 logger。

何时必须记录、级别如何选择、唯一错误 owner、结构化字段和敏感信息要求统一见 [开发日志规范](../../docs/development/logging.md)；本页只说明 Logger API 与资源用法。

## 技术选型

- `zap` 是成熟的高性能结构化日志库，适合服务端长期运行、高频输出和 JSON 日志采集场景。
- 项目不把 `zap.Logger`、`zap.Field` 或 `zap.Config` 暴露给业务层；字段、配置和 logger 契约都由本包定义。
- 标准库 `log/slog` 保持为未来可评估方案。只有当项目更看重标准库统一生态，且能接受性能、字段语义和迁移成本变化时，才通过本包内部替换实现。

## 设计目标

- 简单：通过 `logger.New` 创建 Resource，直接调用 `Info`、`Error` 等方法并在所有者边界关闭。
- 高效：内部使用 zap 的结构化日志能力，适合服务端高频日志场景。
- 通用：支持开发环境、生产环境、日志级别、输出位置和结构化字段。
- 可维护：配置、默认值、级别、字段构造和 zap 适配分文件维护，便于后续扩展。

## 目录结构

```text
pkg/logger/
├── builder.go      # 配置补全、校验和 zap 配置构建
├── config.go       # Config、Environment、Encoding 等配置类型
├── constants.go    # 稳定字符串和编码字段名
├── defaults.go     # 默认配置和默认值
├── field.go        # 项目自有结构化字段类型、构造函数和内部 zap 字段转换
├── level.go        # 日志级别定义和底层级别映射
├── logger.go       # Logger、Resource、配置校验、构造和资源关闭
└── README.md       # 使用文档
```

## 配置项说明

| 字段 | 说明 | 默认值 |
| --- | --- | --- |
| `Environment` | 运行环境，支持 `logger.EnvironmentDevelopment` 和 `logger.EnvironmentProduction` | `development` |
| `Level` | 日志级别，支持 `debug`、`info`、`warn`、`error` | development 为 `debug`，production 为 `info` |
| `Encoding` | 输出格式，支持 `console` 和 `json` | 开发环境为 `console`，生产环境为 `json` |
| `OutputPaths` | 普通日志输出位置，例如 `stdout` 或文件路径 | `stdout` |
| `ErrorOutputPaths` | logger 内部错误输出位置 | `stderr` |
| `AddCaller` | 是否输出调用位置，`nil` 表示使用环境默认值 | 开发和生产均启用 |
| `AddStacktrace` | 是否输出错误堆栈，`nil` 表示使用环境默认值 | 开发关闭，生产启用 |

`AddCaller` 和 `AddStacktrace` 是 `*bool`，用于区分“未配置”和“显式关闭”。如果要从默认值开始调整，推荐使用 `logger.DefaultConfig()`。

## 基础使用示例

```go
package main

import (
	"github.com/rin721/go-scaffold-template/pkg/logger"
)

func main() {
	log, err := logger.New(nil)
	if err != nil {
		panic(err)
	}
	defer func() {
		if err := log.Close(); err != nil {
			panic(err)
		}
	}()

	log.Info("service started")
	log.Error("request failed", logger.String("path", "/api/users"))
}
```

## 自定义配置示例

```go
package main

import (
	"github.com/rin721/go-scaffold-template/pkg/logger"
)

func main() {
	cfg := logger.DefaultConfig()
	cfg.Environment = logger.EnvironmentProduction
	cfg.Level = logger.LevelDebug
	cfg.OutputPaths = []string{"stdout", "./app.log"}

	log, err := logger.New(&cfg)
	if err != nil {
		panic(err)
	}
	defer func() {
		if err := log.Close(); err != nil {
			panic(err)
		}
	}()

	log.Info(
		"service started",
		logger.String("service", "user-api"),
		logger.Int("port", 8080),
	)
}
```

## 在业务代码中的推荐使用方式

独立使用时推荐在程序入口创建 Resource、负责 Close，再把窄 `logger.Logger` 通过构造函数传入业务组件。业务包按自身需要定义依赖接口，或者直接依赖本包的 `logger.Logger`，不要在业务函数内部重复创建或关闭 logger。

```go
package user

import "github.com/rin721/go-scaffold-template/pkg/logger"

type Service struct {
	log logger.Logger
}

func NewService(log logger.Logger) *Service {
	return &Service{log: log}
}

func (s *Service) Create(name string) {
	s.log.Info("create user", logger.String("name", name))
}
```

由 Kernel 托管时，应用入口创建基线 Resource，`internal/kernel/logging.Manager` 提供始终可用、且动态类型不带控制方法的稳定 Logger view；composition 可以通过显式 `app.Replace` 选择 `internal/kernel/app/logger` 配置化 replacement。调用方直接注入 `Capabilities.Logger`，不会取得 Manager 的替换权或 Resource 的关闭权。未选择 replacement、候选失败或 replacement 停止后都继续使用 baseline。本包仍不提供全局 logger；独立使用和 Kernel 托管都保持显式所有权与注入。
