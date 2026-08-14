# 015 Todo 路由中间件示例

## 状态

- 任务性质：非纯文档 HTTP 行为变更。
- 当前状态：**已完成实现与本地验证**。
- 研究与计划日期：2026-08-15。
- 代码基线：`2239f4c`。
- 当前授权：用户已在计划报告后的后续消息中明确要求“执行中间件方案”，授权实施当前 `MW-001/BIND-001/VER-001/DOC-001`。

## 一句话结论

014 已安装进程级 `Recovery -> RequestID -> AccessLog -> SecureHeaders`，也支持 `Route.Middlewares`，但 Todo route contribution 没有真实模块级示例。015 拟新增 `internal/business/todo/middleware`，实现并仅在创建路由绑定 `RequireJSONContentType`，以真实的 `415 todo_unsupported_media_type` 展示 middleware 的构造、短路、错误传播、路由绑定和顺序测试。

## 阅读顺序

1. [研究档案](research/README.md)：当前 middleware 链路和缺口证据。
2. [requirements.md](requirements.md)：行为、范围、兼容影响和验收标准。
3. [design.md](design.md)：目录、API、执行顺序、错误语义和文件影响。
4. [tasks.md](tasks.md)：稳定任务 ID 与确认门禁。

## 确认边界

`POST /api/v1/todos` 现在明确要求 `Content-Type: application/json`；缺失、格式非法或非 JSON 都由 middleware 在 Handler 前返回 415。本次实现只覆盖已确认任务；若后续改成认证、限流、CORS、数据库事务 middleware 或增加配置项，需建立新变更并重新确认。
