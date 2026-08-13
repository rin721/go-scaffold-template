package app

import (
	"context"
	"errors"
	"fmt"
)

// RoleID 是 Kernel 封闭内置能力目录中的稳定身份。
type RoleID string

// BuiltinPhase 表示内置能力在 Kernel Assembly 中的构造阶段。
type BuiltinPhase uint8

const (
	// Bootstrap 在首次配置加载和普通 App 构建前完成。
	Bootstrap BuiltinPhase = iota
	// PreStart 在 Plan 冻结后、Kernel 启动前完成。
	PreStart
	// Runtime 与普通 RuntimeComponent 一同治理。
	Runtime
)

// BuiltinActivation 表示内置能力是否必须在所属阶段激活。
type BuiltinActivation uint8

const (
	// RequiredActivation 要求 Assembly 始终构造 baseline。
	RequiredActivation BuiltinActivation = iota
	// SelectedActivation 只在进程模式明确选择后构造 baseline。
	SelectedActivation
)

// BuiltinVisibility 表示内置输出是否可注入普通 App。
type BuiltinVisibility uint8

const (
	// KernelOnly 只允许 Assembly 和 Kernel 使用输出。
	KernelOnly BuiltinVisibility = iota
	// AppVisible 允许 Assembly 向 Composition 返回 typed root Binding。
	AppVisible
)

// ReplacementPolicy 描述内置 Role 的替换时机。
type ReplacementPolicy uint8

const (
	// Fixed 只允许 baseline。
	Fixed ReplacementPolicy = iota
	// StartupReplace 在所属阶段首次使用前冻结目标。
	StartupReplace
	// RuntimeTransaction 通过稳定 slot 完成运行期换代。
	RuntimeTransaction
)

// BaselineOwnership 表示 baseline 最终由谁释放。
type BaselineOwnership uint8

const (
	// BorrowedBaseline 由提供方负责最终释放。
	BorrowedBaseline BaselineOwnership = iota
	// AssemblyOwnedBaseline 由 Assembly 在 Runtime 停止后释放。
	AssemblyOwnedBaseline
)

// BuiltinDefinition 是 Kernel catalog 中一项内置能力的完整声明。
// 字段保持私有，普通 Composition 只能使用 Assembly 返回的 Role handle。
type BuiltinDefinition[TTarget, TOutput any] struct {
	id         RoleID
	phase      BuiltinPhase
	activation BuiltinActivation
	visibility BuiltinVisibility
	policy     ReplacementPolicy
	ownership  BaselineOwnership
	build      func() (TTarget, error)
	expose     Exposure[TTarget, TOutput]
	project    func(TTarget) (TOutput, error)
	stop       func(context.Context, TTarget) error
}

// RuntimeBuiltin 建立具有稳定 slot 的运行期可替换内置声明。
func RuntimeBuiltin[TTarget, TOutput any](
	id RoleID,
	phase BuiltinPhase,
	activation BuiltinActivation,
	visibility BuiltinVisibility,
	ownership BaselineOwnership,
	build func() (TTarget, error),
	expose Exposure[TTarget, TOutput],
	stop func(context.Context, TTarget) error,
) (BuiltinDefinition[TTarget, TOutput], error) {
	definition := BuiltinDefinition[TTarget, TOutput]{
		id: id, phase: phase, activation: activation, visibility: visibility,
		policy: RuntimeTransaction, ownership: ownership, build: build, expose: expose, stop: stop,
	}
	if err := definition.validate(); err != nil {
		return BuiltinDefinition[TTarget, TOutput]{}, err
	}
	return definition, nil
}

// BorrowedRuntimeBuiltin 建立由外部提供方保留关闭权的测试或嵌入式 baseline。
func BorrowedRuntimeBuiltin[TTarget, TOutput any](
	id RoleID,
	phase BuiltinPhase,
	activation BuiltinActivation,
	visibility BuiltinVisibility,
	build func() (TTarget, error),
	expose Exposure[TTarget, TOutput],
) (BuiltinDefinition[TTarget, TOutput], error) {
	return RuntimeBuiltin(id, phase, activation, visibility, BorrowedBaseline, build, expose, nil)
}

