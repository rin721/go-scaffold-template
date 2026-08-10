package i18n

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	goi18n "github.com/nicksnyder/go-i18n/v2/i18n"
	"golang.org/x/text/language"
	"gopkg.in/yaml.v3"
)

func resolveConfig(cfg *Config) (resolvedConfig, error) {
	defaultLanguage := DefaultLanguage
	if cfg != nil && cfg.DefaultLanguage != "" {
		defaultLanguage = cfg.DefaultLanguage
	}

	defaultTag, err := language.Parse(defaultLanguage)
	if err != nil {
		return resolvedConfig{}, fmt.Errorf("parse default language %q: %w", defaultLanguage, err)
	}

	resolved := resolvedConfig{
		DefaultLanguage: defaultTag,
		MessageFS:       os.DirFS("."),
		MissingBehavior: DefaultMissingBehavior,
	}
	if cfg == nil {
		return resolved, nil
	}

	if len(cfg.MessageFiles) > 0 {
		resolved.MessageFiles = cloneStrings(cfg.MessageFiles)
	}
	if cfg.MessageFS != nil {
		resolved.MessageFS = cfg.MessageFS
	}
	if cfg.MissingBehavior != "" {
		behavior, err := normalizeMissingBehavior(cfg.MissingBehavior)
		if err != nil {
			return resolvedConfig{}, err
		}
		resolved.MissingBehavior = behavior
	}

	for _, path := range resolved.MessageFiles {
		if err := validateMessageFile(path); err != nil {
			return resolvedConfig{}, err
		}
	}

	return resolved, nil
}

func buildBundle(cfg resolvedConfig) (*goi18n.Bundle, error) {
	bundle := goi18n.NewBundle(cfg.DefaultLanguage)
	bundle.RegisterUnmarshalFunc(messageFormatYAML, yaml.Unmarshal)
	bundle.RegisterUnmarshalFunc(messageFormatYML, yaml.Unmarshal)

	for _, path := range cfg.MessageFiles {
		if _, err := bundle.LoadMessageFileFS(cfg.MessageFS, filepath.ToSlash(path)); err != nil {
			return nil, fmt.Errorf("load message file %q: %w", path, err)
		}
	}

	return bundle, nil
}

func normalizeMissingBehavior(behavior MissingBehavior) (MissingBehavior, error) {
	switch MissingBehavior(strings.ToLower(string(behavior))) {
	case MissingBehaviorError:
		return MissingBehaviorError, nil
	case MissingBehaviorUseID:
		return MissingBehaviorUseID, nil
	default:
		return "", fmt.Errorf("unsupported missing behavior %q", behavior)
	}
}

func validateMessageFile(path string) error {
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("message file path is empty")
	}

	switch strings.ToLower(filepath.Ext(path)) {
	case fileExtensionJSON, fileExtensionYAML, fileExtensionYML:
		return nil
	default:
		return fmt.Errorf("unsupported message file extension %q for %q", filepath.Ext(path), path)
	}
}

func cloneStrings(values []string) []string {
	cloned := make([]string, len(values))
	copy(cloned, values)
	return cloned
}

func isMissingMessageError(err error) bool {
	var missing *goi18n.MessageNotFoundErr
	return errors.As(err, &missing)
}
