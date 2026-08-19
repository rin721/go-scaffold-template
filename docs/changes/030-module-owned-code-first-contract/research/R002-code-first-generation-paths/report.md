# R002 typed code-first 生成路径与工具可行性

## 1. 研究问题

用户要求“契约文件由代码生成，而不是由 openapi.yaml 生成代码”，并且“先实现通用再拿来使用，先封装再使用尽量不暴露底层第三方库”。本报告核对：模块能否用 Go 代码声明自己的 HTTP 契约；现有依赖树/本地模块缓存中的工具能否支撑从 Go struct 生成 JSON Schema 与 OpenAPI 文档；与 024 已评估过的 Huma/ogen 相比，项目自有 typed 声明 + 生成器是否可行。

## 2. 方法与范围

- 只使用已固定版本与本地模块缓存做工具能力核实（web 检索不可用，避免引用无法复核的在线资料）；仓库证据为当前 generate 链与 go.mod。
- 不评估运行期动态注册；只评估“编译期契约声明 -> 生成产物 -> 运行期绑定”的单向数据流。

## 3. 证据（事实）

### 3.1 需求侧：模块自有契约的最小形态

- 028/026 已确立模块 HTTP 结构：`internal/module/<name>/binding/http` 拥有 Handler、DTO 映射与错误呈现，但不创建 Router、不绑定整份路由。
- 用户要求契约由模块单独管理：模块应能声明自己负责的 method/path、operationId、policy、security 与 DTO schema，且声明与 Go 类型同源，不复制一份 YAML。
- 因此“typed code-first”的最小形态是：`module.Contract`（或等价物）以 Go 值/类型描述 operation 集合，生成器据此渲染 `api/openapi.yaml`。

### 3.2 invopop/jsonschema：Go struct -> JSON Schema

本地模块缓存 `github.com/invopop/jsonschema@v0.14.0` 存在且为已解析依赖（`go.sum` 有记录，go.mod 的间接依赖中已出现 `github.com/invopop/jsonschema v0.14.0 // indirect`）。源码事实：

- 提供包级 `Reflect(v any) *Schema` 与 `Reflector.ReflectFromType(t reflect.Type)`，可把 Go struct 转成 JSON Schema（draft-2020-12 风格，字段名/required 由 json tag 与 `omitempty` 推断）。
- `Reflector` 提供可配置项：`RequiredFromJSONSchemaTags`、`AllowAdditionalProperties`、`Mapper`、`Namer`、`KeyNamer`、`CommentMap`、`LookupComment` 等；struct tag 支持 `jsonschema:"title=...,description=...,enum=...,min=...,max=...,nullable"`。
- 示例文件展示 `FavColor` 等多枚举场景与 `oneof_required`/`oneof_type` 组合。
- 该库维护活跃、MIT 许可、已随 oapi-codegen 生态进入本仓库依赖图；其输出是 JSON Schema，与 OpenAPI 3.0 schema 语义兼容（OpenAPI 3.0 的 schema 对象本质上是 JSON Schema 子集）。

### 3.3 kin-openapi：程序化构建 openapi3.T 并序列化 YAML

本地模块缓存 `github.com/getkin/kin-openapi@v0.142.0`（当前已在 `internal/transport/http/routes.go` 直接使用）存在。源码事实：

- `openapi3.T` 是完整 OpenAPI 文档内存模型；`openapi3.NewLoader()` 可加载 YAML，`openapi3.NewSwaggerLoader`（旧名）已迁移为 `Loader`，支持 `ReadFromURIFunc` 自定义读取策略。
- 既有代码已证明：加载 YAML -> `spec.Validate(context.Background())` -> 用于 `openapi3filter` 请求校验与 `nethttpmiddleware.OapiRequestValidatorWithOptions`。
- 序列化侧：`openapi3.T` 可 `json.Marshal` 为 JSON，再经 `gopkg.in/yaml.v3`（已在 go.mod）转 YAML；或使用 kin-openapi 的 `*openapi3.T` 的扩展字段保留 `x-policy` 等 vendor extension（`spec.Extensions` 已支持任意扩展字段）。

