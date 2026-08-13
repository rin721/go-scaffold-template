# 008 Web 框架基线建设

## 状态

- 当前状态：**已确认后暂停**。
- 原方案建立与确认日期：2026-08-13。
- 原代码事实基线：`main@139d437e4407583f6a71afd17808e149a9663d72`。
- 2026-08-14 用户将已经实施的 Database 范围重新定位为独立 [010 数据库单轨 GORM 与稳定访问边界](../010-database-gorm-boundary/README.md)，该部分不再计入 008 的待交付范围。
- Web、Plugin、Runner、Password、Mail 和 User 等其余目标尚未实施；后续恢复前必须按当前代码事实重新形成方案并获得确认，不能直接复用本记录的旧授权。

## 原目标

原方案尝试在 Kernel 显式装配基础上，同时建设 Web Server、静态插件注册、Password、Mail、User 业务纵切和公共边界治理。范围过大，且除 Database 外尚无实现证据，因此当前只作为历史规划保留。

## 阅读顺序

1. [requirements.md](requirements.md)：原范围摘要与当前迁出关系。
2. [design.md](design.md)：原目标设计边界，不代表当前实现。
3. [tasks.md](tasks.md)：暂停状态与任务归属。

当前真实能力必须从根 [README.md](../../../README.md) 和主题文档进入；Database 当前实现以 [pkg/database/README.md](../../../pkg/database/README.md) 为准。
