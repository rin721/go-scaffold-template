package i18n

import (
	"io/fs"

	"golang.org/x/text/language"
)

// MissingBehavior 表示翻译资源缺失时的处理策略。
type MissingBehavior string

const (
	// MissingBehaviorError 表示缺失翻译时返回错误。
	MissingBehaviorError MissingBehavior = missingBehaviorErrorValue
	// MissingBehaviorUseID 表示缺失翻译时返回消息 ID。
	MissingBehaviorUseID MissingBehavior = missingBehaviorUseIDValue
)

// Config 定义 Translator 构造参数。
type Config struct {
	DefaultLanguage string
	MessageFiles    []string
	MessageFS       fs.FS
	MissingBehavior MissingBehavior
}

type resolvedConfig struct {
	DefaultLanguage language.Tag
	MessageFiles    []string
	MessageFS       fs.FS
	MissingBehavior MissingBehavior
}
