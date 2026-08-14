# R001：当前 Todo 路由中间件缺口

## 研究问题

核验 014 是否已经存在 middleware 实现与安装链，区分“全局 middleware 已有”和“模块级示例缺失”，并选择一个不虚构认证或配置需求的最小真实示例。

## 代码事实

### 1. 全局 middleware 已真实启用

`internal/composition/service.go` 的 `applicationRouter` 按 `Recovery -> RequestID -> AccessLog -> SecureHeaders` 调用 `Router.Use`。这些 middleware 已有 panic 恢复、ID 注入、结构化访问日志和安全响应头行为，不是空目录或设计占位。

### 2. Route contract 已预留且真实安装

`internal/business.Route` 包含 `Middlewares []httpx.Middleware`。`ValidateContributions` 在启动前拒绝 nil middleware；`applicationRouter` 把每条 route 的 middlewares 传给 `Router.Handle`；`pkg/httpx.chain` 按声明顺序包装 Handler。

`pkg/httpx/router_test.go` 已证明实际顺序为全局 middleware 外层、route middleware 内层、最后 Handler，并在返回时反向展开。

### 3. Todo 没有模块级 middleware 示例

`internal/business/todo/binding/http/routes.go` 定义四条 route，但每条都只设置 Method、Path 和 Handler，`Middlewares` 全部为 nil。014 文档也把 CORS、认证和限流列为非目标。因此用户看到的目录和代码确实缺少“业务模块如何实现并绑定 middleware”的样例。

### 4. 既有研究可以复用

012 R005 的当前有效结论是：Recovery/Middleware 在统一 Router 层安装，业务模块只贡献已绑定 Route，不创建 Server 或全局 DAL。当前代码已经落实这条边界，不需要重新选择 HTTP 框架。

## 方案比较

### A. 只补文档引用全局 middleware

优点是零行为变化；缺点是仍没有模块目录、route contribution 和短路测试，不能回答用户期望的结构化学习示例。

### B. 增加 `NoStore` 响应头 middleware

实现最小且兼容风险低，但只有放行路径，不能清晰展示输入校验、短路、统一错误和 cause 保留。

### C. 增加 POST JSON Content-Type middleware

能展示构造、标准库解析、合法放行、非法短路、`StatusError`、安全 envelope 和 route 选择性绑定；不需要外部依赖或配置。代价是把未声明 Content-Type 的旧请求从 JSON binder 行为收紧为 415，需要用户明确确认并更新文档。

### D. 增加认证、限流或 CORS

这些能力需要真实安全主体、可信代理、速率模型或跨域来源配置；当前证据不足，若作为教学示例会制造假业务或隐藏默认值。

## 研究结论

选择 C：新增 Todo-owned `RequireJSONContentType`，只绑定 `POST /api/v1/todos`。它属于 HTTP 入站 Adapter，不承载 Todo 标题/状态不变量，不访问 Service/Repository，也不把业务对象塞进 context。

该结论会改变公开 HTTP 失败语义，因此 015 必须进入完整确认门禁。当前只形成计划，不修改源码或运行状态。

## 局限与刷新条件

本报告基于 `2239f4c` 静态代码与既有测试，没有在计划阶段启动服务或新增测试。若 Router middleware API、Todo create 契约、认证方案或全局 Content-Type 策略变化，应重新研究并更新计划。
