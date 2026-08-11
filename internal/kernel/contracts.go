package kernel

import (
	"context"
	"fmt"
)

// Builder 根据经过校验的配置创建一个全新实例。
//
// Builder 不得修改正在服务的旧实例，并且必须响应 ctx 取消。
type Builder[C, T any] interface {
	Build(context.Context, C) (T, error)
}

// BuilderFunc 把普通函数适配为 Builder。
type BuilderFunc[C, T any] func(context.Context, C) (T, error)

// Build 调用底层构造函数。
func (f BuilderFunc[C, T]) Build(ctx context.Context, cfg C) (T, error) {
	if f == nil {
		var zero T
		return zero, fmt.Errorf("kernel builder function is nil")
	}
	return f(ctx, cfg)
}

// InstanceHooks 定义候选实例的启动和停止钩子。
//
// Start 必须在实例可被发布前完成就绪确认；Stop 必须释放该实例拥有的资源。
type InstanceHooks[T any] interface {
	Start(context.Context, T) error
	Stop(context.Context, T) error
}

// ActivationHooks 定义实例在不可失败提交区的发布与撤回动作。
//
// 实现不得执行 I/O、阻塞或失败；所有可能失败的准备工作必须在 Builder 或
// InstanceHooks.Start 中完成。
type ActivationHooks[T any] interface {
	Activate(T)
	Deactivate(T)
}

// InstanceHookFuncs 用函数实现 InstanceHooks，便于测试和简单能力保持显式。
type InstanceHookFuncs[T any] struct {
	OnStart func(context.Context, T) error
	OnStop  func(context.Context, T) error
}

// Start 执行启动钩子。
func (h InstanceHookFuncs[T]) Start(ctx context.Context, instance T) error {
	if h.OnStart == nil {
		return fmt.Errorf("kernel start hook is nil")
	}
	return h.OnStart(ctx, instance)
}

// Stop 执行停止钩子。
func (h InstanceHookFuncs[T]) Stop(ctx context.Context, instance T) error {
	if h.OnStop == nil {
		return fmt.Errorf("kernel stop hook is nil")
	}
	return h.OnStop(ctx, instance)
}

// Access 是业务构造函数接收的稳定能力入口。
//
// use 回调返回前不得让 instance 或其派生资源逃逸；回调边界同时也是 Kernel
// 判断旧实例是否仍被使用的租约边界。
type Access[T any] interface {
	Use(ctx context.Context, use func(T) error) error
}
