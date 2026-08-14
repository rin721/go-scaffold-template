# R009：HashiCorp go-plugin 与 Mattermost

## 研究问题

确认一个真实动态插件系统除了“注册模块”还需要什么，判断当前业务模块是否应预留进程外扩展协议。

## go-plugin 事实

HashiCorp go-plugin 把插件作为子进程运行，通过 RPC/gRPC 暴露预先约定接口。Host 配置 handshake、protocol version 和 plugin map，启动 Client 后 `Dispense` 取得接口代理，并负责 `Kill`/进程清理。

该机制包含协议协商、进程启动、日志、checksum/TLS 等选项，用本地可靠网络进行通信。它提供故障/语言边界，但也引入序列化、子进程监管、崩溃恢复和兼容责任。

## Mattermost 事实

Mattermost 是实际插件 host。官方文档说明 server plugin 与 server 交互；公开 `plugin.API` 覆盖命令、事件、数据、集群和大量产品能力。这样的 host API 是长期产品契约，需要版本、权限、安全、升级和第三方开发者文档治理，不只是一个 Go interface。

## 适用条件

- 插件来自独立团队/第三方，不能与 host 同时编译发布；
- 需要崩溃或依赖隔离，且能承担 RPC 延迟与进程监管；
- host API 有明确稳定性、权限、版本和弃用策略；
- 插件安装、签名、升级、回滚、Secret 和审计有产品需求。

## 当前拒绝理由

当前模块由同一仓库、同一进程和同一发布单元拥有，没有第三方分发、独立升级或故障隔离需求。引入 plugin protocol 只会把普通构造依赖变成远程协议，并制造庞大兼容面。

即使只做进程内动态 Registry，也会引入扫描、顺序、缺失依赖和卸载语义，与当前编译期模块目标冲突。

## 对 012 的结论

模块使用编译期选择和手工 composition；typed contribution 不是插件 API。未来出现真实第三方扩展条件时必须单独 ADR 和威胁模型，重新研究 go-plugin/Mattermost，并明确 host API 最小能力、权限、版本与进程生命周期。

## 局限

本报告不评估插件沙箱强度。子进程/RPC 本身不等于安全沙箱；真正插件需求还要单独进行安全设计。
