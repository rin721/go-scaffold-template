# 研究报告

本目录保存面向架构判断的阶段性研究。研究报告描述特定时间点的仓库事实、外部样本和推导结论，不替代根 [README](../../README.md) 或主题文档中的当前实现说明，也不等同于已经确认的实施方案。

## 报告

- [001 Go 脚手架底层能力装配架构对比](001-go-capability-composition/README.md)：比较当前 Kernel Capability 模型与 Kratos/Wire、go-zero、Uber Fx、go-clean-template 的装配方式，并给出分轨演进建议。
- [002 Kernel 底层组件手动装配与安全重载](002-kernel-app-manual-composition/README.md)：解释当前扩展路径为何复杂，提出 `pkg -> kernel/app -> composition` 手动装配模型、组件级重载策略和带观察期的安全实例切换目标。

后续研究使用递增三位序号和语义名称建立独立目录；每份报告必须说明研究快照、样本范围、事实与推断边界以及来源。
