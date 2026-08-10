# testkit

`pkg/testkit` 收纳跨包测试夹具：固定时钟、临时文件和健康检查。

它只面向测试使用，不进入生产装配链路。

## 推荐入口

- `Clock(t, now)`：返回固定测试时钟。
- `TempConfigFile(t, content)`：写入临时 YAML 配置并返回文件路径。
- `HealthyRegistry(t, name)`：创建包含一个通过项的健康检查注册表。

## 基础使用示例

```go
package service_test

import (
	"testing"
	"time"

	"github.com/rin721/go-scaffold2/pkg/testkit"
)

func TestClock(t *testing.T) {
	now := time.Date(2026, 8, 10, 0, 0, 0, 0, time.UTC)
	clock := testkit.Clock(t, now)
	if !clock.Now().Equal(now) {
		t.Fatal("clock returned unexpected time")
	}
}
```

## 边界说明

- 本包函数都会调用 `t.Helper()`，测试失败位置会指向调用方。
- `TempConfigFile` 使用 `t.TempDir()`，文件生命周期由测试框架管理。
- `TempConfigFile` 和 `HealthyRegistry` 遇到错误会 `t.Fatalf`，只适合测试代码，不应被生产代码导入。
