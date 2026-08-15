# R001 当前 Handler 与 HTTP route binding 耦合复核

## 1. 研究问题

本报告回答两个问题：当前项目新增一个业务 HTTP 模块时，是否仍能体现简单、高效、通用；当前 HTTP 链路是否真正遵循“先完成 Handler，再绑定路由，最后交给应用 Router”的职责顺序。

“通用”在本报告中不表示运行时自动发现，而表示新增一个编译期内置模块时，只需要增加该模块自身 operation Handler、更新唯一 OpenAPI authority，并在唯一 composition root 增加显式连接；既有模块不应被迫实现新模块的方法，也不应复制完整路由表。

## 2. 方法与范围

1. 从 HEAD `a42703f0644355c2f6344ca225c682491c8ecc3f` 追踪 `generation -> todo.NewHTTP -> httpbinding.New -> generated strict/route binding -> applicationRouter -> Server`。
2. 检查 `api/openapi.yaml`、生成的完整 `StrictServerInterface` 和 `HandlerWithOptions` 的作用域。
3. 复核 025-R001 的边界结论、适用范围和“第二个 HTTP 模块”刷新触发器。
4. 运行当前定向测试，区分“现有行为正确”与“扩展结构通用”。

本研究不新增第二个业务用例，不修改 OpenAPI、生成物、源码或依赖。

## 3. 当前事实

### 3.1 已经正确的部分

- `api/openapi.yaml` 是路径、method、operationId、DTO、security 与 policy 的唯一事实源；路由没有手写第二份 method/path 表。
- Todo 手写 DTO 映射、错误呈现和用例调用已经收口到 `internal/module/todo/binding/http`，没有回到顶层通用 transport。
- `internal/composition` 仍是唯一跨模块连接位置；Auth 与 Todo 通过窄 Adapter 连接。
- Application Generation 按 Capability、Auth、Todo、Ops、Router、Server 的确定顺序构造；没有 Service Locator、扫描或隐式初始化。
- 2026-08-16 执行 `go test ./internal/module/todo/binding/http ./internal/composition ./pkg/httpx -count=1`，三个目标全部通过。

因此，当前链路不是运行缺陷；对只有 Todo 的现状，它是正确且可验证的。

### 3.2 职责耦合点

`httpbinding.New` 当前一次完成五类工作：

1. 调用 `NewTodoHandler` 构造 Todo operation Handler；
2. 读取整份 embedded OpenAPI；
3. 创建模块私有 Chi Router；
4. 安装 request validator、strict middleware 和错误边界；
5. 调用生成的 `api.HandlerWithOptions` 绑定整份 OpenAPI 路由并返回 `net/http.Handler`。

`TodoHandler` 末尾又直接声明满足应用级 `api.StrictServerInterface`。该接口当前只有四个 Todo operation，所以暂时看不出耦合；当同一 OpenAPI 增加另一个模块的方法时，这个接口会整体扩张，Todo Handler 即使没有业务变化也无法继续满足它。

`applicationRouter` 接收到的参数名是 `businessHandler`，只执行 `Mount("/", businessHandler)`。这一行视觉上简短，但它隐藏的是“模块已经私自绑定整份应用路由”，不是清晰的模块级路由贡献。

### 3.3 新增第二模块时的实际摩擦

如果直接复制 Todo 模式：

- 两个模块都从整份 OpenAPI 生成 binding，各自会尝试注册所有 operation；
- 两个模块都必须满足扩张后的完整 `StrictServerInterface`，形成反向编译耦合；
- 每个模块都加载规范并创建 validator/router，重复构造协议设施；
- composition 无法从类型和名称看出哪一处是唯一 route authority；
- 根挂载多个完整 Handler 会产生重叠路径、顺序依赖或不可达分支。

如果让 Todo Handler 顺手实现其他模块方法，又会把模块边界重新打穿。以上两条都不符合单轨演进。

## 4. 标准判断

| 标准 | 当前单模块 | 新增第二模块 | 结论 |
| --- | --- | --- | --- |
| 简单 | 文件少，当前行为容易跑通 | Handler、validator、Router、route binding 混在同一构造器，新增路径不直观 | 部分符合 |
| 高效 | 当前只有一次构造，运行开销可接受 | 复制模式会重复解析规范、创建 validator/router，维护改动扩散 | 当前符合，扩展后不符合 |
| 通用 | Todo HTTP 可用 | 完整 strict interface 和整份 route binding 都假设只有一个业务模块 | 不符合 |
| 简洁美观 | 外层只有一次 `Mount` | 简短代码掩盖了错误 owner，语义名称不足 | 表面简洁，职责不清 |

结论：新增业务模块的 Model/Service/Repository/Capability 评估路径已经比较清晰，但 HTTP 接入链仍是单模块特例，不能把当前“能工作”描述成通用模块模板。

## 5. 目标方向

把三个概念拆开且只拆到必要程度：

- **operation Handler**：模块拥有，只实现本模块的生成 request/response 方法与 DTO/错误映射，不创建 Router。
- **route binding**：应用协议拥有，接收完整静态 strict API aggregate，一次安装 OpenAPI validator、operation middleware 和生成路由，输出一个 `net/http.Handler`。
- **application Router**：进程/application owner 拥有，只安装全局 middleware，并挂载唯一 route binding。

构造顺序必须在 composition 中可读：

```text
Todo UseCases -> Todo Operations Handler
all module Operations -> strict API aggregate
aggregate -> generated route binding
route binding -> application Router -> Server
```

根路径上的一次 `Mount` 可以保留。它不是问题本身；必须消除的是“每个业务模块都藏一份完整 Router 和整份 OpenAPI binding”。

## 6. 能力、资源与生命周期评估

- 用例与数据语义不变；不新增真实业务模块。
- 继续复用 `pkg/httpx`、I18n、Auth、Todo Service 和现有 OpenAPI generated API。
- 不新增第三方技术或底层 Kernel Capability。
- Handler、aggregate、validator 和 Router 都是 generation-owned 纯内存对象；没有新 listener、Client、goroutine 或 Close owner。
- Application Generation 继续整体重建这些对象，Reload 和 drain 语义不变。
- 当前契约足以表达本次职责拆分，无需扩展 Kernel、Host、module Contribution 或 `pkg/httpx.Router`。

## 7. 适用、不适用与局限

适用于当前单体进程内、单一公开 OpenAPI、编译期显式选择的多个业务 HTTP 模块。

不适用于动态插件在运行时增删路由、独立部署服务、多份外部 OpenAPI authority、WebSocket/hijack 或要求模块拥有独立 listener 的场景。

本轮没有真实第二模块，因此实现验收只能证明：既有 Todo Handler 不再满足整份应用接口、route binding 只存在一处、架构门禁能阻止旧模式复发。未来首个真实第二模块仍需按应用模块开发指南完成业务和能力评估。

## 8. 剩余未知与研究门禁

未来 API 规模显著增长后，是否需要按 tag/spec 拆分生成包，取决于生成物规模、schema 共享和团队 ownership；当前没有证据需要承担该复杂度。该未知不妨碍形成 026 的静态聚合计划。

关键代码事实、当前测试结果、扩展失败模式和 owner 边界均已明确，研究门禁通过。
