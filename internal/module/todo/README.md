# Todo 应用模块

Todo 是本仓库首个真实应用模块，也是一个纵向业务切片。HTTP 与 CLI 共享同一个 Service，数据通过 Kernel 提供的稳定 Database Access 持久化；默认配置使用 SQLite。

## 目录

```text
todo/
├── model/                  # Todo、Status 与不变量
├── service/                # 用例、输入输出、Repository port
├── repo/                   # Record 转换与数据库实现
├── handler/                # HTTP DTO、Handler、错误映射
├── middleware/             # Todo-owned HTTP route middleware
├── binding/
│   ├── model/              # Schema 与 migration participant
│   ├── config/             # todo 配置节
│   ├── http/               # 路由贡献
│   └── cli/                # Application command
└── module.go               # 局部纯装配
```

## 业务操作

- 创建 Todo，标题去除首尾空白并受 `todo.titleMaxRunes` 限制。
- 按 ID 查询。
- 按状态分页列表，稳定排序并返回总数。
- 将 `pending` Todo 完成为 `completed`；串行重复完成保持幂等，并发修改由 Version 冲突保护。

## Middleware 示例

`middleware.RequireJSONContentType` 由 `binding/http` 只绑定到创建路由。它在 Handler 前校验 `Content-Type`，合法 JSON media type 放行，缺失、格式非法或非 JSON 类型返回稳定 415；它不读取 Body、不调用 Service，也不承载 Todo 业务不变量。全局 middleware 仍由 `internal/composition` 统一安装。

HTTP 路由与 CLI 命令的运行方式见根 [README](../../../README.md)。模块边界、配置、Schema 和错误协议的实施依据保存在 [014 变更记录](../../../docs/changes/014-todo-business-vertical-slice/README.md)，模块级 middleware 的需求、设计和验证见 [015 变更记录](../../../docs/changes/015-todo-route-middleware-example/README.md)。

长期 Service 的 Todo Config、Policy、Repository、Service、Handler 与 Router 都属于不可变 Application Generation。配置提交后新连接只进入新对象图，旧连接完成后旧代资源才释放。初始代允许执行 additive migration；切换 Database DSN 的 reload 只做只读 Schema readiness，目标数据库缺表时拒绝候选，不自动迁移或复制业务数据。one-shot CLI 仍按 invocation 构造并释放自己的 Kernel 与 Todo 模块。
