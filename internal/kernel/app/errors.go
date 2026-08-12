package app

import (
	"errors"
	"fmt"
	"strings"
)

var (
	// ErrNilContext 标识调用方传入了 nil Context。
	ErrNilContext = errors.New("component context is nil")
	// ErrStopped 标识组件租约已经永久停止。
	ErrStopped = errors.New("component access is stopped")
	// ErrRestartRequired 标识配置变化只能通过进程重启生效。
	ErrRestartRequired = errors.New("component configuration requires restart")
)

// RestartRequiredError 保存本轮要求进程重启的全部组件 ID。
type RestartRequiredError struct {
	Components []ID
}

func (e *RestartRequiredError) Error() string {
	values := make([]string, 0, len(e.Components))
	for _, id := range e.Components {
		values = append(values, string(id))
	}
	return fmt.Sprintf("%s: %s", ErrRestartRequired, strings.Join(values, ", "))
}

func (*RestartRequiredError) Is(target error) bool { return target == ErrRestartRequired }
