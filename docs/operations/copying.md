# 复制为独立项目

本项目是 copy-owned source scaffold，不是 generator 或可通过 `go get` 升级的运行时框架。副本从创建起拥有全部 `cmd`、`internal`、`pkg`、配置、文档和 CI；上游后续变化只能人工审阅迁移。

## 复制输入

只从已验证的 release tag/commit 导出 tracked files，不复制工作目录、`.git`、本地配置、`.data`、凭据、缓存或构建产物：

```bash
git archive --format=tar --output=go-scaffold-template.tar <release-tag>
mkdir <target-directory>
tar -xf go-scaffold-template.tar -C <target-directory>
```

`.scaffold/identity.yaml` 是迁移清单，不是长期 generator authority。复制者必须逐项确定并审阅：目标 module path、应用名/描述、binary、环境变量前缀、配置文件名、OCI source、CI repository guard 和来源记录。

## 迁移规则

1. 先记录 source repository、tag、完整 commit、复制日期和 archive checksum 到 `docs/scaffold-baseline.md`。
2. 按 manifest 的 owner 范围做受控替换；不要对历史证据或第三方文本执行盲目全局替换。
3. 迁移 `go.mod` 与全部当前 Go import，并同步 architecture/boundary tests 中的 module identity。
4. 同步 `cmd/app`、Dockerfile、GoReleaser、CI、README、`.gitignore` 与配置示例的运行身份。
5. 选择保留 Todo 或完整移除 Todo。新业务模块必须位于 `internal/module/<name>`；`internal/composition` 只提取最小 capability 并装配模块。
6. 初始化新的 Git 仓库且不继承 source remote；提交 identity migration 后，为目标项目配置自己的 remote，再制作需要绑定 source commit 的本地 RC。GoReleaser 不得依赖源仓库 remote，也不得靠硬编码 source owner/name 推断目标身份。
7. 禁止 `replace`、`go.work` 或相对路径连接 source checkout；运行 manifest 中的全部验证，并扫描 source module、应用名、环境前缀和配置名残留。

Todo 默认保留，因为它是 HTTP、CLI、认证授权、Database、migration 和 management/observability 的真实垂直示例。选择移除时必须同时移除 Todo module、transport route/spec、CLI、配置、migration、装配、测试和当前文档；不能只删目录后保留失效入口。
