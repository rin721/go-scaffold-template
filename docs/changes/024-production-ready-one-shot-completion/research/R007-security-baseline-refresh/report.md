# R007：C7 安全基线刷新

## 研究问题

C7 把 `govulncheck v1.3.0` 变成 fail-closed 门禁后，原计划固定的 Go 1.26.5 和当前依赖图是否仍可形成 release candidate？如果不能，最小单轨修复是什么？

## 快照与证据

- 源码基线：`411ff860958b60fe66a3b41ccd750765689a0793` 加 C7 未提交交付资产。
- 扫描器：固定 `govulncheck v1.3.0`，使用当前 Go 工具链重新构建，漏洞库更新时间为 2026-08-14 UTC。
- 结果：扫描器给出真实 symbol trace，不只是 module inventory 命中。

可达结果分为三组：

1. Go 1.26.5 标准库：`net/url`、`html/template`、`crypto/tls`、`net/http`、`encoding/xml`、`encoding/asn1` 等路径共 7 条，修复版本为 Go 1.26.6；
2. `google.golang.org/grpc v1.81.1`：HTTP/2 transport 路径可达，修复版本为 1.82.1；
3. `golang.org/x/image v0.38.0`：TIFF/BMP decode 路径可达，修复版本至少为 0.43.0。

扫描还报告了没有项目调用 trace 的 imported/required 漏洞；它们不冒充可达阻断，但仍随依赖升级和后续漏洞库刷新继续审计。

## 决策影响

原 R003 的 “Go 1.26.5 current stable” 是研究时点事实，现已失效。最小修复保持 Go 1.26、同一 gRPC/x.image 技术栈和全部公共契约，只更新 patch/minor 安全版本：

- `go.mod`、CI、Docker builder 和 release toolchain 统一 Go 1.26.6；
- gRPC 更新到 1.82.1；
- x/image 更新到 0.43.0；
- `go mod tidy` 后重新执行 test/race/vet/build、gosec、gitleaks 和 govulncheck；
- Docker builder 使用 `golang:1.26.6-bookworm` 对应的固定 index digest。

这不是引入新框架、公共 API 或 module 边界；它是既定 fail-closed 安全门禁首次运行后必须执行的单轨补丁。继续保留 1.26.5 会让 `REL-REQ-004` 和 C7 完成声明失真。

## 局限与刷新条件

漏洞数据库会变化；本报告只证明 2026-08-15 的扫描结论。每次 RC 都必须重新运行固定版本扫描器并使用当日漏洞库。容器 OS package 风险还需真实 OCI build 后的 SBOM/image scan 证据，本机没有 Docker，不能由本报告替代。
