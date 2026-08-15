# 安全响应

## 报告与分级

正式公开 release 前，repository owner 必须配置私密漏洞报告渠道和响应责任人；当前仓库文档不虚构邮箱或 SLA。不要在公开 Issue 中提交凭据、利用细节或生产数据。

收到报告后先固定受影响 tag/commit、可达调用路径和泄露范围，再区分：代码漏洞、依赖漏洞、凭据事件、配置误用和基础设施事件。日志、Problem Details、diagnostics、trace 和 SBOM 都只能作为证据，不能包含 Token、密钥或完整 DSN。

## 修复与传播

1. 在隔离环境复现并建立负向测试。
2. 修复项目自有 contract/Adapter 边界；不要让业务调用方直接接管第三方客户端。
3. 运行 quality、security、DB、container 和 release gates。
4. 发布新版本和安全说明，列出受影响 baseline、修复 commit、迁移步骤、临时缓解和验证命令。
5. copy-owned 消费者人工评估并迁移修复；上游不会自动覆盖副本。

疑似凭据泄露时应先轮换/撤销，再调查使用记录；删除 Git 文件不能撤销已经暴露的 secret。发现 artifact、checksum、SBOM 或签名不一致时停止发布并重新从固定 source commit 构建，不允许覆盖证据继续发布。
