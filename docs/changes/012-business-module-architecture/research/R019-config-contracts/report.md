# R019：严格 Config Binding、Schema 与 Snapshot 契约研究

## 1. 研究问题与本地缺口

本地 `DecodeSection` 使用 mapstructure v2，开启 `WeaklyTypedInput`，没有开启 `ErrorUnused`。Default generator 与 runtime typed binder 分离，Snapshot 又允许任意 Source 的 `map[string]any`。这些事实可能导致拼写错误静默失效、空值被转换成零、生成模板与运行值漂移、以及自定义 Source 破坏不可变性。

研究目标是判断这些是否为真实库行为，以及成熟项目如何在边界暴露错误；不是引入 Kubernetes 或建立通用 schema engine。

## 2. mapstructure v2.5.0

主源：[mapstructure.go at v2.5.0](https://github.com/go-viper/mapstructure/blob/v2.5.0/mapstructure.go)

真实做法：

- 文档明确未映射 source key 默认被静默忽略；设置 `DecoderConfig.ErrorUnused` 才把 extra keys 作为 error。
- `ErrorUnset` 可把目标中未被 source 设置的字段报告为 error，但是否 required 仍需项目字段语义决定。
- `WeaklyTypedInput` 允许多种强制转换；源码中空字符串可变成整数零/浮点零/false，布尔和数值也可转字符串或相互转换。
- DecodeHook 可以表达 duration 等项目允许的转换，hook error 会使整个 decode 失败。

适用判断：当前 unknown/weak behavior 是确定事实，不是推测。mapstructure 可以继续使用，但应配置 strict unused 和收敛 weak conversion；required/default 不能机械开启全局 ErrorUnset，而由 owner contract 处理。

## 3. Go `encoding/json`

主源：[Decoder.DisallowUnknownFields](https://pkg.go.dev/encoding/json#Decoder.DisallowUnknownFields)

真实做法：标准库 Decoder 可以在目标为 struct 时把未知 object key 作为错误。这证明严格未知字段不需要庞大 schema runtime；但 JSON decoder 本身不能替项目决定 defaults、deprecated、cross-field validation、YAML duplicate 或 reload class。

适用判断：项目可以选择标准库、严格配置后的 mapstructure 或其他薄 Adapter。架构契约应描述“未知字段失败”和 typed owner，而不是绑定具体 decoder。

## 4. Kubernetes v1.34.1 / apimachinery v0.34.1

主源：

- [kubeadm strict configuration](https://github.com/kubernetes/kubernetes/blob/v1.34.1/cmd/kubeadm/app/util/config/strict/strict.go)
- [apimachinery JSON/YAML serializer](https://github.com/kubernetes/apimachinery/blob/v0.34.1/pkg/runtime/serializer/json/json.go)

真实做法：

- kubeadm 先依据已知 GroupVersionKind 选择 schema，未知 configuration 直接失败，再以 `Strict: true` 解码。
- apimachinery strict YAML 在普通 YAML-to-JSON 转换之外对原始输入执行 `YAMLToJSONStrict`，专门捕获否则会被覆盖丢失的重复字段；之后 strict JSON unmarshal 收集未知/重复等严格错误。
- 这套机制服务 Kubernetes 的版本化 API/多 scheme，复杂度远高于当前项目需要。

适用判断：应借鉴“已注册 schema 才接受、duplicate/unknown 在边界暴露”的原则；不引入 Kubernetes runtime、GVK、scheme 或 serializer。当前项目按 section owner 的 typed contract 足够；只有真实持久格式迁移需求出现时再设计版本转换。

## 5. Caddy v2.11.2（复用 R014）

R014 已确认 Caddy 配置生命周期按 load/parse、provision、validate、use、cleanup 分段：新配置完全准备成功后才替换 current，候选失败清理候选并保留旧配置。

适用判断：这继续支持本地 Kernel candidate transaction 和“候选失败保持旧代”。当前缺口是 Application section 未加入同一 candidate，而不是需要第二个配置 runtime；应在 Kernel 上方增加薄协调者。

## 6. 可选设计

| 方案 | 收益 | 代价与风险 | 判断 |
|---|---|---|---|
| 保持 weak decode/ignore unknown | 兼容宽松输入 | typo、删除字段、空值转换静默，默认漂移不可治理 | 拒绝 |
| 全局巨型 Config + schema registry | 表面集中 | owner 反转、模块耦合、未来字段污染 | 拒绝 |
| 直接采用 Kubernetes API machinery | strict/version 功能完整 | 依赖与复杂度远超需求，建立第二套对象 runtime | 拒绝 |
| section owner registration + strict binder + semantic validator | 小而明确，能复用现有 typed Config | 需要逐 section 迁移零值和负向测试 | 推荐 |
| 所有变化一律 RestartRequired | 语义简单 | 丢弃现有 DB/Cache/Storage candidate swap 能力 | 不推荐 |

## 7. 对当前架构的建议

- **保留**：ordered Source、typed Config、owner defaults、Snapshot/digest/provenance、Kernel candidate transaction、RestartRequired preflight。
- **补齐**：SectionID/path/default/bind/validate/classify/sensitivity 的同 owner registration；default-to-runtime round-trip；Application single candidate coordinator。
- **优化**：unknown/duplicate/type strict failure，收敛 weak conversions，canonical Source value domain，Snapshot complete deep copy，owner-driven redaction。
- **按需设计**：deprecated/version conversion 只在真实外部兼容或迁移出现时增加，必须有删除计划。
- **拒绝**：巨型 Config、无类型 Map 公共 API、隐藏 defaults、第二套 reload/container、从文档模板反推 runtime schema。

## 8. 推荐流水线

```text
ordered Sources
  -> syntax/duplicate/canonical shape
  -> deterministic merge
  -> immutable Snapshot candidate
  -> registered section strict binding
  -> field/cross-field semantic validation
  -> change preflight
  -> candidate resource build/probe
  -> coordinated commit/generation update
  -> previous cleanup or persistent degraded
```

默认配置生成只走到 semantic validation，并在写前完成 encode/decode round-trip；资源 probe 明确不执行。

## 9. 验证方法与证据强度

证据强度：mapstructure 固定 tag 源码直接证明当前 decoder 风险；Go 官方文档证明 strict unknown 能力；Kubernetes 固定 tag 源码证明 duplicate/unknown 可在边界治理；Caddy 证据由 R014 固定 tag 复用。外部证据支持原则，不证明本地 API 形态。

验证要求：逐 section unknown/type/zero/empty/default matrix，YAML/JSON duplicate tests，default round-trip，Source canonical/mutable rejection，Snapshot mutation/redaction/digest tests，single Loader call spy，Application/Kernel same generation，RestartRequired no-side-effect，candidate rollback 与 cleanup degraded gate。

## 10. 未决问题

- 现有每个数值 `0` 是否都等同 missing，需要逐 owner 迁移审计，不能全局推断。
- null 和显式清空是否有真实场景；没有场景时应拒绝而非建立 Optional 泛型体系。
- 字段级 provenance 是否是运维验收必需；当前只推荐 source-level provenance。
- schema version 的首个真实迁移场景尚不存在，因此当前不冻结版本 API。
