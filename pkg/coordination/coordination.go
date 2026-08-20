// Package coordination 定义跨进程执行权租约的项目自有契约。
//
// 该包只表达获取、续租与释放语义，不绑定 Redis 或其他第三方实现，也不承诺
// 业务副作用 exactly-once。调用方必须在失权后关闭新准入，并让业务执行尊重取消。
package coordination

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Key 是协调后端中的稳定、低敏执行权标识。
type Key string

// LeaseOptions 声明租约的有效期。TTL 必须覆盖一次续租安全窗口。
type LeaseOptions struct {
	TTL time.Duration
}

// Manager 为一个稳定 Key 争取分布式执行权。
type Manager interface {
	Acquire(context.Context, Key, LeaseOptions) (Lease, error)
}

// Lease 表示当前调用方持有的、可续期且只能由自身释放的执行权。
type Lease interface {
	Renew(context.Context, LeaseOptions) error
	Release(context.Context) error
}

var (
	// ErrNilContext 表示协调调用没有提供可取消的 context。
	ErrNilContext = errors.New("coordination: nil context")
	// ErrInvalidKey 表示执行权 Key 为空或不合法。
	ErrInvalidKey = errors.New("coordination: invalid key")
	// ErrInvalidTTL 表示租约 TTL 不是正值。
	ErrInvalidTTL = errors.New("coordination: invalid ttl")
	// ErrNotAcquired 表示执行权当前由其他实例持有；这不是后端故障。
	ErrNotAcquired = errors.New("coordination: lease not acquired")
	// ErrUnavailable 表示协调后端当前无法完成操作。
	ErrUnavailable = errors.New("coordination: backend unavailable")
	// ErrLeaseLost 表示 token 已不再匹配，当前调用方必须停止新执行准入。
	ErrLeaseLost = errors.New("coordination: lease lost")
)

// ValidateAcquire 校验获取执行权的公共输入。
func ValidateAcquire(ctx context.Context, key Key, options LeaseOptions) error {
	if ctx == nil {
		return ErrNilContext
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if strings.TrimSpace(string(key)) == "" {
		return ErrInvalidKey
	}
	if options.TTL <= 0 {
		return fmt.Errorf("%w: ttl must be positive", ErrInvalidTTL)
	}
	return nil
}

// Unavailable 返回一个明确不可用的 Manager，供配置禁用或依赖缺失时使用。
func Unavailable() Manager { return unavailableManager{} }

type unavailableManager struct{}

func (unavailableManager) Acquire(ctx context.Context, key Key, options LeaseOptions) (Lease, error) {
	if err := ValidateAcquire(ctx, key, options); err != nil {
		return nil, err
	}
	return nil, ErrUnavailable
}
