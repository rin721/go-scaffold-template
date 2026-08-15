# R001 当前本地启动、配置闭环与文档关系复核

## 1. 研究问题

1. 用户执行的命令为什么先后出现缺失配置、unknown section 和 trace endpoint 错误？
2. `config init`、`db migrate`、无参数 Service 之间的真实依赖顺序是什么？
3. 为什么现有测试没有发现生成配置无法直接交给 migration CLI？
4. 哪些文档是当前使用 authority，哪些只是架构说明、运维说明或历史证据？

快照为 `ba50c21`，工作树初始 clean。研究只读取代码、配置、测试、文档和用户提供的运行记录，没有修改本地 `config.yaml`，没有启动 Service 或执行 migration。

## 2. 用户复现事实

运行记录形成了三类独立错误：

| 错误 | 直接原因 | 归属 |
| --- | --- | --- |
| `open config.yaml: The system cannot find the file specified` | migration 命令需要配置文件，首次运行前尚未生成 | 使用前置条件应由快速启动文档说明 |
| `unknown config section "observability"/"management"` | 生成器输出完整配置，migration executor 的 binding 集合却漏掉两个合法 root | 当前代码缺陷，不应要求用户删除 section |
| `observability trace endpoint is invalid` / `requires HTTPS` | 用户启用了 tracing，但 endpoint 为空或非 HTTPS；默认生成值实际是 `enabled: false` | 配置语义与排障说明不足，不是 Logger 故障 |

最后一次 `config init --force` 已把 tracing 恢复为默认关闭，但仍不能消除 migration binding 漂移。反复删除 `management` 或 `observability` 只会在两个合法 section 之间轮流失败，不是正确修复。

## 3. 代码事实

### 3.1 生成器拥有完整配置

`Application.Run` 的 Bootstrap 分支向 `ComposeBootstrap` 传入 Auth、Migration、Todo、Ops 与 Observability；`ConfigurationBindings` 再补齐 Logger、Database、Cache、I18n、Storage 与 HTTP。因此 `config init` 合法生成：

```text
logger database cache i18n storage http
auth migration todo management observability
```

### 3.2 Service 也拥有完整配置

`newServiceRuntime` 使用与 Bootstrap 等价的 application-owned binding，GenerationCoordinator 会严格校验完整候选。Tracing 未启用时 endpoint 可以为空；启用后必须是 HTTPS，或在 `insecure=true` 时使用 HTTP loopback。

### 3.3 Migration 漏掉两个合法 owner

`executeMigration` 只向 `ConfigurationBindings` 传入 Auth、Migration、Todo。基础六段会被补齐，但 Ops/Management 与 Observability 不在 known roots。`ValidateCandidate` 会先调用 owner validator，再拒绝任意未知顶层 section，所以 generator 自己生成的完整文件必然在 migration 路径失败。

Todo CLI 已显式声明“CLI 与 Service 共用正式配置文件”，并包含 Management/Observability；Migration 是当前唯一漂移的 application command。

### 3.4 当前缺少真正的消费者闭环测试

- `TestProcessRunsConfigInitBeforeConfigExists` 只断言生成文本包含 section，不把文件交给 migration 或 Service。
- `TestProcessTodoCLIUsesSQLiteAcrossInvocations` 与 Service smoke 使用手写配置，省略了 Management/Observability，因此绕过缺陷。
- `TestComposeBootstrapGeneratesAllServiceSectionsWithoutKernel` 只比较两种生成方式，没有验证任一运行消费者。

这形成了典型的 producer/consumer contract gap：producer 和各 consumer 分别通过，但它们的组合失败。

## 4. 文档事实

### 4.1 根 README 承担过多职责

根 README 同时包含产品定位、包清单、Bootstrap/Service 架构、配置、启动、HTTP/CLI 示例、reload、management、observability、发布和历史任务链接。首次运行步骤被大量架构背景包围，用户无法快速判断哪些内容是“现在必须做”。

### 4.2 两条近似启动路径没有主次闭合

根 README 推荐先复制 `config.example.yaml`，随后又把 `config init` 写成可选方案。`config.example.yaml` 顶部写的是复制后直接 `go run ./cmd/app`，遗漏 migration；根 README 则要求 status/up 后启动。两个当前文件给出不同顺序。

### 4.3 文档类别虽有声明，导航仍像并列 authority

`docs/README.md` 声明 `docs/changes` 只是历史证据，但当前主题入口没有“本地开发”与“配置”文档。用户只能在根 README、Kernel README、配置示例和运维 migration 文档之间拼接流程；`docs/changes` 又大量出现在根 README 正文中，进一步稀释当前 authority。

## 5. 推断与方案选择

### 5.1 配置必须有一个 application-owned 集合

Bootstrap、Service、Migration 和 Todo CLI 共享同一正式配置文件，就必须从同一函数取得 application-owned bindings。继续在四处复制 slice 会再次发生 section 漂移。

本任务不改变“完整配置严格校验”的既有政策：migration 可以只构造 Database/Migration 资源，但仍识别并校验完整正式配置。若未来要允许 one-shot command 忽略其他 section，需要单独研究 known-roots 与 active-validator 分离，不能在本缺陷中偷偷放宽 strict semantics。

### 5.2 本地首次运行只保留一条推荐路径

推荐路径应是：

```text
config init -> db migrate up -> Service -> readiness check
```

`status` 是检查命令，不是首次启动必须先于 `up` 的步骤。`config.example.yaml` 保留为高级字段参考，但不再与生成器竞争“首选启动入口”。

### 5.3 文档按使用问题分层

```text
README（五分钟启动 + 文档地图）
  -> 本地开发（命令顺序、成功判据、停止）
      -> 配置（来源、优先级、section、环境变量、校验）
      -> 运维 migration（生产顺序、失败和 forward-fix）
  -> 架构主题（为什么这样实现）
  -> docs/changes（历史计划与证据，不用于当前操作）
```

根 README 不再复制完整架构和运维正文，只保留最短成功路径与权威链接。

## 6. 局限与刷新条件

- 本研究没有执行用户本地 migration 或启动 Service；缺陷由当前调用链和错误文本直接对应，实施后必须用临时资源进程测试验证。
- 没有研究 PostgreSQL/MySQL、Redis、S3 或远端 OTLP；本地验收只使用 SQLite、disabled Cache、local Storage 和 disabled tracing。
- 若实施需要放宽完整配置严格校验、改变公共 CLI、增加依赖或改变配置 key，必须更新计划并重新确认。

## 7. 对 029 的影响

029 必须同时关闭代码契约、端到端测试和文档 authority。只改 README 会让真实命令继续失败；只补 binding 会让用户继续在冲突入口之间试错。研究门禁已通过，剩余未知不妨碍形成计划。
