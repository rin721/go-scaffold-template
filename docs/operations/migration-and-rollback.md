# 数据库迁移与回滚

## 部署顺序

1. 备份数据库并验证恢复路径。
2. 使用与目标二进制相同版本的 artifact 执行 `db migrate status`。
3. 在受控 one-shot job 中执行 `db migrate up`。
4. 再次执行 `status`，要求 version 精确等于二进制 target、`dirty=false` 且 Todo owner completion 已完成。
5. 启动 Service；Service 只读检查兼容性，不执行 DDL。

```powershell
./go-scaffold-template.exe db migrate status
./go-scaffold-template.exe db migrate up
```

旧 Todo 行需要真实 owner 时，先建立经过审计的映射，再显式执行：

```powershell
./go-scaffold-template.exe db migrate up --legacy-owner-subject <subject>
```

## 失败处理

- lock timeout：确认没有存活 migration owner，再重试同一版本；不要并发执行第二套客户端。
- dirty version：停止发布并保留现场。当前 CLI 不暴露 `force`，不得直接篡改版本表冒充成功。
- completion required：补齐真实 owner 映射；不要使用默认用户或时间推断所有权。
- DSN/权限/网络失败：修复外部条件后重新执行 `status`，错误日志不得包含完整 DSN。

## 回滚与 forward-fix

当前生产接口只提供 `up/status/version`，不提供自动 `down`。应用回滚必须满足旧二进制仍兼容当前 schema；不满足时停止流量切换，优先发布 forward-fix。确需数据库恢复时，只能使用部署前验证过的备份恢复流程，并由数据库 owner 审批。不得把仓库中的 `.down.sql` 当作自动生产回滚授权。