// StartupBuiltin 建立在所属阶段冻结目标的内置声明。
func StartupBuiltin[TTarget, TOutput any](
	id RoleID,
	phase BuiltinPhase,
	activation BuiltinActivation,
	visibility BuiltinVisibility,
	ownership BaselineOwnership,
	build func() (TTarget, error),
	project func(TTarget) (TOutput, error),
	stop func(context.Context, TTarget) error,
) (BuiltinDefinition[TTarget, TOutput], error) {
	definition := BuiltinDefinition[TTarget, TOutput]{
		id: id, phase: phase, activation: activation, visibility: visibility,
		policy: StartupReplace, ownership: ownership, build: build, project: project, stop: stop,
	}
	if err := definition.validate(); err != nil {
		return BuiltinDefinition[TTarget, TOutput]{}, err
	}
	return definition, nil
}

// DeferredStartupBuiltin 建立由 Assembly 在激活 target 后完成最终输出构造的内置声明。
// 它只适用于 SelectedActivation + KernelOnly，例如需要冻结 Contract 集合后再构造 App 的 CLI。
func DeferredStartupBuiltin[TTarget, TOutput any](
	id RoleID,
	phase BuiltinPhase,
	ownership BaselineOwnership,
	build func() (TTarget, error),
	stop func(context.Context, TTarget) error,
) (BuiltinDefinition[TTarget, TOutput], error) {
	definition := BuiltinDefinition[TTarget, TOutput]{
		id: id, phase: phase, activation: SelectedActivation, visibility: KernelOnly,
		policy: StartupReplace, ownership: ownership, build: build, stop: stop,
	}
	if err := definition.validate(); err != nil {
		return BuiltinDefinition[TTarget, TOutput]{}, err
	}
	return definition, nil
}

// FixedBuiltin 建立只允许 baseline、拒绝任何 Replace 的内置声明。
func FixedBuiltin[TTarget, TOutput any](
	id RoleID,
	phase BuiltinPhase,
	activation BuiltinActivation,
	visibility BuiltinVisibility,
	ownership BaselineOwnership,
	build func() (TTarget, error),
	project func(TTarget) (TOutput, error),
	stop func(context.Context, TTarget) error,
) (BuiltinDefinition[TTarget, TOutput], error) {
	definition := BuiltinDefinition[TTarget, TOutput]{
		id: id, phase: phase, activation: activation, visibility: visibility,
		policy: Fixed, ownership: ownership, build: build, project: project, stop: stop,
	}
	if err := definition.validate(); err != nil {
		return BuiltinDefinition[TTarget, TOutput]{}, err
	}
	return definition, nil
}

func (d BuiltinDefinition[TTarget, TOutput]) validate() error {
	if d.id == "" {
		return fmt.Errorf("builtin role id is required")
	}
	if d.phase > Runtime {
		return fmt.Errorf("builtin role %s phase %d is invalid", d.id, d.phase)
	}
	if d.activation > SelectedActivation {
		return fmt.Errorf("builtin role %s activation %d is invalid", d.id, d.activation)
	}
	if d.visibility > AppVisible {
		return fmt.Errorf("builtin role %s visibility %d is invalid", d.id, d.visibility)
	}
	if d.ownership > AssemblyOwnedBaseline {
		return fmt.Errorf("builtin role %s ownership %d is invalid", d.id, d.ownership)
	}
	if d.build == nil {
		return fmt.Errorf("builtin role %s baseline builder is nil", d.id)
	}
	if d.policy == RuntimeTransaction && d.expose == nil {
		return fmt.Errorf("builtin role %s runtime exposure is nil", d.id)
	}
	deferred := d.policy == StartupReplace && d.activation == SelectedActivation && d.visibility == KernelOnly
	if (d.policy == StartupReplace || d.policy == Fixed) && d.project == nil && !deferred {
		return fmt.Errorf("builtin role %s startup projection is nil", d.id)
	}
	if d.ownership == AssemblyOwnedBaseline && d.stop == nil {
		return fmt.Errorf("builtin role %s owned baseline stop is nil", d.id)
	}
	return nil
}

// BuiltinRole 是只能与同一 Plan 配合使用的 typed 替换 handle。
type BuiltinRole[TTarget any] struct {
	plan  *Plan
	entry *builtinEntry[TTarget]
}

// BuiltinOutput 保存 AppVisible Role 的 typed root Binding。
type BuiltinOutput[TOutput any] struct{ binding Binding[TOutput] }

