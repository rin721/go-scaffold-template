# coordination

`pkg/coordination` 定义跨进程执行权的获取、续租、释放和稳定错误契约。当前 production Adapter 位于 Cache Redis resource owner 内，复用同一 go-redis client；本包不暴露 Redis 类型或业务锁 API。

租约保证与故障恢复边界见 [业务模块接入定时调度能力](../../docs/development/scheduled-task-capability.md) 和 [定时任务运维](../../docs/operations/scheduled-tasks.md)。
