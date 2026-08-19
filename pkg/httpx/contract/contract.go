// Package contract 提供项目自有的 HTTP 契约声明、生成与绑定能力。
//
// 设计目标（030 计划）：
//
//  1. 模块自有契约：每个业务模块在 internal/module/<name>/binding/http 中以本包类型声明自己
//     拥有的 operation（method/path/operationId/policy/security 与 typed DTO），修改模块契约
//     不需要改动任何全局 OpenAPI 文件。
//  2. 代码优先（code-first）：Go 代码是唯一 authority；internal/tools/contract-gen 从模块声明
//     渲染 api/openapi.yaml 与 operation_inventory.gen.go，不再由 openapi.yaml 生成 Go 代码。
//  3. 通用能力先行：本包作为项目通用 HTTP 契约能力先于任何业务模块落地；模块与 transport 只
//     依赖本包，底层第三方库（kin-openapi、jsonschema、yaml）只存在于本包与生成器内部，不
//     泄漏到业务模块或 composition。
//
// 本包不导入 internal/**；对外暴露的 API 只包含项目自有类型。
package contract

import (
	"fmt"
)

// OperationID 是公开契约中的稳定 operation 标识，用于路由、授权、日志、trace、metrics 与
// inventory 的低基数 identity。
type OperationID string

// String 返回 operationId 字符串形式。
func (id OperationID) String() string { return string(id) }

// Method 是 HTTP 方法。
type Method string

// 支持的 HTTP 方法。当前模板使用 GET/POST/PATCH；扩展新方法时必须同步生成器与绑定逻辑。
const (
	MethodGet    Method = "GET"
	MethodPost   Method = "POST"
	MethodPut    Method = "PUT"
	MethodPatch  Method = "PATCH"
	MethodDelete Method = "DELETE"
)

// Security 标识 operation 的认证要求。
type Security string

// 支持的认证方案。
const (
	// SecurityNone 表示公开 operation，无需认证。
	SecurityNone Security = ""
	// SecurityBearer 表示需要 Bearer JWT，对应 OpenAPI security scheme bearerAuth。
	SecurityBearer Security = "bearerAuth"
)

// PolicyMode 标识 operation 的授权策略模式。
type PolicyMode string

// 支持的策略模式，与 openapi-inventory 历史规则一致。
const (
	// PolicyModePublic 表示公开 operation，不声明 scope/action。
	PolicyModePublic PolicyMode = "public"
	// PolicyModeProtected 表示受保护 operation，必须声明 scope 与 action。
	PolicyModeProtected PolicyMode = "protected"
)

// Policy 描述 operation 的授权策略，随 operation 声明并进入生成 inventory 与 OpenAPI x-policy。
type Policy struct {
	Mode   PolicyMode
	Scope  string
	Action string
}

// ParamLocation 是请求参数的位置。
type ParamLocation string

// 支持的参数位置。
const (
	ParamPath   ParamLocation = "path"
	ParamQuery  ParamLocation = "query"
	ParamHeader ParamLocation = "header"
)

// Param 描述单个请求参数（path/query/header）。
type Param struct {
	Name     string
	Location ParamLocation
	Required bool
	Schema   *Schema
}

// Response 描述 operation 的一个成功响应；错误响应统一走 RFC 9457 Problem，不逐 operation
// 重复声明。
type Response struct {
	Status int
	Schema *Schema
}

// Request 描述 operation 的请求体；无请求体时为 nil。
type Request struct {
	Schema *Schema
}

// Operation 是模块声明的一个 HTTP operation。模块在 binding/http 中构造 Operation 列表并
// 暴露为 contract.Module；生成器据此渲染 OpenAPI 文档，transport 据此绑定路由。
type Operation struct {
	ID        OperationID
	Method    Method
	Path      string
	Tags      []string
	Security  Security
	Policy    Policy
	Params    []Param
	Request   *Request   // 请求体；无 body 时为 nil
	Responses []Response // 成功响应
}

// Module 是模块声明的契约聚合。
type Module struct {
	// Name 是模块语义名（例如 "Todo"），用于生成 OpenAPI tag 的稳定 identity。
	Name string
	// Description 是模块契约的简介，进入 OpenAPI tag description。
	Description string
	// Operations 是该模块拥有的全部 HTTP operation。
	Operations []Operation
	// Schemas 是该模块贡献的 component schema；跨模块共享的 schema 由生成器统一定义。
	Schemas []*Schema
}

// 校验模块契约不变量，供生成器与 binder 复用。
func validateModule(module Module) error {
	if module.Name == "" {
		return fmt.Errorf("contract module name is required")
	}
	seen := make(map[OperationID]struct{}, len(module.Operations))
	for _, operation := range module.Operations {
		if operation.ID == "" {
			return fmt.Errorf("module %s declares operation with empty ID", module.Name)
		}
		if _, exists := seen[operation.ID]; exists {
			return fmt.Errorf("module %s duplicates operation ID %q", module.Name, operation.ID)
		}
		seen[operation.ID] = struct{}{}
		if operation.Method == "" || operation.Path == "" {
			return fmt.Errorf("operation %q must declare method and path", operation.ID)
		}
		switch operation.Policy.Mode {
		case PolicyModePublic:
			if operation.Policy.Scope != "" || operation.Policy.Action != "" {
				return fmt.Errorf("public operation %q cannot declare scope or action", operation.ID)
			}
		case PolicyModeProtected:
			if operation.Policy.Scope == "" || operation.Policy.Action == "" {
				return fmt.Errorf("protected operation %q requires scope and action", operation.ID)
			}
			if operation.Security != SecurityBearer {
				return fmt.Errorf("protected operation %q requires bearer security", operation.ID)
			}
		default:
			return fmt.Errorf("operation %q must declare policy mode public or protected", operation.ID)
		}
		for _, param := range operation.Params {
			if param.Name == "" || param.Schema == nil {
				return fmt.Errorf("operation %q declares incomplete parameter", operation.ID)
			}
		}
	}
	return nil
}
