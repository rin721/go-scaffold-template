package app

import (
	"fmt"

	"github.com/rin721/go-scaffold2/internal/kernel/config"
)

type planState uint8

const (
	planOpen planState = iota
	planFrozen
)

// Plan 保存 composition 按人工顺序选择的底层组件。
type Plan struct {
	state      planState
	ids        map[ID]struct{}
	outputs    []any
	tokens     []*bindingToken
	components []RuntimeComponent
	configs    []config.Binding
	replaced   map[*bindingToken]struct{}
}

// NewPlan 创建尚未冻结的空组件计划。
func NewPlan() *Plan {
	return &Plan{ids: make(map[ID]struct{}), replaced: make(map[*bindingToken]struct{})}
}

// Add 把一个 Definition 原子加入当前 Plan。
func Add[O any](plan *Plan, definition Definition[O]) (Added[O], error) {
	if plan == nil {
		return Added[O]{}, fmt.Errorf("component plan is nil")
	}
	if plan.state != planOpen {
		return Added[O]{}, fmt.Errorf("component plan is frozen")
	}
	if definition.id == "" {
		return Added[O]{}, fmt.Errorf("component definition id is required")
	}
	if _, exists := plan.ids[definition.id]; exists {
		return Added[O]{}, fmt.Errorf("component %s is duplicated", definition.id)
	}
	index := len(plan.outputs)
	if definition.instantiate == nil {
		return Added[O]{}, fmt.Errorf("component %s definition is invalid", definition.id)
	}
	output, component, err := definition.instantiate(plan, index)
	if err != nil {
		return Added[O]{}, err
	}
	if isNil(output) {
		return Added[O]{}, fmt.Errorf("component %s output is nil", definition.id)
	}
	token := &bindingToken{}
	plan.ids[definition.id] = struct{}{}
	plan.outputs = append(plan.outputs, output)
	plan.tokens = append(plan.tokens, token)
	if component != nil {
		plan.components = append(plan.components, component)
	}
	if definition.configuration != nil {
		plan.configs = append(plan.configs, *definition.configuration)
	}
	binding := Binding[O]{plan: plan, index: index, token: token}
	return Added[O]{Binding: binding, Output: output}, nil
}

// Replace 把一个明确的替换声明绑定到同一 Plan 中已经存在的 typed target。
// Replacement 不发布第二份输出；调用方继续使用 target Binding 对应的稳定对象。
func Replace[T any](plan *Plan, target Binding[T], replacement ReplacementDefinition[T]) error {
	if plan == nil {
		return fmt.Errorf("component plan is nil")
	}
	if plan.state != planOpen {
		return fmt.Errorf("component plan is frozen")
	}
	if replacement.id == "" || replacement.instantiate == nil {
		return fmt.Errorf("replacement component definition is invalid")
	}
	if _, exists := plan.ids[replacement.id]; exists {
		return fmt.Errorf("component %s is duplicated", replacement.id)
	}
	if target.plan == nil || target.token == nil {
		return fmt.Errorf("replacement component %s target is a zero binding", replacement.id)
	}
	if target.plan != plan {
		return fmt.Errorf("replacement component %s target belongs to another plan", replacement.id)
	}
	if target.index < 0 || target.index >= len(plan.outputs) || plan.tokens[target.index] != target.token {
		return fmt.Errorf("replacement component %s target is not registered", replacement.id)
	}
	if _, exists := plan.replaced[target.token]; exists {
		return fmt.Errorf("replacement target component %d is already replaced", target.index)
	}
	resolved, ok := plan.outputs[target.index].(T)
	if !ok {
		return fmt.Errorf("replacement component %s target has an internal output type mismatch", replacement.id)
	}
	component, err := replacement.instantiate(plan, len(plan.outputs), resolved)
	if err != nil {
		return err
	}
	if component == nil {
		return fmt.Errorf("replacement component %s runtime node is nil", replacement.id)
	}

	plan.ids[replacement.id] = struct{}{}
	plan.replaced[target.token] = struct{}{}
	plan.components = append(plan.components, component)
	if replacement.configuration != nil {
		plan.configs = append(plan.configs, *replacement.configuration)
	}
	return nil
}

// FrozenPlan 是完成校验后的不可变 Kernel 安装输入。
type FrozenPlan struct {
	valid      bool
	components []RuntimeComponent
	configs    []config.Binding
}

// Freeze 封存 Plan。成功后不能继续 Add。
func (p *Plan) Freeze() (FrozenPlan, error) {
	if p == nil {
		return FrozenPlan{}, fmt.Errorf("component plan is nil")
	}
	if p.state != planOpen {
		return FrozenPlan{}, fmt.Errorf("component plan is already frozen")
	}
	p.state = planFrozen
	return FrozenPlan{
		valid:      true,
		components: append([]RuntimeComponent(nil), p.components...),
		configs:    append([]config.Binding(nil), p.configs...),
	}, nil
}

// Validate 确认 FrozenPlan 来自一次成功的 Freeze，且运行节点完整。
func (p FrozenPlan) Validate() error {
	if !p.valid {
		return fmt.Errorf("component plan is not frozen")
	}
	for index, component := range p.components {
		if component == nil {
			return fmt.Errorf("component plan runtime node %d is nil", index)
		}
	}
	return nil
}

// Components 返回供 Kernel 安装的组件副本。
func (p FrozenPlan) Components() []RuntimeComponent {
	return append([]RuntimeComponent(nil), p.components...)
}

// Configurations 返回当前组件真实贡献的配置节契约副本。
func (p FrozenPlan) Configurations() []config.Binding {
	return append([]config.Binding(nil), p.configs...)
}
