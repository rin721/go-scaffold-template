// Package app 定义底层应用组件的声明、显式装配和运行契约。
package app

import (
	"context"
	"fmt"

	"github.com/rin721/go-scaffold2/internal/kernel/config"
)

// ID 是一个底层组件角色在当前进程中的稳定标识。
type ID string

// Decoder 从完整配置快照解码并校验组件自己的 typed 配置。
// Decoder 不得打开资源或启动 goroutine。
type Decoder[C any] func(config.Snapshot) (C, error)

// Builder 创建一个尚未发布的组件实例。
type Builder[C, D, I any] func(context.Context, C, D) (I, error)

// ReloadPolicy 表示组件配置在当前进程中的生效方式。
type ReloadPolicy uint8

const (
	// NoReload 表示组件没有运行期配置，不参与配置变更事务。
	NoReload ReloadPolicy = iota
	// KernelInstanceSwap 表示 Kernel 通过候选实例和租约排空完成换代。
	KernelInstanceSwap
	// RestartRequired 表示相关配置只能在下次进程启动时生效。
	RestartRequired
)

// ConfiguredSource 描述一个有运行期配置的构造源。
type ConfiguredSource[C any] struct {
	path     string
	decode   Decoder[C]
	defaults config.DefaultContract
}

func (s ConfiguredSource[C]) binding(id ID) config.Binding {
	return config.Binding{
		CapabilityID: string(id),
		ConfigPath:   s.path,
		Contract:     s.defaults,
		Validate: func(snapshot config.Snapshot) error {
			_, err := s.decode(snapshot)
			return err
		},
	}
}

// Configured 创建 typed 配置源。defaults 可以为 nil，表示不生成默认配置段。
func Configured[C any](path string, decode Decoder[C], defaults config.DefaultContract) (ConfiguredSource[C], error) {
	if path == "" {
		return ConfiguredSource[C]{}, fmt.Errorf("component config path is required")
	}
	if decode == nil {
		return ConfiguredSource[C]{}, fmt.Errorf("component config decoder is nil")
	}
	if isNil(defaults) {
		defaults = nil
	}
	return ConfiguredSource[C]{path: path, decode: decode, defaults: defaults}, nil
}

type lifecycle[I any] struct {
	start      func(context.Context, I) error
	ready      func(context.Context, I) error
	stop       func(context.Context, I) error
	activate   func(I)
	deactivate func(I)
}

// Option 为 Managed 组件按需附加真实生命周期契约。
type Option[I any] func(*lifecycle[I]) error

// WithStart 声明实例启动动作。
func WithStart[I any](start func(context.Context, I) error) Option[I] {
	return func(target *lifecycle[I]) error {
		if start == nil {
			return fmt.Errorf("component start function is nil")
		}
		if target.start != nil {
			return fmt.Errorf("component start function is duplicated")
		}
		target.start = start
		return nil
	}
}

// WithReady 声明实例发布前的就绪门禁。
func WithReady[I any](ready func(context.Context, I) error) Option[I] {
	return func(target *lifecycle[I]) error {
		if ready == nil {
			return fmt.Errorf("component ready function is nil")
		}
		if target.ready != nil {
			return fmt.Errorf("component ready function is duplicated")
		}
		target.ready = ready
		return nil
	}
}

// WithStop 声明实例资源释放动作。
func WithStop[I any](stop func(context.Context, I) error) Option[I] {
	return func(target *lifecycle[I]) error {
		if stop == nil {
			return fmt.Errorf("component stop function is nil")
		}
		if target.stop != nil {
			return fmt.Errorf("component stop function is duplicated")
		}
		target.stop = stop
		return nil
	}
}

// Lease 是 app 组件用来收敛稳定租约输出的最小入口。
// 具体组件必须把 I 继续收窄为不含关闭权的项目接口。
type Lease[I any] interface {
	Use(context.Context, func(I) error) error
}

// Exposure 把 Kernel 私有实例 I 适配为稳定输出 O。
type Exposure[I, O any] func(Lease[I]) (O, error)

// Leased 创建稳定租约输出策略。
func Leased[I, O any](adapt Exposure[I, O]) Exposure[I, O] {
	return adapt
}
