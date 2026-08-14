# 业务模块

`internal/business` 保存进程内真实业务能力。每个模块按业务名称纵向收口 Model、Service、Repository/协议 Adapter 和显式 binding；底层资源仍由 Kernel 统一创建，业务对象不进入 Kernel Plan。

当前首个模块是 [Todo](todo/README.md)。新增模块时先复制它的依赖方向和验证方式，不复制业务字段：

```text
model <- service <- repo/handler <- binding <- module.go <- internal/composition
                         middleware ─┘
```

- `model` 只表达业务状态与不变量。
- `service` 定义用例以及调用方拥有的窄 port。
- `repo`、`handler` 和各 binding 负责技术/协议转换。
- `middleware` 只实现模块拥有的 HTTP 横切策略；不能放入业务不变量、Service、Repository 或事务。
- `module.go` 只做纯内存局部装配。
- `internal/composition` 是唯一可以同时知道 Kernel capability 与业务模块的位置。

禁止自动扫描、`init` 注册、Service Locator、全局可变 Registry，以及让 Handler 直接访问 Repository。
