# clock

`pkg/clock` 提供系统时钟、固定测试时钟和稳定时间格式化。

需要时间的组件应通过 `Clock` 注入，避免在测试和重载流程中直接依赖 `time.Now`。

## 推荐入口

- `System()`：生产代码使用的系统时钟，内部调用标准库 `time.Now` 和 `time.Sleep`。
- `Fixed(now)`：测试或可重复流程使用的固定时钟，`Sleep` 不会阻塞。
- `RFC3339Millis(t)`：输出 UTC 毫秒精度时间文本，适合日志字段、快照摘要和测试断言。

## 基础使用示例

```go
package audit

import (
	"time"

	"github.com/rin721/go-scaffold-template/pkg/clock"
)

type Recorder struct {
	clock clock.Clock
}

func NewRecorder(c clock.Clock) *Recorder {
	if c == nil {
		c = clock.System()
	}
	return &Recorder{clock: c}
}

func (r *Recorder) Timestamp() string {
	return clock.RFC3339Millis(r.clock.Now())
}

func fixedRecorder() *Recorder {
	return NewRecorder(clock.Fixed(time.Date(2026, 8, 10, 1, 2, 3, 456_000_000, time.UTC)))
}
```

## 边界说明

- `Clock` 只抽象“当前时间”和“等待”，不负责定时任务调度、重试策略或生命周期管理。
- `Fixed` 适合测试确定性时间；生产代码不应把固定时钟作为隐藏全局默认值。
- 业务组件优先通过构造函数接收 `clock.Clock`，不要在业务函数内部直接调用 `time.Now()`，这样测试和配置重载流程才能稳定复现时间行为。

当前进程统一装配时，`internal/kernel/app/clock.System()` 通过 `app.Value` 输出同一个普通 `clock.Clock` 接口，composition 将它放入 `Capabilities.Clock`。该组件没有运行期配置、生命周期或 `Access.Use`；替换实现只需在 composition 选择另一个输出相同接口的 Definition。
