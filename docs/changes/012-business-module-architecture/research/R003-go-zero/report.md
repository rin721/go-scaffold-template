# R003：go-zero

## 研究问题

检查 goctl 实际模板如何连接配置、Server、Handler、Logic 和依赖，评估生成式黄金路径是否适合本仓库。

## 源码事实

快照 `91a4cdba...` 的 API main template 依次加载 typed config、创建 REST Server、构造 `ServiceContext`、注册 Handler 并启动 Server。入口短且一致，适合大量风格相似的 API 服务。

Handler template 负责解析请求，创建对应 Logic 并把结果交给 HTTP 响应函数。Logic template 保存 request context、logger 和 `*svc.ServiceContext`，业务实现由开发者补充。生成器由 API 描述驱动命名、路由和模型，能减少协议样板。

## 优点

- `main -> ServiceContext -> RegisterHandlers -> Start` 路径清晰，初学者容易找到入口。
- Handler 和 Logic 的生成结构一致，协议解析与用例代码有基本分离。
- 成熟工具链能同步 API 定义、路由和基础模型。
- 适用于约束统一、吞吐开发量大的服务团队。

## 对当前仓库的代价

`ServiceContext` 往往成为每个 Logic 都可访问的依赖集合。即使字段是显式的，调用者实际需求仍被放大，容易形成万能依赖对象，难以从构造签名审查边界。模板还默认围绕 go-zero Server/config/toolchain 组织，与现有 Kernel/Host/Capability 重叠。

生成 Logic 中留给开发者的待实现内容只是脚手架，不是完成的业务架构；当前 AGENTS 也禁止用 TODO/空实现冒充切片。直接引入 goctl 还会增加生成物、版本和定制模板治理。

## 对 012 的结论

吸收“入口短、Handler 只做协议映射、统一注册前校验、开发路径可文档化”的优点，但不采用全局 `ServiceContext`。每个 Handler/Command 应只接收最小 Service 接口；composition root 和模块局部装配替代生成式依赖上下文。

在至少两个真实模块证明重复样板后，才评估只生成机械 DTO/route 代码的工具；生成器不得生成业务 TODO、隐藏依赖或绕过 Kernel 生命周期。

## 局限

本报告基于官方生成模板快照，未对 go-zero 全部 runtime 能力做性能或产品比较；结论只用于模块装配与开发体验取舍。
