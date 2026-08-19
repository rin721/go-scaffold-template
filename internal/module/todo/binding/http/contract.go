// Package HTTP contract 声明 Todo 模块拥有的 HTTP 契约（030 代码优先）。
//
// 本文件以项目自有 pkg/httpx/contract 类型声明 Todo 的全部公开 operation、DTO 与策略；
// internal/tools/contract-gen 据此渲染 api/openapi.yaml 与 operation inventory。这是 Todo
// 模块自己的路由契约，不再依赖全局生成包 internal/transport/http/api。
package httpbinding

import (
	"github.com/rin721/go-scaffold-template/pkg/httpx/contract"
)

// CreateTodoRequest 是创建 Todo 的请求 DTO。
var createTodoRequestSchema = contract.Object().
	Describe("创建 Todo 的请求体。").
	Required("title").
	Prop("title", contract.String().Describe("Todo 标题，去除首尾空白后必填。").MinLength(1).MaxLength(200))

// TodoStatus 枚举值。
var todoStatusSchema = contract.String().Describe("Todo 生命周期状态。").Enum("pending", "completed")

// todoSchema 是 Todo 资源的响应 DTO。
var todoSchema = contract.Object().
	Describe("Todo 资源。").
	Required("id", "title", "status", "createdAt", "updatedAt").
	Prop("id", contract.String().Describe("Todo 稳定 ID。")).
	Prop("title", contract.String().Describe("Todo 标题。")).
	Prop("status", contract.Ref("TodoStatus")).
	Prop("createdAt", contract.String().Describe("创建时间。").Format("date-time")).
	Prop("updatedAt", contract.String().Describe("最后更新时间。").Format("date-time")).
	Prop("completedAt", contract.String().Describe("完成时间。").Format("date-time").Nullable())

// todoListSchema 是 Todo 分页列表 DTO。
var todoListSchema = contract.Object().
	Describe("Todo 分页列表。").
	Required("items", "offset", "limit", "total").
	Prop("items", contract.Array(contract.Ref("Todo"))).
	Prop("offset", contract.Integer().Min(0)).
	Prop("limit", contract.Integer().Min(1)).
	Prop("total", contract.Int64().Min(0))

// todoIDParam 是路径参数 id 的 schema。
var todoIDParam = contract.String().Describe("Todo 稳定 ID。").MinLength(1).MaxLength(128)