// Binding 返回 Assembly 建立的 root Binding。
func (o BuiltinOutput[TOutput]) Binding() Binding[TOutput] { return o.binding }

type builtinRuntime interface {
	roleID() RoleID
	closeBaseline(context.Context) error
	validateRuntime() error
}

type builtinEntry[TTarget any] struct {
	id                 RoleID
	phase              BuiltinPhase
	activation         BuiltinActivation
	visibility         BuiltinVisibility
	policy             ReplacementPolicy
	ownership          BaselineOwnership
	selected           bool
	baseline           TTarget
	target             TTarget
	active             bool
	slot               *lease[TTarget]
	stop               func(context.Context, TTarget) error
	replacementStop    func(context.Context) error
	startupReplacement func() (TTarget, func(context.Context) error, error)
	replacer           bool
	consumerSet        bool
	rootIndex          int
	build              func() (TTarget, error)
}

func (e *builtinEntry[TTarget]) roleID() RoleID { return e.id }
func (e *builtinEntry[TTarget]) markConsumer()  { e.consumerSet = true }

func (e *builtinEntry[TTarget]) validateRuntime() error {
	if e == nil || e.id == "" {
		return fmt.Errorf("builtin role is invalid")
	}
	if e.activation == RequiredActivation && !e.active {
		return fmt.Errorf("builtin role %s required baseline is inactive", e.id)
	}
	if e.policy == RuntimeTransaction && e.active && e.slot == nil {
		return fmt.Errorf("builtin role %s runtime slot is nil", e.id)
	}
	return nil
}

func (e *builtinEntry[TTarget]) closeBaseline(ctx context.Context) error {
	if e == nil || !e.active {
		return nil
	}
	var err error
	if e.slot != nil {
		drained, drainErr := e.slot.beginDrain()
		if drainErr != nil {
			return fmt.Errorf("drain builtin baseline %s: %w", e.id, drainErr)
		}
		select {
		case <-ctx.Done():
			e.slot.resume()
			return fmt.Errorf("wait builtin baseline %s drain: %w", e.id, ctx.Err())
		case <-drained:
			e.slot.takeWhileDraining()
		}
	}
	if e.replacementStop != nil {
		err = errors.Join(err, e.replacementStop(ctx))
		e.replacementStop = nil
	}
	if e.ownership == AssemblyOwnedBaseline && e.stop != nil {
		err = errors.Join(err, e.stop(ctx, e.baseline))
	}
	e.active = false
	if err != nil {
		return fmt.Errorf("close builtin baseline %s: %w", e.id, err)
	}
	return nil
}

