# 应用模块

`internal/module` 保存由应用组合根显式选择的纵向模块。这里的 Module 是进程内业务单元，不是 Go module、Kernel Component 或动态插件。每个模块按业务名称收口 Model、Service、Repository、协议 Adapter、binding 与 contribution；底层资源仍由 Kernel 统一创建，模块对象不进入 Kernel Plan。

新增模块必须先按 [应用模块开发指南](../../docs/development/application-module-development.md) 完成真实用例、现有能力、新 Capability、资源 owner、生命周期和当前契约适配性评估，再进入目录与接口设计。

当前已有 Auth、[Ops](ops/README.md)、Migration 与 [Todo](todo/README.md) 模块。Auth 拥有认证/授权/审计，Ops 拥有 management/observability，Migration 编排显式 status/up，Todo 拥有业务实体、对象授权 port 与 SQL migration set；composition 只连接完成品：

```text
model <- service <- repo/binding <- module.go <- internal/composition
                    middleware ───────────────┘
```

- `model` 只表达业务状态与不变量。
- `service` 定义用例以及调用方拥有的窄 port。
- `repo`、operation Handler 和各 binding 负责技术/协议转换，并留在拥有该业务语义的模块内；应用级生成 DTO 可以被 Handler 使用，但模块不创建 Router、不绑定整份应用路由，也不满足完整应用 server interface。
- `middleware` 只实现所属模块拥有的 HTTP 横切策略；不能放入其他模块的业务不变量、Service、Repository 或事务。
- `module.go` 只做纯内存局部装配。
- `internal/composition` 是唯一可以同时知道 Kernel Capability 与应用模块的位置。

HTTP 模块遵循固定的 Handler-first 链路：模块先返回窄 operation Handler，`internal/composition` 静态聚合各模块 operation，`internal/transport/http` 只绑定一次 OpenAPI validator、strict middleware 和生成路由，application Router 最后安装全局 middleware 并挂载唯一 API route tree。新增模块不得复制完整 Router、route binding 或 method/path 表。

目录按真实职责建立，不为对称制造空 Handler、Repository 或 binding。模块专属第三方 SDK、Client、cache、goroutine、migration 或 Adapter 默认仍归模块；只有能力评估证明其跨业务复用、具有稳定项目契约并由进程统一选择和治理时，才进入 `pkg -> internal/kernel/app -> internal/kernel/composition`。

禁止自动扫描、`init` 注册、Service Locator、全局可变 Registry，以及让 Handler 直接访问 Repository。
