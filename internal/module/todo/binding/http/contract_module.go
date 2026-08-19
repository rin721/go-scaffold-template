package httpbinding

import (
	"github.com/rin721/go-scaffold-template/pkg/httpx/contract"
)

// ModuleContract 返回 Todo 模块的完整 HTTP 契约（030 代码优先）。它只描述 route、policy、security
// 与 DTO schema；不创建 Router、不加载 OpenAPI、不调用生成 binding。contract-gen 从返回值渲染
// api/openapi.yaml 与 operation inventory。
func ModuleContract() contract.Module {
	return contract.Module{
		Name:        "Todo",
		Description: "Todo 示例资源。",
		Schemas: []*contract.Schema{
			createTodoRequestSchema.Named("CreateTodoRequest"),
			todoSchema.Named("Todo"),
			todoStatusSchema.Named("TodoStatus"),
			todoListSchema.Named("TodoList"),
		},
		Operations: []contract.Operation{
			{
				ID:       "createTodo",
				Method:   contract.MethodPost,
				Path:     "/api/v1/todos",
				Tags:     []string{"Todo"},
				Security: contract.SecurityBearer,
				Policy:   contract.Policy{Mode: contract.PolicyModeProtected, Scope: "todos:write", Action: "todo.create"},
				Request:  &contract.Request{Schema: contract.Ref("CreateTodoRequest")},
				Responses: []contract.Response{
					{Status: 201, Schema: contract.Ref("Todo")},
				},
			},
			{
				ID:       "listTodos",
				Method:   contract.MethodGet,
				Path:     "/api/v1/todos",
				Tags:     []string{"Todo"},
				Security: contract.SecurityBearer,
				Policy:   contract.Policy{Mode: contract.PolicyModeProtected, Scope: "todos:read", Action: "todo.list"},
				Params: []contract.Param{
					{Name: "status", Location: contract.ParamQuery, Required: false, Schema: contract.Ref("TodoStatus")},
					{Name: "offset", Location: contract.ParamQuery, Required: false, Schema: contract.Integer().Min(0).Default(0)},
					{Name: "limit", Location: contract.ParamQuery, Required: false, Schema: contract.Integer().Min(1)},
				},
				Responses: []contract.Response{
					{Status: 200, Schema: contract.Ref("TodoList")},
				},
			},
			{
				ID:       "getTodo",
				Method:   contract.MethodGet,
				Path:     "/api/v1/todos/{id}",
				Tags:     []string{"Todo"},
				Security: contract.SecurityBearer,
				Policy:   contract.Policy{Mode: contract.PolicyModeProtected, Scope: "todos:read", Action: "todo.read"},
				Params: []contract.Param{
					{Name: "id", Location: contract.ParamPath, Required: true, Schema: todoIDParam},
				},
				Responses: []contract.Response{
					{Status: 200, Schema: contract.Ref("Todo")},
				},
			},
			{
				ID:       "completeTodo",
				Method:   contract.MethodPatch,
				Path:     "/api/v1/todos/{id}/complete",
				Tags:     []string{"Todo"},
				Security: contract.SecurityBearer,
				Policy:   contract.Policy{Mode: contract.PolicyModeProtected, Scope: "todos:write", Action: "todo.complete"},
				Params: []contract.Param{
					{Name: "id", Location: contract.ParamPath, Required: true, Schema: todoIDParam},
				},
				Responses: []contract.Response{
					{Status: 200, Schema: contract.Ref("Todo")},
				},
			},
		},
	}
}
