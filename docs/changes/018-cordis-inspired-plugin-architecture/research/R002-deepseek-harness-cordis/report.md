# R002：DeepSeek Harness 与 Cordis 时空可组合研究

> **历史档案。** 018 方案已废除；本报告只保留外部研究快照，不再支撑当前项目方向或任何实施授权。

## 1. 范围与证据快照

本报告只使用主源：DeepSeek Harness 固定 Commit `47f943859bef60e4160492346772ded9b24f765a` 的架构、bundle 配置和 vendored Cordis 源码；Cordis 4.0.0-rc.7 对应上游 Commit `56b3d4f725681cf4556c1a8695a709cc3b6eed74`；2026-08-13 论文草稿；Go 标准库 `plugin` 文档。

DeepSeek Harness 与 Cordis 均明确处于快速变化阶段，论文也是 active revision 的 preprint。因此本报告提取不变量，不把当前 API 名称冻结为 go-scaffold2 的目标 API。

## 2. “Everything is a Plugin”实际表示什么

DeepSeek Harness 的产品能力——model adapter、tool registry、session log、agent loop、sandbox、persistence、UI surface——都作为 Cordis plugin 挂到共享 Context。扩展产品不需要修改某个产品级 privileged core，而是并列 mount 新插件。

这不表示系统没有内核。Cordis 自身仍有不可约的 Context、Reflect、Registry、Events、Logger 和 Fiber 生命周期；DeepSeek 还把 Cordis vendored 进仓库并维护本地生命周期与事务补丁。正确理解是：

> 产品能力全部通过统一插件协议贡献；插件协议的解释器、生命周期和诊断仍是小而可信的微内核。

## 3. 五个核心机制

### 3.1 Service 与稳定 key

插件提供稳定的 `ctx.<key>` Service，消费者通过 `inject` 声明依赖。provider 不可用时 consumer 等待；provider 身份变化时 consumer 会卸载并重新执行。

收益是空间可组合，但字符串 key 本身并不证明接口兼容。论文 6.6 明确把 key collision、interface drift 和版本化列为开放问题。Go 方案不能只复制 `map[string]any`。

### 3.2 Fiber 是插件实例，不是包

每次插件应用形成 Fiber，Fiber 拥有配置、依赖快照、状态、子 Context、Effect 和 disposer。组件包可以被多个配置实例化；实例 ID 才是 reconciliation key。

这个区分适合 Go：Go package 只是编译与代码组织单位，插件应是带稳定实例身份和生命周期的运行单元。

### 3.3 Effect 是创建点旁边的可逆副作用

`ctx.effect()` 立即执行 setup，收集其返回的 disposer，并在显式 dispose 或 Fiber unload 时按 LIFO 运行。provider 在任何 inverse 执行前先进入 UNLOADING，dependents 先失活和退出，然后才回收 provider。

论文同时强调 inverse 的正确性仍是插件作者义务，Runtime 不能自动证明；跨系统的写入、消息发送和其他 emission 通常不在可逆边界内，只能延迟提交或执行补偿。

### 3.4 Reactive dependency，而不是人工启动顺序

配置行顺序不承载加载语义。依赖满足决定激活，依赖撤回决定失活；循环依赖可以从声明中检测。Cordis 用 provider Fiber UID 识别依赖身份，而不是只比较值。

可迁移原则是“声明决定拓扑，顺序由编译器产生”，不必照搬运行期 Proxy 访问。

### 3.5 Profile、Bundle 与配置树

DeepSeek Harness 从空树开始叠加 base bundle、surface bundle、profile patch、home patch 和 CLI overlay。每个配置 row 有稳定 ID、插件名、config、disabled、isolate/intercept；`--dump-config` 可以查看最终树。

Bundle 是分发/组合单元，不是 Service；Profile 是命名装配；row 是插件实例。三者分开是“积木组装”可运营的关键。

## 4. 论文给出的重要边界

