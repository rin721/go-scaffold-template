# R005：认证授权能力的应用模块归属复核

## 1. 触发事实

C4 初次实施把项目安全契约放入顶层 `internal/security`，把 JWT 与审计实现放入顶层 `internal/adapter/security`，又在 `internal/composition/security.go` 直接拼接配置和生命周期。用户指出新增业务模块没有在 `internal/module` 内收口。复核当前代码与权威模块指南后确认，这个质疑成立；这批未提交骨架已经撤回。

问题不在于文件数量，而在于同一个业务能力失去唯一 owner：模型、用例、第三方 Adapter、配置、HTTP middleware、资源启动与跨模块连接分别落在不同顶层包，绕过了 `internal/module/<name>` 的固定开发路径，也让 composition root 开始承载模块内部逻辑。

## 2. 当前架构事实

- `internal/module/README.md` 明确要求纵向模块按业务名称收口 model、service、Adapter、binding 与 `module.go`。
- `docs/development/application-module-development.md` 明确区分模块专属 Adapter/Participant 与进程底层 Capability。拥有外部 SDK、Client、cache 或 goroutine，不会自动把一项业务能力升级成 Kernel Component。
- 当前同步 HTTP profile 已由 `ApplicationGeneration` 完整重建 Todo、Router 与 Server。模块配置应随同一 Snapshot 重建，模块资源可以作为 generation-owned Participant 在候选 Ready 前启动、在旧代 drain 后停止。
- `internal/composition` 的责任是选择模块、注入已有底层 Capability、适配跨模块窄 port 和合并 contribution；不应实现 JWT claim 解析、policy 决策或审计事件格式。
- JWT/JWKS authentication、operation authorization 与 security audit 共同服务同一个安全业务语义。当前没有第二个独立业务域需要把 verifier 作为 `pkg`/Kernel 底层 Capability 消费。

## 3. 修订后的唯一归属

```text
internal/module/auth/
├── model/                 Principal、Scope、Action、Decision
├── service/               Authenticate、Authorize、Audit 用例和调用方 port
├── adapter/
│   ├── jwt/               jwx v3.2.0、JWKS fetch/cache/refresh
│   └── audit/             低敏日志 Adapter
├── middleware/            HTTP bearer / development actor 边界
├── binding/config/        auth 配置、默认值与严格校验
└── module.go              纯内存局部装配与 generation contribution
```

- `module.New` 不执行网络 I/O、不启动 goroutine。JWT Adapter 先构造未启动 owner；其 Participant 在 `Start` 获取首份 JWKS 并进入 Ready，在 `Stop` 有界关闭 cache 与刷新 goroutine。
- Application Generation 每代构造 Auth module，并在构造 Router 前启动该代 Auth Participant；候选认证未 Ready 时不取得 connection admission。
- OpenAPI operation inventory 仍是 public operation policy 唯一 authority。composition 只把生成 inventory 转成 Auth module 构造输入，不复制 action/scope 清单。
- HTTP middleware 属于 Auth module；根 `internal/transport/http` 只负责生成 OpenAPI strict transport 与 Todo DTO 映射。
- CLI actor 由 Auth module 的显式本机 operator 入口构造，不解析 bearer token。

## 4. Todo 跨模块边界

Todo 不导入 jwx、JWK、OpenAPI、Auth Adapter 或 composition。Todo `service` 定义自己需要的窄 actor、对象授权与审计 port；HTTP/CLI 输入在边界显式携带 actor，不通过全局变量或可变 service locator 隐藏身份。唯一 composition root 使用小 Adapter 把 Auth module 的完成品连接到 Todo port。

Todo `owner_subject`、真实 resource facts 和跨 actor 隐藏存在性语义仍属于 Todo 模块；认证凭据、scope policy 与安全审计格式属于 Auth 模块。这样不会把 Todo 业务不变量塞进安全 middleware，也不会让 Auth 模块访问 Todo Repository。

## 5. 不选择的路径

- 不建立顶层 `internal/security` 与 `internal/adapter/security`：它们拆散同一 module owner。
- 不把 Auth 机械加入 `internal/kernel/app`：当前 Auth 随完整 Application Generation 换代，且没有被多个独立模块当作底层资源借用的证据。
- 不把 jwx 类型放入 module service/model 或 Todo：第三方类型只留在 `internal/module/auth/adapter/jwt`。
- 不让 composition 解析 claims、维护 JWKS cache 或实现授权规则：composition 只 wiring。

## 6. 门禁结论

研究证据足以形成修订计划。C4 必须以 `internal/module/auth` 为唯一业务 owner，并先更新 024 的需求、设计和任务状态。由于该修订改变模块边界与生命周期归属，按仓库门禁需要用户在修订报告之后再次确认；确认前不恢复 C4 非文档实施。R004 的 `jwx v3.2.0` 版本结论继续有效。
