# 来源与复核入口

## 本地仓库

研究基线：`1cae643cd9f1734f216505b43d69aad6d8780386`，提交日期 2026-08-11，标题 `feat: 注入 Kernel Logger Capability`。

| 主题 | 复核入口 |
| --- | --- |
| 项目定位与边界 | [根 README](../../../README.md) |
| Kernel 运行语义 | [Kernel README](../../../internal/kernel/README.md) |
| 当前 App 组件约束 | [Kernel App README](../../../internal/kernel/app/README.md) |
| 进程入口 | `cmd/app/main.go` |
| 显式组合清单 | `internal/kernel/composition/composition.go` |
| Definition/Register | `internal/kernel/definition.go` |
| Start/Reload/Stop | `internal/kernel/kernel.go` |
| 租约与排空 | `internal/kernel/handle.go` |
| Host/Supervisor | `internal/kernel/host.go`、`pkg/supervisor/supervisor.go` |
| 边界守护 | `pkg/boundary_test.go`、`internal/kernel/capability/boundary_test.go` |

## 外部研究快照

所有外部元数据和源码均于 2026-08-11 访问。为降低默认分支变化造成的漂移，源码链接尽量固定到当日 HEAD。

| 项目 | 分支与快照 | 维护状态 |
| --- | --- | --- |
| go-kratos/kratos | `main@668db92c2c001e9552594ba5a8aede8456af6d7e` | 活跃，MIT |
| go-kratos/kratos-layout | `main@94dbfcc4264a6be8e7b6c4929923c1e1f738b980` | 活跃，MIT |
| zeromicro/go-zero | `master@91a4cdbaf4e987f1c44ab14fb639756f213328f0` | 活跃，MIT |
| uber-go/fx | `master@d5da5b04ac906bfbad8b400baeee9b970c1be6f3` | 未归档，MIT |
| evrone/go-clean-template | `master@376bcd4fa54ffba55540408f80b68db22bcc16cc` | 活跃，MIT |
| google/wire | `main@9c25c9016f6825302537c4efdd5e897976f9c826` | 已归档，Apache-2.0 |

“活跃”仅表示研究快照附近存在仓库更新；不代表本报告完成了维护团队、发布节奏或安全响应的全面审计。

## Kratos / Wire

- [Kratos 官方仓库与基本 App 组合示例](https://github.com/go-kratos/kratos/tree/668db92c2c001e9552594ba5a8aede8456af6d7e)
- [kratos-layout 项目说明](https://github.com/go-kratos/kratos-layout/tree/94dbfcc4264a6be8e7b6c4929923c1e1f738b980)
- [Wire injector：组合各层 ProviderSet](https://github.com/go-kratos/kratos-layout/blob/94dbfcc4264a6be8e7b6c4929923c1e1f738b980/cmd/server/wire.go)
- [Wire 生成代码：真实构造顺序与 cleanup](https://github.com/go-kratos/kratos-layout/blob/94dbfcc4264a6be8e7b6c4929923c1e1f738b980/cmd/server/wire_gen.go)
- [Data ProviderSet 与 cleanup](https://github.com/go-kratos/kratos-layout/blob/94dbfcc4264a6be8e7b6c4929923c1e1f738b980/internal/data/data.go)
- [Google Wire README：无运行时状态/反射与归档声明](https://github.com/google/wire/tree/9c25c9016f6825302537c4efdd5e897976f9c826)
- [Wire FAQ：与反射 DI 的区别及小应用优先手工装配](https://github.com/google/wire/blob/9c25c9016f6825302537c4efdd5e897976f9c826/docs/faq.md)

## go-zero

- [go-zero README：生成目录与 ServiceContext 职责](https://github.com/zeromicro/go-zero/tree/91a4cdbaf4e987f1c44ab14fb639756f213328f0)
- [API main 模板：Config、Server、ServiceContext、Handler、Stop](https://github.com/zeromicro/go-zero/blob/91a4cdbaf4e987f1c44ab14fb639756f213328f0/tools/goctl/api/gogen/main.tpl)
- [RPC main 模板：ServiceContext、Server 与 Stop](https://github.com/zeromicro/go-zero/blob/91a4cdbaf4e987f1c44ab14fb639756f213328f0/tools/goctl/rpc/generator/main.tpl)

## Uber Fx

- [Fx README：运行时 DI 定位、managed singleton 和稳定性声明](https://github.com/uber-go/fx/tree/d5da5b04ac906bfbad8b400baeee9b970c1be6f3)
- [Fx Application lifecycle：Provide/Decorate/Invoke 与 Start/Wait/Stop](https://github.com/uber-go/fx/blob/d5da5b04ac906bfbad8b400baeee9b970c1be6f3/docs/src/lifecycle.md)
- [Fx Lifecycle 契约源码](https://github.com/uber-go/fx/blob/d5da5b04ac906bfbad8b400baeee9b970c1be6f3/lifecycle.go)

## go-clean-template

- [官方 README：composition root 与构造函数 DI](https://github.com/evrone/go-clean-template/tree/376bcd4fa54ffba55540408f80b68db22bcc16cc)
- [internal/app/app.go：资源、use case、server 的真实手工装配和清理](https://github.com/evrone/go-clean-template/blob/376bcd4fa54ffba55540408f80b68db22bcc16cc/internal/app/app.go)

## 来源限制

- 没有把博客、二次教程或搜索摘要作为关键架构结论依据。
- GitHub Star、更新时间和归档状态会变化，只用于说明本次样本选择和维护风险快照。
- 没有实际运行外部项目，也没有验证其所有可选插件；结论来自官方默认模板、官方文档和可见源码。
- 外部项目的许可证只记录仓库元数据，不构成法律意见。
