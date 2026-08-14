# 020 研究档案

本目录回答脚手架的分发边界和升级语义，不把候选方案写成已实施能力。

## 检索与复核

- 先检索 `docs/**/research/**/metadata.yaml` 中的 `scaffold product`、`generator`、`module`、`internal`、`version` 和 `upgrade`；019 只识别了缺口，没有专门决定产品形态。
- 内部证据来自 `go.mod`、`cmd/app`、`internal/**`、`pkg/**`、测试、Git tag 和跟踪文件清单，快照为 `af7fdadc`。
- 外部证据只使用 Go 官方文档、Go tool 本地帮助和 GitHub 官方模板仓库文档；`gonew` 仅作为实验性参考，不作为已选依赖。

## 记录

- [R001 当前分发边界](R001-current-distribution-boundary/report.md)：核验当前仓库能否作为外部 library、可复制 template 或 generator 输入。
- [R002 Go 分发和版本语义](R002-go-distribution-versioning/report.md)：建立 module、`internal`、模板实例化和版本兼容的约束。
