// Package httpbinding 提供 Todo 模块 HTTP 的「绑定」职责（031 分责）。
//
// 只负责代码优先契约声明（schemas、ModuleContract）与运行期把模块顶层 typed handler 装箱为
// contract.Handler（RuntimeHandlers）；不承载业务 handler 实现、DTO 映射或错误呈现（这些在本
// 模块顶层 handler 包 internal/module/todo/handler）。internal/tools/contract-gen 据此渲染
// api/openapi.yaml 与 operation inventory。
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
