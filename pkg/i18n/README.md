# i18n

`pkg/i18n` 是项目内通用国际化封装。它使用 `github.com/nicksnyder/go-i18n/v2/i18n` 作为底层消息库，并通过项目自己的 `Translator`、`Config` 和 `Message` API 暴露给业务代码。

## 技术选型

- `go-i18n/v2` 覆盖消息文件、语言匹配、模板变量和复数规则，适合后端服务、CLI 和多语言业务文本的长期维护。
- `golang.org/x/text/language` 是 Go 官方国际化基础库，用于语言标签解析和匹配，稳定性优先于手写字符串匹配。
- `gopkg.in/yaml.v3` 只用于加载 YAML 资源文件；业务代码不依赖 YAML 解析类型。
- 本包不把 go-i18n 的 `Bundle`、`Localizer`、`Message` 暴露给业务层。后续如需替换消息库，应保持 `Translator`、`Config` 和 `Message` 的项目契约稳定。

## 设计目标

- 简单：通过 `i18n.New` 创建翻译器，再调用 `Translate`。
- 通用：支持 JSON、YAML、默认语言、语言匹配、模板变量和复数消息。
- 可维护：资源加载、默认值、配置校验和业务 API 分文件维护。
- 边界清晰：业务代码不直接依赖 go-i18n 的 bundle、localizer 或 message 类型。

## 目录结构

```text
pkg/i18n/
├── builder.go      # 配置补全、校验和底层 bundle 构建
├── config.go       # Config、MissingBehavior 等配置类型
├── constants.go    # 默认语言、文件扩展名、策略字符串
├── defaults.go     # 默认配置和默认值
├── message.go      # Message 和 LocalizeOption
├── translator.go   # Translator 接口、ValidateConfig 和 New 构造函数
└── README.md       # 使用文档
```

## 配置项说明

| 字段 | 说明 | 默认值 |
| --- | --- | --- |
| `DefaultLanguage` | 默认语言标签，例如 `zh-CN`、`en` | `zh-CN` |
| `MessageFiles` | 翻译资源文件路径，支持 `.json`、`.yaml`、`.yml` | 空 |
| `MessageFS` | 资源文件所在文件系统，可传入 `embed.FS` | `os.DirFS(".")` |
| `MissingBehavior` | 缺失翻译策略：`MissingBehaviorError` 或 `MissingBehaviorUseID` | `MissingBehaviorError` |

`New(nil)` 会创建一个空翻译器。它适合测试或逐步接入，但没有资源文件时，实际翻译默认会返回缺失消息错误。

## 资源文件格式

资源文件名需要包含语言标签和格式扩展名，例如：

```text
locales/active.zh-CN.yaml
locales/active.en.json
```

YAML 示例：

```yaml
hello:
  other: "你好，{{.Name}}"
user_count:
  one: "{{.Count}} 个用户"
  other: "{{.Count}} 个用户"
```

`WithCount` 会同时提供模板变量 `.Count` 和 `.PluralCount`。业务资源推荐使用更短的 `.Count`，底层复数选择仍由同一个计数值驱动。

JSON 示例：

```json
{
  "hello": {
    "other": "Hello, {{.Name}}"
  },
  "user_count": {
    "one": "{{.Count}} user",
    "other": "{{.Count}} users"
  }
}
```

## 基础使用示例

```go
package main

import (
	"fmt"

	"github.com/rin721/go-scaffold-template/pkg/i18n"
)

func main() {
	translator, err := i18n.New(&i18n.Config{
		DefaultLanguage: "zh-CN",
		MessageFiles: []string{
			"locales/active.zh-CN.yaml",
			"locales/active.en.json",
		},
	})
	if err != nil {
		panic(err)
	}

	text, err := translator.Translate("zh-CN", i18n.Text(
		"hello",
		i18n.WithData(map[string]any{"Name": "Rin"}),
	))
	if err != nil {
		panic(err)
	}

	fmt.Println(text)
}
```

## 自定义配置示例

```go
package main

import (
	"embed"
	"fmt"

	"github.com/rin721/go-scaffold-template/pkg/i18n"
)

//go:embed locales/*.yaml locales/*.json
var localeFS embed.FS

func main() {
	cfg := i18n.DefaultConfig()
	cfg.MessageFS = localeFS
	cfg.MessageFiles = []string{
		"locales/active.zh-CN.yaml",
		"locales/active.en.json",
	}
	cfg.MissingBehavior = i18n.MissingBehaviorUseID

	translator, err := i18n.New(&cfg)
	if err != nil {
		panic(err)
	}

	text := translator.MustTranslate("en-US", i18n.Text(
		"user_count",
		i18n.WithCount(2),
	))

	fmt.Println(text)
}
```

## 在业务代码中的推荐使用方式

独立使用时推荐在应用入口创建 `Translator`；Kernel 组合模式直接把 `capabilities.I18n` 这个稳定 facade 通过构造函数注入业务组件。业务组件只依赖 `i18n.Translator`，不要在业务函数内部重复加载翻译资源。

```go
package user

import "github.com/rin721/go-scaffold-template/pkg/i18n"

type Service struct {
	translator i18n.Translator
}

func NewService(translator i18n.Translator) *Service {
	return &Service{translator: translator}
}

func (s *Service) Welcome(language string, name string) (string, error) {
	return s.translator.Translate(language, i18n.Text(
		"hello",
		i18n.WithData(map[string]any{"Name": name}),
	))
}
```

本包当前不提供全局 translator。Kernel facade 也是 `Compose` 的显式输出，不是全局变量；配置成功重载时 facade 身份不变、内部 Translator 换代，资源加载失败时保留旧实例。Kernel 配置中的消息文件以进程工作目录为根，缺失策略字符串使用 `error` 或 `use-id`。
