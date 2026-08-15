# 设计：HTTP API 脚手架成熟化 Program

## 1. 设计结论

成熟化不是更换现有 Kernel，也不是继续增加孤立 `pkg`。现有进程、配置、资源、模块和 copy-owned 边界保持不变；新增治理沿一条 operation identity 主线接入：

```text
copy-owned release
       |
       v
API authority -> Operation contract -> Router / OpenAPI / compatibility
                                      |          |             |
                                      v          v             v
                               protocol/edge  security   observability
                                      \          |          /
                                       v         v         v
                                      application modules
                                               |
                                               v
                                management + data evolution
                                               |
                                               v
                              delivery + copied-service proof
```

每个平面有唯一 owner；业务 Service 继续不依赖 chi、GORM、OpenTelemetry SDK、身份 SDK 或部署平台类型。

## 2. 保留的现有边界

- `cmd/app` 继续只做进程模式选择和顶层错误退出。
- `internal/composition` 继续是业务对象、HTTP、CLI 和 Host 的唯一 application composition root。
- `internal/kernel/app` 继续治理底层可替换资源；业务模块不进入 Kernel Plan。
- `internal/module` 继续贡献完成品 Route 与 Participant，但 Route 的未来形态必须由 API authority 决定后单轨替换。
- `pkg/httpx` 继续隔离 chi 与 `net/http` 细节；协议公共类型由项目拥有。
- copy-owned 项目复制后拥有全部源码，不依赖源模板 Runtime，也不接受 generator 覆盖。

## 3. Phase 0：可重复 baseline

先建立不改变产品语义的交付可重复性：

- 明确 `.gitattributes`/行尾政策，使 Windows 与 Linux 的 `go mod tidy -diff` 一致；
- 固化完整 validation manifest 和支持平台；
- 建立正式复制指南、identity 清单和 release provenance 格式；
- 保留 Todo 与删除 Todo 两条验收路径。

该阶段可与 API authority 研究并行，因为它不决定 HTTP 公共协议。任何配置、CI 或脚本修改仍是独立非文档变更。

## 4. Phase 1：API authority 决策与原型

### 4.1 只比较两条路线

1. **spec-first**：OpenAPI 是 authority，生成 transport DTO/server contract/client/contract tests，Router 绑定生成 contract。
2. **typed code-first**：项目自有 typed Operation 是 authority，生成 OpenAPI、route catalog、政策矩阵与 tests。

用 Todo 四个 operation 做隔离原型，比较 nullable/enum/error/security、生成稳定性、可读性、breaking diff、IDE、第三方 client 和 identity 迁移。原型不得进入 production import graph，决策通过 ADR 单轨选择。

### 4.2 目标输出

目标 Operation 需要表达 ID、method/path、version/deprecation、request/response/errors、access policy、reliability class 和 observability identity。这里是语义清单，不是已确认的 Go struct 或公开 API。

被选路线必须替换当前 Route 权威；不得长期保留“Route + 手写 OpenAPI + 独立权限清单”。

## 5. Phase 2：协议与 edge policy

以已选 Operation 为输入建立：

- strict decode/validation 与统一 problem presenter；
- 404/405/panic/middleware/handler 同轨错误；
- body/header/query/time/concurrency budget；
- trusted proxy、CORS/CSRF 与 rate/overload class；
- 分页、条件更新、幂等和 streaming 的显式 opt-in 契约。

Middleware 只执行已编译政策，不在运行时任意查询 registry。错误替换必须一次迁移 Todo、404/405、tests 和文档，不保留旧 `{error,message}` 兼容层，除非独立迁移计划明确期限与消费者。

## 6. Phase 3：管理面与可观测性

### 6.1 Management

由 composition 拥有唯一 health registry，Kernel capability、application participant 和外部 adapter 在 listener 启动前贡献命名 Check。默认建立独立 management listener：

- `/startupz`：初始化、migration 和必需 runner 是否完成；
- `/livez`：进程是否进入不可恢复失败；
- `/readyz`：是否应接收新业务流量；
- `/metrics`、build info、脱敏 diagnostics；
- pprof 仅显式启用且受网络/认证政策保护。

### 6.2 Observability

项目自有窄契约表达 operation、status、duration、bytes、dependency 与 error class；OpenTelemetry Adapter 留在技术边界。日志、trace 和 metrics 共享 operation ID/request ID，禁止原始 URL、用户 ID、错误文本或 payload 成为默认指标标签。

