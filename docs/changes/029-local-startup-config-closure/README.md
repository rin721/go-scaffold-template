# 029 本地启动与配置闭环

## 状态

- 当前阶段：已实施，验证完成。
- 研究门禁：已通过，证据为 `R001`。
- 计划状态：用户已确认 029。
- 当前授权：允许按本计划修改源码、测试、当前使用文档并在临时目录执行 migration/Service 验证；不覆盖用户 `config.yaml`，不 push、tag、Release 或部署。

## 问题

当前失败不是单纯的使用错误：`config init` 会生成包含 `management` 与 `observability` 的完整配置，但 `db migrate` 使用的配置绑定集合缺少这两个 owner，因此把生成器自己的输出拒绝为未知 section。现有测试分别证明了“配置可生成”和“手写最小配置可迁移”，却没有证明用户真正需要的 `config init -> db migrate up -> Service` 连续路径。

文档同时把根 README、`config.example.yaml`、Kernel README、运维文档和历史 `docs/changes` 暴露成近似并列入口。根 README 内容过长，配置示例顶部又遗漏 migration，用户难以判断命令关系、配置来源和错误归属。

## 计划结论

1. 建立唯一 application-owned 配置绑定集合，由 Bootstrap generator、Service、Migration CLI 和 Todo CLI 共用，消除 section 漂移。
2. 新增从生成配置开始的真实闭环测试：临时配置与 SQLite、loopback 临时端口、migration、Service ready、停止和资源释放。
3. 本地开发只推荐 `config init -> db migrate up -> Service` 一条路径；`config.example.yaml` 降为字段参考，不再成为第二条推荐启动流程。
4. 根 README 只保留五分钟启动和文档地图；本地启动、配置、生产迁移、架构说明和历史证据明确分层。
5. 增加按错误文本定位 owner 的排障表，明确生成配置被报 unknown section 属于产品缺陷，不要求用户反复删除合法 section。

## 阅读顺序

1. [R001 当前启动、配置与文档关系复核](research/R001-current-startup-config-docs/report.md)
2. [需求](requirements.md)
3. [设计](design.md)
4. [任务与确认状态](tasks.md)

本目录只保存任务计划和后续证据；实施后的当前使用方式必须回到根 README 与新的本地开发/配置 authority。
