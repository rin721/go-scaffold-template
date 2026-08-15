# Todo 应用模块

Todo 是本仓库首个真实应用模块，也是一个纵向业务切片。HTTP 与 CLI 共享同一个 Service，数据通过 Kernel 提供的稳定 Database Access 持久化；默认配置使用 SQLite。

## 目录

```text
todo/
├── model/                  # Todo、Status 与不变量
├── service/                # 用例、输入输出、Repository port
├── repo/                   # Record 转换与数据库实现
├── binding/
│   ├── config/             # todo 配置节
│   ├── cli/                # 显式 actor 的 Application command
│   ├── http/               # Todo operation Handler、DTO 映射、错误呈现与 Actor 端口
│   └── migration/          # 三 driver SQL、checksum、owner completion/readiness
└── module.go               # 局部纯装配
```

HTTP binding 不属于 module core，但属于 Todo 模块。公开契约由根目录 [`api/openapi.yaml`](../../../api/openapi.yaml) 定义，生成的 strict server interface 与 DTO 位于 `internal/transport/http/api`；手写 `Operations`/`Handler`、DTO 映射、错误呈现和 `ActorAccess` 位于本模块 `binding/http`。该 Handler 只实现 Todo operation，不创建 Chi Router、不加载 OpenAPI、不执行 operation policy，也不满足整份应用 strict interface；生成类型不会进入 model、service 或 repo。

## 业务操作

- 创建 Todo，标题去除首尾空白并受 `todo.titleMaxRunes` 限制，同时写入 actor subject。
- 按 ID 查询真实记录后执行 owner 授权；跨 actor 与不存在对象使用相同 Not Found 语义。
- 按 owner 与状态分页列表，稳定排序并返回总数。
- 读取真实记录并授权后将 `pending` Todo 完成为 `completed`；串行重复完成保持幂等，并发修改由 Version 冲突保护。

HTTP 路由与 CLI 命令的运行方式见根 [README](../../../README.md)。模块边界、配置和 Schema 的早期实施依据保存在 [014 变更记录](../../../docs/changes/014-todo-business-vertical-slice/README.md)；[015 变更记录](../../../docs/changes/015-todo-route-middleware-example/README.md) 仅保留已被 strict OpenAPI transport 取代的历史证据。

长期 Service 的 Todo Config、Policy、Repository、Service、对象授权 port 与 HTTP Handler 都属于不可变 Application Generation。Todo HTTP profile 返回完成的 Service、窄 `Operations` 与 contribution；唯一 composition root 用小 Adapter 连接 Auth Principal 与 Todo-owned `ActorAccess`/对象授权端口，把 Todo Operations 接入应用静态 strict aggregate，再由 `internal/transport/http` 一次绑定 validator、operation policy 和生成 routes。最外层 Router 只安装全局 middleware 并挂载该 route tree。所有 Service/CLI 候选只读校验 migration version、dirty 与 legacy owner completion，目标数据库不兼容时 fail closed；只有独立 `db migrate up` command 可以执行 Todo-owned versioned SQL。
