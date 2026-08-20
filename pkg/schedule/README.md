# schedule

`pkg/schedule` 是业务模块声明 cron/fixedDelay 任务、并发和分布式执行策略的项目自有契约。它不依赖 gocron、Redis、Kernel 或业务模块，也不提供启动、停止或注册底层调度器的入口。

业务接入、配置和保证边界见 [业务模块接入定时调度能力](../../docs/development/scheduled-task-capability.md)。
