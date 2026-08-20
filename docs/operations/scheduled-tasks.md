# 定时任务运维

定时调度默认关闭。启用前先确认模块已经声明任务、目标环境的 `scheduler` 配置通过校验；严格分布式任务还要求 `cache.driver: redis` 指向受支持的 Redis 端点。

## 观测状态

读取现有 management `/diagnostics`，关注 `schedulerHealth` 和 `scheduler.tasks[]`：

| 状态 | 含义 | 运维动作 |
| --- | --- | --- |
| `local` / `leader` | 已开放本地或领导者准入 | 正常观察 |
| `standby` / `contending` | 未持有执行权或正在争抢 | 多实例中的正常状态 |
| `degraded` | `skip` 已关闭准入，但应用保持 ready | 检查 Redis；接受声明允许的漏跑 |
| `paused` | `pause` 已关闭准入并使 readiness 失败 | 隔离流量并恢复 Redis，不必重启应用 |
| `weakened` | best-effort 任务按显式 `local` 策略运行 | 确认业务允许弱化语义，尽快恢复 Redis |
| `failed` | `fail` 已进入既有 Generation/Supervisor fatal path | 按进程失败策略和日志错误类型处置 |
| `stopping` | Generation 正在撤销准入和排空 | 等待受控关停预算 |

协调依赖恢复后，`skip`、`pause` 和 best-effort `local` 任务会自动重新争抢后续执行权，不要求重启。不要通过临时修改为 `local` 来“修复”严格任务；严格策略会拒绝这种配置组合。

## 故障演练

上线严格任务前，在目标部署环境至少验证：

1. 两个以上实例竞争同一 Task ID 时只有一个实例为 `leader`；
2. Redis 中断后任务进入声明的 `degraded` / `paused` / `failed` 状态，没有隐式本地执行；
3. Redis 恢复后任务自动重新参与，standby/leader 分工恢复；
4. leader 停止或租约失权后旧代不再开放新执行，新 owner 在租约安全窗口后接管；
5. Redis failover、长 GC pause 和网络分区下的业务副作用符合目标系统幂等或 fencing 设计。

Redis 租约只协调执行权，不承诺业务副作用 exactly-once。涉及扣款、结算、库存等不可重复副作用时，业务模块仍须使用稳定幂等键或目标资源支持的 fencing / 条件写协议。

## 安全处置

- 日志和工单只保留 Task ID、Generation、state 与稳定错误类型；不要粘贴 Redis 密码、完整连接信息、token 或任务原始参数。
- 修改 `leaseTTL` / `renewInterval` 前保持至少两个续租安全窗口；校验会拒绝 `renewInterval * 3 > leaseTTL`。
- `tasks.<id>` 必须命中代码声明；拼写错误会让候选 Generation Prepare 失败并保留旧代。
- 配置恢复后优先观察自动恢复，不通过进程重启掩盖协调依赖问题。
