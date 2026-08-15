# Todo 应用模块

Todo 是本仓库首个真实应用模块，也是一个纵向业务切片。HTTP 与 CLI 共享同一个 Service，数据通过 Kernel 提供的稳定 Database Access 持久化；默认配置使用 SQLite。

## 目录

```text
todo/
├── model/                  # Todo、Status 与不变量
├── service/                # 用例、输入输出、Repository port
├── repo/                   # Record 转换与数据库实现
├── binding/
│   ├── model/              # Schema 与 migration participant
│   ├── config/             # todo 配置节
│   └── cli/                # Application command
└── module.go               # 局部纯装配
```

HTTP transport 不属于 module core。公开契约由根目录 [`api/openapi.yaml`](../../../api/openapi.yaml) 定义，生成的 strict server interface 与 DTO 位于 `internal/transport/http/api`，手写 Adapter 位于 `internal/transport/http`。Adapter 只依赖本模块的 `service.UseCases`，生成类型不会进入 model、service 或 repo。

## 业务操作

- 创建 Todo，标题去除首尾空白并受 `todo.titleMaxRunes` 限制。
- 按 ID 查询。
- 按状态分页列表，稳定排序并返回总数。
- 将 `pending` Todo 完成为 `completed`；串行重复完成保持幂等，并发修改由 Version 冲突保护。

HTTP 路由与 CLI 命令的运行方式见根 [README](../../../README.md)。模块边界、配置和 Schema 的早期实施依据保存在 [014 变更记录](../../../docs/changes/014-todo-business-vertical-slice/README.md)；[015 变更记录](../../../docs/changes/015-todo-route-middleware-example/README.md) 仅保留已被 strict OpenAPI transport 取代的历史证据。

长期 Service 的 Todo Config、Policy、Repository、Service 与生成 transport Adapter 都属于不可变 Application Generation。配置提交后新连接只进入新对象图，旧连接完成后旧代资源才释放。初始代允许执行 additive migration；切换 Database DSN 的 reload 只做只读 Schema readiness，目标数据库缺表时拒绝候选，不自动迁移或复制业务数据。one-shot CLI 仍按 invocation 构造并释放自己的 Kernel 与 Todo 模块。
