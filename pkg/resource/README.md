# resource

`pkg/resource` 提供可实例化资源注册表。调用方通过 `NewRegistry()` 登记连接池、监听器、文件句柄等资源，并按反向注册顺序释放。

共享资源默认不由 Registry 关闭，避免误关调用方不拥有的对象。

## 推荐入口

- `NewRegistry()`：创建独立资源注册表。
- `Add(handle)`：注册带名称、关闭函数和共享标记的资源。
- `AddCloser(name, closer)`：注册实现 `io.Closer` 的资源。
- `Close()`：按反向注册顺序关闭非共享资源，并聚合关闭错误。

## 基础使用示例

```go
package runtime

import (
	"io"

	"github.com/rin721/go-scaffold2/pkg/resource"
)

func Register(file io.Closer, sharedCache io.Closer) (*resource.Registry, error) {
	registry := resource.NewRegistry()
	if err := registry.AddCloser("file", file); err != nil {
		return nil, err
	}
	if err := registry.Add(resource.Handle{
		Name:   "shared-cache",
		Shared: true,
		Close:  sharedCache.Close,
	}); err != nil {
		return nil, err
	}
	return registry, nil
}
```

## 关闭语义

- `Close` 可重复调用；第一次关闭后再次调用会直接返回 `nil`。
- 非共享资源按后进先出顺序关闭，适合“后构造的资源依赖先构造的资源”的场景。
- 共享资源不会由 registry 关闭，调用方必须保证真正所有者负责释放。
- 关闭失败会被 `errors.Join` 聚合，调用方应向上返回，不要只记录后吞掉。

## 边界说明

本包不替代 `supervisor.Supervisor`。`resource.Registry` 只负责释放资源；启动顺序、信号处理和后台任务退出仍由进程监督层或应用入口协调。
