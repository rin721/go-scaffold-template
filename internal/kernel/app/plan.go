package app

import (
	"fmt"

	kernelcli "github.com/rin721/go-scaffold2/internal/kernel/cli"
	"github.com/rin721/go-scaffold2/internal/kernel/config"
)

type planState uint8

const (
	planOpen planState = iota
	planFrozen
)

// Plan 保存 composition 按人工顺序选择的底层组件。
type Plan struct {
	state        planState
	ids          map[ID]struct{}
	outputs      []any
	tokens       []*bindingToken
	components   []RuntimeComponent
	defaults     []config.Binding
	cli          []kernelcli.Contract
	configPaths  map[string]ID
	roles        map[RoleID]builtinRuntime
	roleOrder    []builtinRuntime
	rootRoles    map[int]builtinRuntime
	outputPhases []BuiltinPhase
}

// NewPlan 创建尚未冻结的空组件计划。
func NewPlan() *Plan {
	return &Plan{
		ids: make(map[ID]struct{}), configPaths: make(map[string]ID),
		roles: make(map[RoleID]builtinRuntime), rootRoles: make(map[int]builtinRuntime),
	}
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
	if err := plan.validateIdentity(definition.id, definition.configPath); err != nil {
		return Added[O]{}, err
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
	if definition.configPath != "" {
		plan.configPaths[definition.configPath] = definition.id
	}
	if definition.markConsumers != nil {
		definition.markConsumers(plan)
	}
	plan.outputs = append(plan.outputs, output)
	plan.tokens = append(plan.tokens, token)
	plan.outputPhases = append(plan.outputPhases, Runtime)
	if component != nil {
		plan.components = append(plan.components, component)
	}
	if definition.defaults != nil {
		path := ""
		if configured, ok := component.(interface{ ConfigPath() string }); ok {
			path = configured.ConfigPath()
		}
		plan.defaults = append(plan.defaults, config.Binding{
			CapabilityID: string(definition.id),
			ConfigPath:   path,
			Contract:     definition.defaults,
		})
	}
	plan.cli = append(plan.cli, definition.cli...)
	binding := Binding[O]{plan: plan, index: index, token: token}
	return Added[O]{Binding: binding, Output: output}, nil
}

func (p *Plan) validateIdentity(id ID, path string) error {
	if id == "" {
		return fmt.Errorf("component id is required")
	}
	if _, exists := p.ids[id]; exists {
		return fmt.Errorf("component %s is duplicated", id)
	}
	if path == "" {
		return nil
	}
	if err := validateConfigPath(path); err != nil {
		return fmt.Errorf("component %s config path: %w", id, err)
	}
	for existing, owner := range p.configPaths {
		if configPathsOverlap(existing, path) {
			return fmt.Errorf("component %s config path %s overlaps component %s path %s", id, path, owner, existing)
		}
	}
	return nil
}

// FrozenPlan 是完成校验后的不可变 Kernel 安装输入。
type FrozenPlan struct {
	valid      bool
	origin     *Plan
	components []RuntimeComponent
	defaults   []config.Binding
	cli        []kernelcli.Contract
}

// Freeze 封存 Plan。成功后不能继续 Add。
func (p *Plan) Freeze() (FrozenPlan, error) {
	if p == nil {
		return FrozenPlan{}, fmt.Errorf("component plan is nil")
	}
	if p.state != planOpen {
		return FrozenPlan{}, fmt.Errorf("component plan is already frozen")
	}
	for _, role := range p.roleOrder {
		if err := role.validateRuntime(); err != nil {
			return FrozenPlan{}, fmt.Errorf("validate builtin role %s: %w", role.roleID(), err)
		}
	}
	p.state = planFrozen
	return FrozenPlan{
		valid:      true,
		origin:     p,
		components: append([]RuntimeComponent(nil), p.components...),
		defaults:   append([]config.Binding(nil), p.defaults...),
		cli:        append([]kernelcli.Contract(nil), p.cli...),
	}, nil
}

// BelongsTo 确认 FrozenPlan 来自指定 Plan，供 Assembly 阻止跨计划安装。
func (p FrozenPlan) BelongsTo(plan *Plan) bool { return p.valid && p.origin == plan }

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

// Defaults 返回当前组件真实贡献的默认配置绑定副本。
func (p FrozenPlan) Defaults() []config.Binding {
	return append([]config.Binding(nil), p.defaults...)
}

// CLIContracts 返回当前组件真实贡献的 CLI 契约副本。
func (p FrozenPlan) CLIContracts() []kernelcli.Contract {
	return append([]kernelcli.Contract(nil), p.cli...)
}
