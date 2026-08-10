package cli

import (
	"context"
	"fmt"
	"path"
	"sort"
	"strconv"
	"strings"
)

const chainArgPrefix = "--chain."

type promptAnswerSource interface {
	promptAnswer(string) (string, bool)
}

type chainPromptUI struct {
	base    PromptUI
	answers map[string]string
}

// WithPromptAnswers 返回一个优先使用链式参数答案的 PromptUI。
func WithPromptAnswers(base PromptUI, answers map[string]string) PromptUI {
	normalized := normalizePromptAnswers(answers)
	if len(normalized) == 0 {
		return base
	}
	return &chainPromptUI{base: base, answers: normalized}
}

// PromptAnswer 读取指定 prompt key 的链式参数答案。
func PromptAnswer(ui PromptUI, key string) (string, bool) {
	source, ok := ui.(promptAnswerSource)
	if !ok {
		return "", false
	}
	return source.promptAnswer(key)
}

// SelectKey 使用语义 key 读取链式答案；缺失时回退到交互式选择。
func SelectKey(ctx context.Context, ui PromptUI, key string, prompt string, options []SelectOption) (string, error) {
	if value, ok := PromptAnswer(ui, key); ok {
		return resolveSelectAnswer(key, value, options)
	}
	if ui == nil {
		return "", fmt.Errorf("interactive UI is not available")
	}
	return ui.Select(ctx, prompt, options)
}

// ConfirmKey 使用语义 key 读取链式答案；缺失时回退到交互式确认。
func ConfirmKey(ctx context.Context, ui PromptUI, key string, prompt string, defaultValue bool) (bool, error) {
	if value, ok := PromptAnswer(ui, key); ok {
		return parseConfirmAnswer(key, value, defaultValue)
	}
	if ui == nil {
		return false, fmt.Errorf("interactive UI is not available")
	}
	return ui.Confirm(ctx, prompt, defaultValue)
}

// InputKey 使用语义 key 读取链式答案；缺失时回退到交互式输入。
func InputKey(ctx context.Context, ui PromptUI, key string, prompt string, defaultValue string) (string, error) {
	if value, ok := PromptAnswer(ui, key); ok {
		value = strings.TrimSpace(value)
		if value == "" {
			return defaultValue, nil
		}
		return value, nil
	}
	if ui == nil {
		return "", fmt.Errorf("interactive UI is not available")
	}
	return ui.Input(ctx, prompt, defaultValue)
}

// PasswordKey 使用语义 key 读取链式答案；缺失时回退到交互式密码输入。
func PasswordKey(ctx context.Context, ui PromptUI, key string, prompt string) (string, error) {
	if value, ok := PromptAnswer(ui, key); ok {
		return strings.TrimSpace(value), nil
	}
	if ui == nil {
		return "", fmt.Errorf("interactive UI is not available")
	}
	return ui.Password(ctx, prompt)
}

func (ui *chainPromptUI) Select(ctx context.Context, prompt string, options []SelectOption) (string, error) {
	base, err := ui.requireBase()
	if err != nil {
		return "", err
	}
	return base.Select(ctx, prompt, options)
}

func (ui *chainPromptUI) Confirm(ctx context.Context, prompt string, defaultValue bool) (bool, error) {
	base, err := ui.requireBase()
	if err != nil {
		return false, err
	}
	return base.Confirm(ctx, prompt, defaultValue)
}

func (ui *chainPromptUI) Input(ctx context.Context, prompt string, defaultValue string) (string, error) {
	base, err := ui.requireBase()
	if err != nil {
		return "", err
	}
	return base.Input(ctx, prompt, defaultValue)
}

func (ui *chainPromptUI) Password(ctx context.Context, prompt string) (string, error) {
	base, err := ui.requireBase()
	if err != nil {
		return "", err
	}
	return base.Password(ctx, prompt)
}

func (ui *chainPromptUI) Info(message string) error {
	base, err := ui.requireBase()
	if err != nil {
		return err
	}
	return base.Info(message)
}

func (ui *chainPromptUI) promptAnswer(key string) (string, bool) {
	key = normalizePromptKey(key)
	if value, ok := ui.answers[key]; ok {
		return value, true
	}
	if value, ok := wildcardPromptAnswer(ui.answers, key); ok {
		return value, true
	}
	return PromptAnswer(ui.base, key)
}

func (ui *chainPromptUI) requireBase() (PromptUI, error) {
	if ui == nil || ui.base == nil {
		return nil, fmt.Errorf("interactive UI is not available")
	}
	return ui.base, nil
}

