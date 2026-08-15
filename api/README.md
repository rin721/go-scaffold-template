# HTTP API 契约

`openapi.yaml` 是公开 HTTP operation、路径、请求/响应 schema、security 与兼容性的唯一权威。不要手写第二份路由、DTO、权限清单或 Swagger 注解。

## 修改与生成

1. 修改 `openapi.yaml`，为每个 operation 保留唯一稳定的 `operationId`、显式 `security`、`x-policy` 和默认 Problem response。
2. 在仓库根目录执行：

   ```powershell
   go generate ./internal/transport/http/api
   ```

3. 审阅 `internal/transport/http/api/*.gen.go`。生成 DTO 只允许进入模块自己的 operation Handler，不能进入 module core、service 或 repository；顶层 transport 只承载应用级 validator、strict middleware 和单一 generated route binding，不承载手写业务 Adapter。
4. 执行 contract tests 与 breaking gate：

   ```powershell
   go test ./internal/module/todo/binding/http ./internal/transport/http ./pkg/httpx -count=1
   go tool oasdiff breaking <base-openapi.yaml> api/openapi.yaml
   ```

首次建立契约时没有历史 base，只验证规范、生成物和运行 contract；后续变更必须从目标 Git 基线提取 `api/openapi.yaml` 作为 base。公共破坏不得通过更新一份仓库内复制品绕过，必须先采用明确版本/弃用策略并记录决策。

`oapi-codegen.yaml` 固定 strict Chi 生成选项。各模块先产出只包含自己 operation 的 Handler；`internal/composition` 是唯一满足完整生成接口的静态 aggregate；`internal/transport/http` 再把 aggregate 一次绑定为 API routes。新增模块只扩展自身 Handler、aggregate 转发和 composition，不复制 Router 或 method/path。`internal/tools/openapi-inventory` 在生成前校验 operation identity 与策略完整性，并从同一规范生成低基数 inventory，供 Router、授权、日志、trace、metrics 和测试复用。
