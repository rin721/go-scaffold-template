# 开发设计：应用启动入口

## 1. 入口与模式

`cmd/app/main.go` 是唯一 `package main`。入口创建信号 Context，把标准流和固定配置约定交给 `process`：

- 配置文件：`config.yaml`。
- 环境变量前缀：`APP_`，双下划线表达嵌套字段。
- 应用名：`go-scaffold2`。

`process.run` 只按参数数量选择两种互斥模式：

1. 参数非空：composition 显式构造 CLI，执行参数后退出，不调用 `Host.Run`。因此 `config init` 不依赖已有配置文件。
2. 参数为空：composition 不构造 CLI，创建 Host 并长期运行，直到信号 Context 取消。

## 2. 依赖与生命周期

入口只负责选择现有契约，不复制 Kernel 或 Supervisor 行为：

```text
main
  -> supervisor.SignalContext
  -> config.Loader(FileSource, EnvSource)
  -> kernel.New
  -> composition.Compose
       -> CLI.Run(args)          参数非空
       -> kernel.NewHost().Run   参数为空
```

Database 仍是 composition 当前唯一固定能力。服务模式先完成配置加载、构造、Ping 和发布，再进入等待；任一步失败都沿错误链返回。当前没有业务 Participant，因此启动成功表示 Kernel 与 Database 已就绪，不表示存在 HTTP 监听器。

## 3. 错误与退出码

`process.run` 在每个装配边界使用 `%w` 增加上下文。`execute` 只在最外层输出一次错误，并使用 `cli.GetExitCode` 保留 Usage、Command 和 Cancelled 等稳定退出码；错误输出失败时返回通用失败码。

## 4. 文件改动

- 新增 `cmd/app/main.go` 与同包测试。
- 新增 `.gitignore`，排除根目录本地配置文件。
- 更新根 README，提供生成配置、设置 DSN、启动和退出语义。
- 新增本任务 requirements、design 和 tasks 文档。

## 5. 验证

- 测试 CLI 可在配置缺失时生成文件。
- 测试服务模式保留 `os.ErrNotExist` 错误链。
- 测试 nil Context、CLI usage 退出码和 stderr 写失败。
- 执行格式化、tidy diff、build、unit、race、vet、实际 `config init`、受控服务启动探测和 `git diff --check`。
