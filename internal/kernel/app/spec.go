package app

import (
	"fmt"
	"strings"
)

// Spec 明确声明一个 App 实例的组件身份和配置所有权。
type Spec struct {
	ID         ID
	ConfigPath string
}

// ValidateConfigured 校验配置化实例规格。
func (s Spec) ValidateConfigured() error {
	if s.ID == "" {
		return fmt.Errorf("component id is required")
	}
	if err := validateConfigPath(s.ConfigPath); err != nil {
		return fmt.Errorf("component %s config path: %w", s.ID, err)
	}
	return nil
}

func validateConfigPath(path string) error {
	if path == "" {
		return fmt.Errorf("is required")
	}
	parts := strings.Split(path, ".")
	for _, part := range parts {
		if strings.TrimSpace(part) == "" || part != strings.TrimSpace(part) {
			return fmt.Errorf("%q contains an empty or padded segment", path)
		}
	}
	return nil
}

func configPathsOverlap(left, right string) bool {
	if left == right {
		return true
	}
	return strings.HasPrefix(left, right+".") || strings.HasPrefix(right, left+".")
}
