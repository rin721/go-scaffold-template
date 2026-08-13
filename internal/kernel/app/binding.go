package app

import (
	"fmt"
	"sync/atomic"
)

type bindingToken struct{}

// Added 是 Definition 成功加入 Plan 后得到的 typed 装配结果。
type Added[O any] struct {
	Binding Binding[O]
	Output  O
}

// Binding 只声明一个已选择组件的 typed 输出，不提供运行时取值能力。
type Binding[O any] struct {
	plan  *Plan
	index int
	token *bindingToken
}

// Input 是后续组件对一个前置 Binding 的 typed 依赖声明。
type Input[O any] struct {
	binding Binding[O]
}

// InputOf 把已选择 Binding 转成后续组件的 typed 输入。
func InputOf[O any](binding Binding[O]) Input[O] {
	return Input[O]{binding: binding}
}

type reference struct {
	plan  *Plan
	index int
	token *bindingToken
}

// Dependency 是只能由本包 typed Input 实现的依赖声明。
type Dependency interface {
	dependencyReference() reference
}

func (input Input[O]) dependencyReference() reference {
	return reference{plan: input.binding.plan, index: input.binding.index, token: input.binding.token}
}

// Values 是组件 Builder 执行期间对已声明输入的只读视图。
// 它没有按字符串或类型任意查询的入口。
type Values struct {
	plan    *Plan
	allowed map[int]*bindingToken
	active  *atomic.Bool
}

// Resolve 取得一个已在当前组件 Dependencies 中声明的 typed 输入。
func Resolve[O any](values Values, input Input[O]) (O, error) {
	var zero O
	if values.active == nil || !values.active.Load() {
		return zero, fmt.Errorf("component input view is no longer active")
	}
	reference := input.dependencyReference()
	if values.plan == nil || reference.plan != values.plan {
		return zero, fmt.Errorf("component input belongs to another plan")
	}
	if reference.token == nil || values.allowed[reference.index] != reference.token {
		return zero, fmt.Errorf("component input %d was not declared", reference.index)
	}
	if reference.index < 0 || reference.index >= len(reference.plan.outputs) {
		return zero, fmt.Errorf("component input %d is outside the plan", reference.index)
	}
	resolved, ok := reference.plan.outputs[reference.index].(O)
	if !ok {
		return zero, fmt.Errorf("component input %d has an internal output type mismatch", reference.index)
	}
	return resolved, nil
}

// Dependencies 把前置 Input 解码为组件 Builder 需要的 typed 依赖 D。
type Dependencies[D any] struct {
	references []reference
	decode     func(Values) (D, error)
}

// DependencySet 创建显式 typed 依赖集合。
func DependencySet[D any](decode func(Values) (D, error), inputs ...Dependency) (Dependencies[D], error) {
	if decode == nil {
		return Dependencies[D]{}, fmt.Errorf("component dependencies decoder is nil")
	}
	references := make([]reference, 0, len(inputs))
	for index, input := range inputs {
		if input == nil {
			return Dependencies[D]{}, fmt.Errorf("component dependency %d is nil", index)
		}
		references = append(references, input.dependencyReference())
	}
	return Dependencies[D]{references: references, decode: decode}, nil
}

// FixedDependencies 创建不引用其他 App Binding 的显式固定依赖。
func FixedDependencies[D any](value D) Dependencies[D] {
	return Dependencies[D]{decode: func(Values) (D, error) { return value, nil }}
}

func (dependencies Dependencies[D]) resolve(plan *Plan) (D, error) {
	var zero D
	if dependencies.decode == nil {
		return zero, fmt.Errorf("component dependencies decoder is nil")
	}
	allowed := make(map[int]*bindingToken, len(dependencies.references))
	for _, reference := range dependencies.references {
		allowed[reference.index] = reference.token
	}
	active := &atomic.Bool{}
	active.Store(true)
	defer active.Store(false)
	return dependencies.decode(Values{plan: plan, allowed: allowed, active: active})
}

func (dependencies Dependencies[D]) validate(plan *Plan, nextIndex int) error {
	return dependencies.validatePhase(plan, nextIndex, Runtime)
}

func (dependencies Dependencies[D]) validatePhase(plan *Plan, nextIndex int, maximum BuiltinPhase) error {
	for index, reference := range dependencies.references {
		if reference.plan == nil || reference.token == nil {
			return fmt.Errorf("dependency %d is a zero input", index)
		}
		if reference.plan != plan {
			return fmt.Errorf("dependency %d belongs to another plan", index)
		}
		if reference.index < 0 || reference.index >= nextIndex {
			return fmt.Errorf("dependency %d is not an earlier component", index)
		}
		if plan.tokens[reference.index] != reference.token {
			return fmt.Errorf("dependency %d binding is not registered", index)
		}
		if reference.index >= len(plan.outputPhases) || plan.outputPhases[reference.index] > maximum {
			return fmt.Errorf("dependency %d belongs to phase %d after maximum phase %d", index, plan.outputPhases[reference.index], maximum)
		}
	}
	return nil
}

func (dependencies Dependencies[D]) markBuiltinConsumers(plan *Plan) {
	for _, reference := range dependencies.references {
		if role, ok := plan.rootRoles[reference.index]; ok {
			if typed, supported := role.(interface{ markConsumer() }); supported {
				typed.markConsumer()
			}
		}
	}
}
