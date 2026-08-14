# 研究报告

本目录保存面向架构判断的阶段性研究。研究报告描述特定时间点的仓库事实、外部样本和推导结论，不替代根 [README](../../README.md) 或主题文档中的当前实现说明，也不等同于已经确认的实施方案。

## 报告

- [001 Go 脚手架底层能力装配架构对比](001-go-capability-composition/README.md)：比较当前 Kernel Capability 模型与 Kratos/Wire、go-zero、Uber Fx、go-clean-template 的装配方式，并给出分轨演进建议。
- [002 Kernel 底层组件手动装配与安全重载](002-kernel-app-manual-composition/README.md)：解释当前扩展路径为何复杂，提出所有底层能力统一进入 `pkg -> kernel/app -> composition -> Kernel/Host`、按 `Fixed/Configured` 与 `Direct/Leased` 选择治理强度的多态装配模型，以及组件级安全重载目标；不提前设计尚未建设的业务层。
- [012 业务模块架构研究档案](../changes/012-business-module-architecture/research/README.md)：任务级、可检索的结构化研究档案，覆盖当前仓库、手工/生成/运行时装配、模块化单体、分布式组件和插件机制。其设计结论仍处于待确认状态。

后续研究使用递增三位序号和语义名称建立独立目录；每份报告必须说明研究快照、样本范围、事实与推断边界以及来源。