func extractChainArgs(args []string) ([]string, map[string]string, error) {
	if len(args) == 0 {
		return args, nil, nil
	}
	cleaned := make([]string, 0, len(args))
	answers := map[string]string{}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			cleaned = append(cleaned, args[i:]...)
			break
		}
		if !strings.HasPrefix(arg, chainArgPrefix) {
			cleaned = append(cleaned, arg)
			continue
		}

		raw := strings.TrimPrefix(arg, chainArgPrefix)
		key, value, hasValue := strings.Cut(raw, "=")
		key = normalizePromptKey(key)
		if key == "" {
			return nil, nil, &UsageError{Message: "chain argument requires a key"}
		}
		if !hasValue {
			if i+1 >= len(args) || strings.HasPrefix(args[i+1], "--") {
				return nil, nil, &UsageError{Message: fmt.Sprintf("chain argument --chain.%s requires a value", key)}
			}
			value = args[i+1]
			i++
		}
		answers[key] = value
	}
	if len(answers) == 0 {
		return cleaned, nil, nil
	}
	return cleaned, answers, nil
}

func normalizePromptAnswers(values map[string]string) map[string]string {
	if len(values) == 0 {
		return nil
	}
	normalized := make(map[string]string, len(values))
	for key, value := range values {
		key = normalizePromptKey(key)
		if key == "" {
			continue
		}
		normalized[key] = value
	}
	return normalized
}

func mergePromptAnswers(base map[string]string, next map[string]string) map[string]string {
	if len(next) == 0 {
		return base
	}
	merged := normalizePromptAnswers(base)
	if merged == nil {
		merged = map[string]string{}
	}
	for key, value := range normalizePromptAnswers(next) {
		merged[key] = value
	}
	return merged
}

func normalizePromptKey(key string) string {
	return strings.ToLower(strings.TrimSpace(key))
}

func wildcardPromptAnswer(answers map[string]string, key string) (string, bool) {
	if len(answers) == 0 {
		return "", false
	}
	patterns := make([]string, 0, len(answers))
	for pattern := range answers {
		if !strings.ContainsAny(pattern, "*?[") {
			continue
		}
		if _, err := path.Match(pattern, key); err != nil {
			continue
		}
		patterns = append(patterns, pattern)
	}
	sort.SliceStable(patterns, func(i, j int) bool {
		left := wildcardPatternRank(patterns[i])
		right := wildcardPatternRank(patterns[j])
		if left.literalChars == right.literalChars {
			if left.wildcards == right.wildcards {
				return patterns[i] < patterns[j]
			}
			return left.wildcards < right.wildcards
		}
		return left.literalChars > right.literalChars
	})
	for _, pattern := range patterns {
		matched, _ := path.Match(pattern, key)
		if matched {
			return answers[pattern], true
		}
	}
	return "", false
}

type wildcardRank struct {
	literalChars int
	wildcards    int
}

func wildcardPatternRank(pattern string) wildcardRank {
	var rank wildcardRank
	for _, r := range pattern {
		switch r {
		case '*', '?', '[', ']':
			rank.wildcards++
		default:
			rank.literalChars++
		}
	}
	return rank
}

func resolveSelectAnswer(key string, answer string, options []SelectOption) (string, error) {
	if len(options) == 0 {
		return "", fmt.Errorf("select prompt requires at least one option")
	}
	answer = strings.TrimSpace(answer)
	if answer == "" {
		return options[0].Value, nil
	}
	if index, err := strconv.Atoi(answer); err == nil && index >= 1 && index <= len(options) {
		return options[index-1].Value, nil
	}
	for _, option := range options {
		if strings.EqualFold(answer, option.Value) || strings.EqualFold(answer, option.Label) {
			return option.Value, nil
		}
	}
	return "", fmt.Errorf("chain.%s has invalid value %q; expected one of: %s", normalizePromptKey(key), answer, selectOptionList(options))
}

func parseConfirmAnswer(key string, answer string, defaultValue bool) (bool, error) {
	answer = strings.ToLower(strings.TrimSpace(answer))
	if answer == "" {
		return defaultValue, nil
	}
	switch answer {
	case "y", "yes", "true", "1", "是":
		return true, nil
	case "n", "no", "false", "0", "否":
		return false, nil
	default:
		return false, fmt.Errorf("chain.%s must be a boolean value", normalizePromptKey(key))
	}
}

func selectOptionList(options []SelectOption) string {
	values := make([]string, 0, len(options))
	for _, option := range options {
		values = append(values, firstString(option.Value, option.Label))
	}
	return strings.Join(values, ", ")
}
