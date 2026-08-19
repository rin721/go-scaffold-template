package composition

import (
	todohttp "github.com/rin721/go-scaffold-template/internal/module/todo/binding/http"
	"github.com/rin721/go-scaffold-template/pkg/httpx/contract"
)

// applicationHTTPModules 返回当前应用接入的全部 HTTP 契约模块（装配汇总点）。
//
// 这是 composition 内“哪些模块提供 HTTP 公开契约”的唯一汇总：policy 汇总
// （operationPolicies）、observability operation inventory（opsOperations）与路由
// dispatcher（newContractDispatcher）都从该汇总消费，避免各处硬编码具体模块契约
// （例如 service.go/ops.go 各自直接读 todohttp.ModuleContract）。新增 HTTP 业务模块时
// 在此追加一项，并同时扩展其运行时在 composition 的装配。
func applicationHTTPModules() []contract.Module {
	return []contract.Module{
		todohttp.ModuleContract(),
	}
}
