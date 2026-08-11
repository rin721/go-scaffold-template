# 005 全量配置示例

## 状态

用户已于 2026-08-11 确认当前方案并要求实施。示例配置、使用说明和验证已经完成。当前状态：**已完成**。

当前使用方式以根 [README.md](../../../README.md)、[config.example.yaml](../../../config.example.yaml) 和 [Kernel 说明](../../../internal/kernel/README.md) 为准；本目录只保留需求、设计和实施证据。

## 范围

本任务提供可复制的 Logger、Database 全量 YAML 示例。示例默认选择 `development + gorm + postgres`，其他合法方案使用中文注释保留，真实 DSN 只通过环境变量提供。

本任务不改变配置加载顺序、默认配置生成、Capability 契约或应用运行行为。

## 阅读顺序

1. [requirements.md](requirements.md)：目标、约束和验收标准。
2. [design.md](design.md)：示例结构、安全边界和文档入口。
3. [tasks.md](tasks.md)：任务状态和验证证据。
