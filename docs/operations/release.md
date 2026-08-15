# 发布

## 本地 release candidate

先安装固定版本工具，再构建本地候选：

```powershell
./scripts/Install-Tools.ps1
./scripts/Release-Local.ps1
```

Linux 使用 `scripts/install-tools.sh` 与 `scripts/release-local.sh`。输出位于忽略的 `dist/`，包括 Windows/Linux amd64 archive、`checksums.txt`、每个 archive 的 SPDX JSON SBOM，以及本轮临时密钥生成并立即验证的 checksum signature/bundle。临时私钥在脚本结束前删除；`local-rc.pub` 只证明该本地 artifact set 在本轮后未变化，不证明公开发布者身份。

本地候选不创建 tag、不 push、不创建 GitHub Release、不上传 image 或 attestation。

## 正式发布

正式流程只接受受保护的 `v*` tag，并进入 GitHub `release` environment 审批。workflow 使用固定工具版本，重新跑全部质量门禁，以 GoReleaser 创建 draft release，再使用 GitHub OIDC keyless identity 签名并验证 checksum bundle。审批者还必须核对：

- tag、source commit、`/build` 返回值和 archive build info 一致；
- Windows/Linux、三数据库和 container jobs 全部通过；
- checksum、SPDX SBOM 和 Sigstore bundle 都随 draft 提供；
- OpenAPI breaking 结论、migration/rollback 说明和已知限制已审阅；
- 项目许可证状态已由仓库 owner 明确。当前仓库尚未声明源码许可证，所以 OCI label 使用 `NOASSERTION`，这会阻止对外宣称可再分发许可已经完备。

workflow 只生成 draft，不替审批者发布。发布或撤销 release、tag、registry image 仍是外部副作用，必须另有明确授权。