// RegisterBuiltin 把 catalog Definition 加入 Plan，并在选中时构造 baseline。
func RegisterBuiltin[TTarget, TOutput any](plan *Plan, definition BuiltinDefinition[TTarget, TOutput], selected bool) (BuiltinRole[TTarget], BuiltinOutput[TOutput], TOutput, error) {
	var zeroOutput TOutput
	if plan == nil {
		return BuiltinRole[TTarget]{}, BuiltinOutput[TOutput]{}, zeroOutput, fmt.Errorf("component plan is nil")
	}
	if plan.state != planOpen {
		return BuiltinRole[TTarget]{}, BuiltinOutput[TOutput]{}, zeroOutput, fmt.Errorf("component plan is frozen")
	}
	if err := definition.validate(); err != nil {
		return BuiltinRole[TTarget]{}, BuiltinOutput[TOutput]{}, zeroOutput, err
	}
	if _, exists := plan.roles[definition.id]; exists {
		return BuiltinRole[TTarget]{}, BuiltinOutput[TOutput]{}, zeroOutput, fmt.Errorf("builtin role %s is duplicated", definition.id)
	}
	active := definition.activation == RequiredActivation
	entry := &builtinEntry[TTarget]{
		id: definition.id, phase: definition.phase, activation: definition.activation,
		visibility: definition.visibility, policy: definition.policy, ownership: definition.ownership,
		selected: selected, stop: definition.stop, build: definition.build,
	}
	var output TOutput
	var root BuiltinOutput[TOutput]
	if active {
		baseline, err := definition.build()
		if err != nil {
			return BuiltinRole[TTarget]{}, BuiltinOutput[TOutput]{}, zeroOutput, fmt.Errorf("build builtin baseline %s: %w", definition.id, err)
		}
		if isNil(baseline) {
			return BuiltinRole[TTarget]{}, BuiltinOutput[TOutput]{}, zeroOutput, fmt.Errorf("builtin baseline %s is nil", definition.id)
		}
		entry.baseline, entry.target, entry.active = baseline, baseline, true
		if definition.policy == RuntimeTransaction {
			entry.slot = newLease[TTarget]()
			entry.slot.publishInitial(baseline)
			projected, err := definition.expose(entry.slot)
			if err != nil {
				return BuiltinRole[TTarget]{}, BuiltinOutput[TOutput]{}, zeroOutput, errors.Join(fmt.Errorf("expose builtin role %s: %w", definition.id, err), entry.closeBaseline(context.Background()))
			}
			output = projected
		} else {
			projected, err := definition.project(baseline)
			if err != nil {
				return BuiltinRole[TTarget]{}, BuiltinOutput[TOutput]{}, zeroOutput, errors.Join(fmt.Errorf("project builtin role %s: %w", definition.id, err), entry.closeBaseline(context.Background()))
			}
			output = projected
		}
		if isNil(output) {
			return BuiltinRole[TTarget]{}, BuiltinOutput[TOutput]{}, zeroOutput, errors.Join(fmt.Errorf("builtin role %s output is nil", definition.id), entry.closeBaseline(context.Background()))
		}
	}

	plan.roles[definition.id] = entry
	plan.roleOrder = append(plan.roleOrder, entry)
	if active && definition.visibility == AppVisible {
		token := &bindingToken{}
		index := len(plan.outputs)
		plan.outputs = append(plan.outputs, output)
		plan.tokens = append(plan.tokens, token)
		plan.outputPhases = append(plan.outputPhases, definition.phase)
		plan.rootRoles[index] = entry
		entry.rootIndex = index
		root.binding = Binding[TOutput]{plan: plan, index: index, token: token}
	}
	return BuiltinRole[TTarget]{plan: plan, entry: entry}, root, output, nil
}

// ActivateSelected 在 Plan 冻结后构造被进程模式明确选中的 PreStart baseline。
func ActivateSelected[TTarget any](role BuiltinRole[TTarget]) (TTarget, error) {
	var zero TTarget
	if role.plan == nil || role.entry == nil {
		return zero, fmt.Errorf("builtin role handle is invalid")
	}
	entry := role.entry
	if role.plan.state != planFrozen {
		return zero, fmt.Errorf("builtin role %s selected activation requires a frozen plan", entry.id)
	}
	if entry.activation != SelectedActivation || !entry.selected {
		return zero, fmt.Errorf("builtin role %s is not selected", entry.id)
	}
	if entry.active {
		return zero, fmt.Errorf("builtin role %s is already active", entry.id)
	}
	target, err := entry.build()
	if err != nil {
		return zero, fmt.Errorf("build builtin baseline %s: %w", entry.id, err)
	}
	if isNil(target) {
		return zero, fmt.Errorf("builtin baseline %s is nil", entry.id)
	}
	entry.baseline, entry.target, entry.active = target, target, true
	if entry.startupReplacement != nil {
		replacement, close, replacementErr := entry.startupReplacement()
		if replacementErr != nil {
			return zero, errors.Join(replacementErr, entry.closeBaseline(context.Background()))
		}
		entry.target, entry.replacementStop = replacement, close
	}
	return entry.target, nil
}

// BuiltinTarget 返回已经激活并冻结的 Startup Role target。
func BuiltinTarget[TTarget any](role BuiltinRole[TTarget]) (TTarget, error) {
	var zero TTarget
	if role.plan == nil || role.entry == nil || role.entry.id == "" {
		return zero, fmt.Errorf("builtin role handle is invalid")
	}
	if !role.entry.active {
		return zero, fmt.Errorf("builtin role %s is not active", role.entry.id)
	}
	return role.entry.target, nil
}

// CloseBuiltins 按 catalog 登记反序释放 Assembly-owned baseline。
func CloseBuiltins(ctx context.Context, plan *Plan) error {
	if ctx == nil {
		return ErrNilContext
	}
	if plan == nil {
		return nil
	}
	var joined error
	for index := len(plan.roleOrder) - 1; index >= 0; index-- {
		joined = errors.Join(joined, plan.roleOrder[index].closeBaseline(ctx))
	}
	return joined
}