1. **系统边界**：只有系统独占且能恢复的位置才可视为可逆 Effect。外部 DB 写、网络发送、支付等不能因插件 unload 自动撤销。
2. **Service multiplexing**：独占 provider 切换会扰动 dependents；需要多 provider、滚动更新或负载均衡时，应增加稳定 broker，而不是让所有 consumer 跟着重载。
3. **安全**：依赖声明只能约束经 Context 访问的能力，不能隔离恶意代码；不可信插件必须依赖进程、Wasm 或容器等外部 sandbox。
4. **粒度**：拆除依赖环可能增加 integration component 数量，必须用 bundle 和工具降低认知成本。
5. **版本**：key identity 不等于接口兼容，独立开发插件必须有 namespace、版本和兼容策略。

## 5. DeepSeek 实现本身的风险信号

DeepSeek 将 Cordis vendored 以便审计、打补丁和固定版本。其 vendor 记录包含 reentrant disposal、transactional reconciliation、配置监听、串行写入和 lazy config resolution 等本地修复。这说明理念有价值，但实现一个成熟 Runtime 需要大量边界测试，不能把上游库或论文算法视为开箱即用的生产保证。

DeepSeek Harness README 也声明 developer preview 会有 breaking changes；Cordis README 同样声明 API 不稳定。

## 6. Go 语言适配判断

### 6.1 应采用

- 每个插件实例有稳定 ID、Manifest、声明依赖、typed 配置、生命周期和 Effect owner；
- 显式 Catalog 与声明 Profile 分离；
- 先编译依赖图、验证缺失/重复/环/版本，再启动副作用；
- provider 退出前先让 dependents 退出，停止反向拓扑；
- 可逆注册靠 setup 与 cleanup 同地声明并由 Runtime 收集；
- 通过 plan dump、graph、effect tree 和状态暴露诊断。

### 6.2 应做 Go 化改造

- 用 owner 定义的 `Key[T]`/typed token 与构造参数注入替代字符串 Context 查询；
- Catalog 显式加入编译进二进制的 Definition，不使用扫描或 `init`；
- 将内部异构节点 type-erase 限制在 graph compiler，业务代码不接触 `any` 或 Resolver；
- 用稳定 broker 处理 Route、Command、Health、multi-provider 等集合贡献；
- 默认 Profile 在启动前编译，首版 composition 变化归类为 RestartRequired。

### 6.3 当前不应采用

- TypeScript Proxy/模块声明合并；
- 任意字符串 key 与全局 `ctx.get`；
- 为了解耦而建立万能事件总线；
- package HMR 或运行期替换任意 Go 代码；
- 默认采用 Go 标准库 `plugin`。

Go 官方文档指出 `plugin` 不能关闭，仅支持部分 Unix 平台，race detector 支持差，并要求 host/plugin 的工具链、build flags 和共享依赖源高度一致；官方甚至建议同一构建方生成静态导入或改用 IPC。该机制不满足当前 Windows 开发、单一静态二进制和可卸载 Effect 的目标。

## 7. 推荐的装载层级

| 层级 | 代码来源 | 当前建议 | 适用条件 |
| --- | --- | --- | --- |
| 编译期内置 | 同仓 Go package，经显式 Catalog 编译进二进制 | 第一阶段唯一实现 | 当前所有底层能力与业务模块 |
| 构建期组合 | 生成显式 Catalog/Bundle 后重新编译静态二进制 | 后续可选 | 多发行版、不同产品组合 |
| 进程外/Wasm | 独立协议、权限、版本与 sandbox | 不在 018 实施 | 第三方分发、不可信代码、独立升级 |
| Go `plugin` | `.so` 动态链接 | 拒绝作为默认路径 | 与跨平台、卸载和可重复构建冲突 |

## 8. 局限与刷新条件

- 论文案例主要是 Koishi/TypeScript 单一生态，属于存在性与采用证据，不是与 Go 架构的受控性能对比。
- 没有对 Cordis 全部 88 页形式证明逐式复核；本任务完整阅读了摘要/引言、实现、案例与讨论中与工程设计相关的页面。
- 未运行 DeepSeek Harness，本结论不声称其产品功能或性能已被本地验收。
- Cordis 或 DeepSeek Harness 稳定版发布、vendor commit 变化或进入 live reconciliation 实施前必须刷新。

## 9. 对当前任务的影响与研究门禁

研究门禁通过。证据足以确定：go-scaffold2 应吸收 Cordis 的时空可组合不变量，但第一阶段必须是 compiled-in、typed、无公开 Resolver 的 Go 插件内核；动态代码装载与 untrusted plugin 继续作为独立未来问题。
