package app

import (
	"fmt"
	"reflect"

	kernelcli "github.com/rin721/go-scaffold2/internal/kernel/cli"
	"github.com/rin721/go-scaffold2/internal/kernel/config"
)

// Definition 是一个无安装副作用的底层组件声明。
type Definition[O any] struct {
	id          ID
	defaults    config.DefaultContract
	cli         []kernelcli.Contract
	instantiate func(*Plan, int) (O, RuntimeComponent, error)
}

// Value 创建代码固定、直接输出、无配置和无生命周期的组件声明。
func Value[O any](id ID, output O) (Definition[O], error) {
	if id == "" {
		return Definition[O]{}, fmt.Errorf("component id is required")
	}
	if isNil(output) {
		return Definition[O]{}, fmt.Errorf("component %s direct output is nil", id)
	}
	return Definition[O]{
		id: id,
		instantiate: func(*Plan, int) (O, RuntimeComponent, error) {
			return output, nil, nil
		},
	}, nil
}

// ManagedConfigured 创建一个由 typed 配置构造并通过稳定租约输出的组件。
func ManagedConfigured[C, D, I, O any](
	id ID,
	source ConfiguredSource[C],
	dependencies Dependencies[D],
	build Builder[C, D, I],
	exposure Exposure[I, O],
	reload ReloadPolicy,
	options ...Option[I],
) (Definition[O], error) {
	if source.path == "" || source.decode == nil {
		return Definition[O]{}, fmt.Errorf("component %s configured source is invalid", id)
	}
	if reload != KernelInstanceSwap && reload != RestartRequired {
		return Definition[O]{}, fmt.Errorf("component %s configured reload policy %d is invalid", id, reload)
	}
	return newManagedDefinition(
		id,
		source.path,
		source.decode,
		nil,
		source.defaults,
		dependencies,
		build,
		exposure,
		reload,
		options...,
	)
}

// ManagedFixed 创建一个无运行期配置但需要 Kernel 生命周期治理的租约组件。
func ManagedFixed[C, D, I, O any](
	id ID,
	configuration C,
	dependencies Dependencies[D],
	build Builder[C, D, I],
	exposure Exposure[I, O],
	options ...Option[I],
) (Definition[O], error) {
	return newManagedDefinition(
		id,
		"",
		nil,
		func() (C, error) { return configuration, nil },
		nil,
		dependencies,
		build,
		exposure,
		NoReload,
		options...,
	)
}

func newManagedDefinition[C, D, I, O any](
	id ID,
	path string,
	decode Decoder[C],
	fixed func() (C, error),
	defaults config.DefaultContract,
	dependencies Dependencies[D],
	build Builder[C, D, I],
	exposure Exposure[I, O],
	reload ReloadPolicy,
	options ...Option[I],
) (Definition[O], error) {
	if id == "" {
		return Definition[O]{}, fmt.Errorf("component id is required")
	}
	if build == nil {
		return Definition[O]{}, fmt.Errorf("component %s builder is nil", id)
	}
	if exposure == nil {
		return Definition[O]{}, fmt.Errorf("component %s exposure is nil", id)
	}
	if dependencies.decode == nil {
		return Definition[O]{}, fmt.Errorf("component %s dependencies decoder is nil", id)
	}
	lifecycle := lifecycle[I]{}
	for index, option := range options {
		if option == nil {
			return Definition[O]{}, fmt.Errorf("component %s option %d is nil", id, index)
		}
		if err := option(&lifecycle); err != nil {
			return Definition[O]{}, fmt.Errorf("component %s option %d: %w", id, index, err)
		}
	}
	definition := Definition[O]{
		id:       id,
		defaults: defaults,
		cli:      append([]kernelcli.Contract(nil), lifecycle.cli...),
	}
	definition.instantiate = func(plan *Plan, index int) (O, RuntimeComponent, error) {
		var zero O
		if err := dependencies.validate(plan, index); err != nil {
			return zero, nil, fmt.Errorf("component %s dependencies: %w", id, err)
		}
		lease := newLease[I]()
		output, err := exposure(lease)
		if err != nil {
			return zero, nil, fmt.Errorf("component %s exposure: %w", id, err)
		}
		if isNil(output) {
			return zero, nil, fmt.Errorf("component %s exposure output is nil", id)
		}
		component := &managedComponent[C, D, I]{
			componentID: id,
			configPath:  path,
			policy:      reload,
			decode:      decode,
			fixed:       fixed,
			resolveDependencies: func() (D, error) {
				return dependencies.resolve(plan)
			},
			build:     build,
			lifecycle: lifecycle,
			lease:     lease,
		}
		return output, component, nil
	}
	return definition, nil
}

func isNil(value any) bool {
	if value == nil {
		return true
	}
	reflected := reflect.ValueOf(value)
	switch reflected.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return reflected.IsNil()
	default:
		return false
	}
}