### 3.4 go:generate 与 clean diff 门禁

- 当前 go:generate 顺序已在 `internal/transport/http/api/generate.go`：oapi-codegen 先生成代码，再运行 openapi-inventory 从同一 YAML 生成 inventory。
- Verify-Quality.ps1/sh 执行 `go generate ./...` 后检查 `git diff --exit-code -- api internal/transport/http/api`，保证生成物与提交内容一致。
- 若改为 code-first，go:generate 顺序反转为：先经项目自有生成器从模块契约声明生成 `api/openapi.yaml` 与 `operation_inventory.gen.go`；clean diff 门禁仍适用，oasdiff breaking gate（已在 CI quality.yml）对照生成的 YAML 继续有效。

## 4. 方案比较（推断）

| 方案 | 当前事实/成本 | 与用户要求的关系 | 结论 |
| --- | --- | --- | --- |
| 维持 spec-first oapi-codegen | 已实施，行为正确；authority 在 YAML | 方向相反：YAML 生成代码，不是代码生成契约；模块契约不归模块 | 被用户要求否定 |
| Huma/ogen | 024 已评估：第三方框架进入 Handler 或替换 Router 边界，迁移面大 | 让第三方框架/生成器决定模块契约形态，违背“不暴露底层第三方库” | 拒绝 |
| 自研生成器 + invopop/jsonschema + kin-openapi（本项目） | 依赖已在树中；编写一小段声明类型 + 渲染器，不需要重造 schema/parser/breaking diff（oasdiff 沿用） | 模块只依赖项目自有类型；第三方只在 generator 与 transport 内部 | 可行，采用 |
| 手写 openapi.yaml（现状的反向复制） | 在 YAML 中维护第二份权威 | 仍是 YAML 为 authority，且双份事实源 | 拒绝 |

关键结论：本方案不是“自研 schema/parser/generator”的重复造轮子——JSON Schema 生成由成熟库完成，OpenAPI 模型与校验由已在使用的 kin-openapi 完成，breaking diff 由 oasdiff（tools 段已固定）完成。项目新增的只是“模块契约声明类型 + 声明转 openapi3.T 的薄渲染器 + 运行期 binder 适配”，属于项目自有封装层，符合 024 中“不重复造轮子”的红线。

## 5. 适用与不适用场景

- 适用：模块以 Go 声明自有操作集合；生成 openapi.yaml 与 operation inventory；transport 用同一声明绑定路由与校验；保留 oasdiff 兼容门禁。
- 不适用：运行期扫描 `reflect` 动态发现路由（拒绝）；模块手写第二份 YAML；为复用框架直接暴露 Huma/ogen 类型；为图省事把生成器整个外包给不可控脚本。

## 6. 局限与剩余未知

- 未运行生成器原型，未验证 invopop/jsonschema 与 kin-openapi 对当前 openapi.yaml 中 enum/nullable/format/pattern/additionalProperties 的逐项等价输出；该等价性必须由生成器开发任务中的 golden 对比测试确认。
- 未验证 openapi3.T 序列化 YAML 的字段顺序与当前手写文件完全一致；clean diff 门禁可接受顺序差异，但 oasdiff 对照基准要选择稳定 base（建议以生成器首次输出为准，或保留当前 YAML 一次人工审阅迁移）。
- 生成物的 `x-policy` 扩展需要在 openapi3 模型上用 `Extensions` 保留，属实现细节。

## 7. 对 030 的影响

- 生成器可复用 invopop/jsonschema + kin-openapi + yaml.v3，无需新增第三方框架；新增的是项目自有封装与声明类型。
- 迁移顺序：通用契约能力（声明类型+渲染器+binder）先行 -> Todo 单轨迁移 -> 删除 oapi-codegen/nethttp-middleware 依赖与生成链 -> 修生成物 diff 与架构门禁 -> 同步文档。
- 研究门禁通过：工具可行性已成立，剩余差异属于生成器实现与等价性验证任务，不阻塞计划形成。