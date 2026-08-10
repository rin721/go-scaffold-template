package i18n

import (
	"fmt"

	goi18n "github.com/nicksnyder/go-i18n/v2/i18n"
)

// Translator 定义业务代码使用的国际化翻译能力。
type Translator interface {
	Translate(language string, message Message) (string, error)
	MustTranslate(language string, message Message) string
}

type bundleTranslator struct {
	bundle          *goi18n.Bundle
	defaultLanguage string
	missingBehavior MissingBehavior
}

// New 根据配置创建 Translator。
func New(cfg *Config) (Translator, error) {
	resolved, err := resolveConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("resolve i18n config: %w", err)
	}

	bundle, err := buildBundle(resolved)
	if err != nil {
		return nil, fmt.Errorf("build i18n bundle: %w", err)
	}

	return &bundleTranslator{
		bundle:          bundle,
		defaultLanguage: resolved.DefaultLanguage.String(),
		missingBehavior: resolved.MissingBehavior,
	}, nil
}

func (t *bundleTranslator) Translate(language string, message Message) (string, error) {
	if message.ID == "" {
		return "", fmt.Errorf("message id is empty")
	}

	localizer := goi18n.NewLocalizer(t.bundle, t.localizerLanguages(language)...)
	text, err := localizer.Localize(toLocalizeConfig(message))
	if err != nil {
		if t.missingBehavior == MissingBehaviorUseID && isMissingMessageError(err) {
			return message.ID, nil
		}
		return "", fmt.Errorf("translate message %q: %w", message.ID, err)
	}

	return text, nil
}

func (t *bundleTranslator) MustTranslate(language string, message Message) string {
	text, err := t.Translate(language, message)
	if err != nil {
		panic(err)
	}
	return text
}

func (t *bundleTranslator) localizerLanguages(language string) []string {
	if language == "" {
		return []string{t.defaultLanguage}
	}
	return []string{language, t.defaultLanguage}
}

func toLocalizeConfig(message Message) *goi18n.LocalizeConfig {
	config := &goi18n.LocalizeConfig{
		MessageID:    message.ID,
		TemplateData: templateData(message),
		PluralCount:  message.Count,
	}

	if message.DefaultMessage != "" {
		config.DefaultMessage = &goi18n.Message{
			ID:    message.ID,
			Other: message.DefaultMessage,
		}
	}

	return config
}

func templateData(message Message) any {
	if message.Count == nil {
		return message.Data
	}
	if message.Data == nil {
		return map[string]any{
			"Count":       message.Count,
			"PluralCount": message.Count,
		}
	}
	_, hasCount := message.Data["Count"]
	_, hasPluralCount := message.Data["PluralCount"]
	if hasCount && hasPluralCount {
		return message.Data
	}

	data := make(map[string]any, len(message.Data)+2)
	for key, value := range message.Data {
		data[key] = value
	}
	if !hasCount {
		data["Count"] = message.Count
	}
	if !hasPluralCount {
		data["PluralCount"] = message.Count
	}
	return data
}
