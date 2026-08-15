# 构建与容器

## 本机构建

Windows 执行：

```powershell
./scripts/Verify-Quality.ps1
```

Linux 执行：

```bash
bash scripts/verify-quality.sh
```

两个入口都会验证格式、`go.mod`、生成物、test、race、vet、CGO-free build 和禁止跟踪的产物。安全扫描由 CI 的独立门禁执行，避免把“本机没有扫描器”静默视为通过。

## OCI image

`Dockerfile` 的 builder 与 distroless runtime 都按 digest 固定。运行层使用 `nonroot`、没有 shell；镜像不内置凭据，也不把 migration 隐藏在 Service startup 中。

```bash
docker build \
  --build-arg VERSION=v1.0.0-rc.1 \
  --build-arg COMMIT="$(git rev-parse HEAD)" \
  --build-arg BUILD_TIME="$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
  --build-arg DIRTY=false \
  -t go-scaffold-template:v1.0.0-rc.1 .
bash scripts/container-smoke.sh go-scaffold-template:v1.0.0-rc.1
```

生产编排必须显式提供只读配置、可写数据卷、business/management 网络政策以及外部 probe。distroless 镜像没有 shell，所以不要添加依赖容器内 `curl` 的 `HEALTHCHECK`；使用 `/startupz`、`/livez` 和 `/readyz` 的外部探针。

SQLite 需要把 `/app/.data` 挂为 nonroot 可写卷。根文件系统应保持 read-only，并为确有需要的临时文件提供受限 tmpfs。SIGTERM 的成功标准是停止 admission、完成有界 drain 并以 0 退出。