## 7. Phase 4：安全政策

API contract 先支持显式 `public` 与 `protected(policy refs)` 语义。未分类 operation 编译或启动失败；public 必须显式声明，不能靠“没有配置 auth”推断。

真实 actor 出现后，再研究并实现：

```text
Credential Adapter -> Authentication Result -> Principal
Operation policy + resource facts -> Authorization Decision -> Audit Event
```

受保护 operation 没有 credential/policy Adapter 时 fail closed。对象级授权在 Service 已加载真实资源后完成；第三方 claims、Token 与 SDK 类型不进入业务契约。没有真实 actor 时，本阶段只允许研究和协议分类，不制造无调用方认证实现。

## 8. Phase 5：数据演进与交付

- 本地 profile 可显式使用 additive auto-migrate；production 默认由独立 `migrate` command/job 执行 versioned artifact。
- migration 定义 checksum、lock/owner、超时、兼容范围、durable result、backfill 和 expand-contract。
- 构建产出注入 version/commit/build time；容器非 root、只读优先，只声明必要 writable path。
- CI 增加 contract diff、fuzz smoke、`govulncheck`、secret/artifact scan；release 产出 checksum、SBOM、签名和 rollback Runbook。
- deployment 示例只作为 Adapter，不让业务包依赖 Kubernetes 或特定云。

## 9. Phase 6：成熟标签验收

从一个正式 release/tag 建立两个外部隔离副本：

1. 保留 Todo，验证默认协议、管理、数据和部署链；
2. 移除 Todo，装配一个独立最小业务模块，验证模板保证不是 Todo 特例。

两个副本在 Windows/Linux 运行同一 gate，并对正常、协议错误、未授权、依赖失败、migration failure、panic、SIGTERM 和 breaking API 变更做场景验收。只有全部必需 gate 通过，才更新根 README 的产品标签。

## 10. 依赖顺序与并行边界

```text
PORTABILITY-001 -------------------------------> RELEASE-001

API-AUTHORITY-001 -> API-CONTRACT-001 -> PROTOCOL-001 -> SECURITY-001
                                      \-> EDGE-001
                                      \-> OBSERVABILITY-001

MANAGEMENT-001 -----------------------> OBSERVABILITY-001
MIGRATION-001 ------------------------> DELIVERY-001

API/SEC/MGMT/OBS/MIGRATION/DELIVERY/RELEASE -> ACCEPTANCE-001
```

`PORTABILITY-001`、`API-AUTHORITY-001` 和 `MANAGEMENT-001` 的研究可并行；生产实现仍各自遵守确认门禁。`SECURITY-001` 的具体 Adapter 依赖真实 actor，`MIGRATION-001` 的生产策略依赖明确部署模型。

## 11. 关键风险与控制

| 风险 | 后果 | 控制 |
| --- | --- | --- |
| 直接实现 Swagger 注解 | Router、文档、权限多权威 | API authority 原型和 ADR 先行 |
| 一次提交全部 P0 | 公共协议难审查、难回滚 | 每个平面独立变更和确认 |
| 为成熟清单制造空接口 | 看似完整但无真实保证 | 每项必须有调用方、owner、失败和验收 |
| 把 auth 产品写死 | 业务依赖第三方 claims | Principal/Policy 项目边界 + Adapter |
| 管理端点混入公网 | 泄露状态和攻击面 | 独立 listener、最小输出、访问政策 |
| 服务实例自动 production migration | 多副本竞争和不可逆失败 | 独立 versioned job、lock、expand-contract |
| 只在源仓库验证 | 复制后 identity/文档/CI 漂移 | 两个外部副本与双平台 gate |
| 行尾由本机 Git 隐式决定 | Windows tidy 与 CI 结果不一致 | 明确 attributes 和可移植性测试 |

## 12. 未决项

1. API authority 最终选择 spec-first 还是 typed code-first；
2. management listener 的默认地址、认证和部署约束；
3. 第一种真实 actor、credential 和资源授权用例；
4. production migration artifact/tool 与目标部署模型；
5. 支持平台、容器基线、release 渠道和签名方案。

这些未知不阻塞 022 计划完成，但分别阻塞对应源码实施；不得由本 Program 文档直接推断技术选型。
