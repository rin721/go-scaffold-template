// Package configbinding 绑定 Todo 的默认配置、严格解码与业务策略。
package configbinding

import (
	"context"
	"fmt"

	"github.com/rin721/go-scaffold-template/internal/kernel/config"
	"github.com/rin721/go-scaffold-template/internal/module/todo/service"
)

const (
	// CapabilityID 是 Todo 配置 owner 的稳定 ID。
	CapabilityID = "module.todo"
	// ConfigPath 是 Todo 在统一配置候选中的路径。
	ConfigPath          = "todo"
	maxSchemaTitleRunes = 200
)

// Config 是 Todo 的 typed 配置。
type Config struct {
	TitleMaxRunes    int `mapstructure:"titleMaxRunes"`
	DefaultListLimit int `mapstructure:"defaultListLimit"`
	MaxListLimit     int `mapstructure:"maxListLimit"`
}

// Default 返回安全且可直接运行的 Todo 默认配置。
func Default() Config {
	return Config{TitleMaxRunes: 120, DefaultListLimit: 20, MaxListLimit: 100}
}

// Binding 返回 Todo application-owned 配置节契约。
func Binding() config.Binding {
	return config.Binding{
		CapabilityID: CapabilityID,
		ConfigPath:   ConfigPath,
		Contract:     defaults{},
		Validate: func(snapshot config.Snapshot) error {
			_, err := Decode(snapshot)
			return err
		},
	}
}

// Decode 从统一不可变候选严格解码 Todo 配置。
func Decode(snapshot config.Snapshot) (Config, error) {
	resolved := Default()
	if err := snapshot.DecodeSection(ConfigPath, &resolved); err != nil {
		return Config{}, fmt.Errorf("decode todo configuration: %w", err)
	}
	if err := Validate(resolved); err != nil {
		return Config{}, err
	}
	return resolved, nil
}

// Validate 校验 Todo 配置的跨字段不变量。
func Validate(value Config) error {
	if value.TitleMaxRunes <= 0 || value.TitleMaxRunes > maxSchemaTitleRunes {
		return fmt.Errorf("todo titleMaxRunes must be between 1 and %d", maxSchemaTitleRunes)
	}
	if value.DefaultListLimit <= 0 || value.MaxListLimit <= 0 {
		return fmt.Errorf("todo list limits must be positive")
	}
	if value.DefaultListLimit > value.MaxListLimit {
		return fmt.Errorf("todo defaultListLimit cannot exceed maxListLimit")
	}
	return nil
}

// Policy 转换为 Service 使用的协议无关策略。
func (c Config) Policy() service.Policy {
	return service.Policy{
		TitleMaxRunes: c.TitleMaxRunes, DefaultListLimit: c.DefaultListLimit, MaxListLimit: c.MaxListLimit,
	}
}

type defaults struct{}

func (defaults) Defaults(ctx context.Context) (config.Object, config.Control, error) {
	if ctx == nil {
		return nil, config.Continue, fmt.Errorf("todo defaults context is nil")
	}
	if err := ctx.Err(); err != nil {
		return nil, config.Continue, err
	}
	value := Default()
	title, err := config.Number(fmt.Sprint(value.TitleMaxRunes))
	if err != nil {
		return nil, config.Continue, err
	}
	defaultLimit, err := config.Number(fmt.Sprint(value.DefaultListLimit))
	if err != nil {
		return nil, config.Continue, err
	}
	maxLimit, err := config.Number(fmt.Sprint(value.MaxListLimit))
	if err != nil {
		return nil, config.Continue, err
	}
	return config.Object{
		config.FieldOf("titleMaxRunes", title),
		config.FieldOf("defaultListLimit", defaultLimit),
		config.FieldOf("maxListLimit", maxLimit),
	}, config.Continue, nil
}

var _ config.DefaultContract = defaults{}
