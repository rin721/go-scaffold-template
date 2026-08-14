# Todo 业务模块

Todo 是本仓库首个真实业务垂直切片。HTTP 与 CLI 共享同一个 Service，数据通过 Kernel 提供的稳定 Database Access 持久化；默认配置使用 SQLite。

## 目录

```text
todo/
├── model/                  # Todo、Status 与不变量
├── service/                # 用例、输入输出、Repository port
├── repo/                   # Record 转换与数据库实现
├── handler/                # HTTP DTO、Handler、错误映射
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

HTTP 路由与 CLI 命令的运行方式见根 [README](../../../README.md)。模块边界、配置、Schema 和错误协议的实施依据保存在 [014 变更记录](../../../docs/changes/014-todo-business-vertical-slice/README.md)。
