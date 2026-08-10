package i18n

const (
	// DefaultLanguage 是未显式配置语言时使用的默认语言。
	DefaultLanguage = defaultLanguageValue
	// DefaultMissingBehavior 是未显式配置缺失翻译策略时使用的默认行为。
	DefaultMissingBehavior MissingBehavior = MissingBehaviorError
)

// DefaultConfig 返回一份可修改的默认配置。
func DefaultConfig() Config {
	return Config{
		DefaultLanguage: DefaultLanguage,
		MissingBehavior: DefaultMissingBehavior,
	}
}
