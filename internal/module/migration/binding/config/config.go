// Package configbinding 绑定 Migration module 的有界执行配置。
package configbinding

import (
	"context"
	"fmt"
	"time"

	"github.com/rin721/go-scaffold-template/internal/kernel/config"
)

const (
	capabilityID = "module.migration"
	configPath   = "migration"
)

// Config 控制锁等待和单次命令的总期限。
type Config struct {
	LockTimeout      time.Duration `mapstructure:"lockTimeout"`
	OperationTimeout time.Duration `mapstructure:"operationTimeout"`
}

// Default 返回本地与 CI 共用的显式预算。
func Default() Config {
	return Config{LockTimeout: 15 * time.Second, OperationTimeout: 2 * time.Minute}
}

// Binding 返回 Migration module 唯一配置声明。
func Binding() config.Binding {
	return config.Binding{
		CapabilityID: capabilityID, ConfigPath: configPath, Contract: defaults{},
		Validate: func(snapshot config.Snapshot) error {
			_, err := Decode(snapshot)
			return err
		},
	}
}

// Decode 解码并校验 deadline 层级。
func Decode(snapshot config.Snapshot) (Config, error) {
	resolved := Default()
	if err := snapshot.DecodeSection(configPath, &resolved); err != nil {
		return Config{}, fmt.Errorf("decode migration configuration: %w", err)
	}
	if resolved.LockTimeout <= 0 || resolved.OperationTimeout <= 0 || resolved.LockTimeout >= resolved.OperationTimeout {
		return Config{}, fmt.Errorf("migration budgets require 0 < lockTimeout < operationTimeout")
	}
	return resolved, nil
}

type defaults struct{}

func (defaults) Defaults(ctx context.Context) (config.Object, config.Control, error) {
	if ctx == nil {
		return nil, config.Continue, fmt.Errorf("migration defaults context is nil")
	}
	if err := ctx.Err(); err != nil {
		return nil, config.Continue, err
	}
	value := Default()
	return config.Object{
		config.FieldOf("lockTimeout", config.Duration(value.LockTimeout)),
		config.FieldOf("operationTimeout", config.Duration(value.OperationTimeout)),
	}, config.Continue, nil
}

var _ config.DefaultContract = defaults{}
