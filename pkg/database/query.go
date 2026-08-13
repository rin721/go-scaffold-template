package database

// Operator 表示受支持的筛选运算。
type Operator string

const (
	// OpEqual 表示等于。
	OpEqual Operator = "eq"
	// OpNotEqual 表示不等于。
	OpNotEqual Operator = "ne"
	// OpLessThan 表示小于。
	OpLessThan Operator = "lt"
	// OpLessOrEqual 表示小于等于。
	OpLessOrEqual Operator = "lte"
	// OpGreaterThan 表示大于。
	OpGreaterThan Operator = "gt"
	// OpGreaterOrEqual 表示大于等于。
	OpGreaterOrEqual Operator = "gte"
	// OpIn 表示属于集合。
	OpIn Operator = "in"
	// OpNotIn 表示不属于集合。
	OpNotIn Operator = "not_in"
	// OpLike 表示 SQL LIKE 匹配。
	OpLike Operator = "like"
	// OpIsNull 表示为空。
	OpIsNull Operator = "is_null"
	// OpIsNotNull 表示不为空。
	OpIsNotNull Operator = "is_not_null"
)

// Filter 是基于 Schema 字段名的筛选条件。
type Filter struct {
	Field    string
	Operator Operator
	Value    any
}

// Direction 表示排序方向。
type Direction string

const (
	// OrderAscending 表示升序。
	OrderAscending Direction = "asc"
	// OrderDescending 表示降序。
	OrderDescending Direction = "desc"
)

// Order 是基于 Schema 字段名的稳定排序。
type Order struct {
	Field     string
	Direction Direction
}

// Page 定义 offset 分页边界。Limit 必须大于零。
type Page struct {
	Offset int
	Limit  int
}

// Query 定义受 Schema 字段白名单约束的查询。
type Query struct {
	Filters []Filter
	Orders  []Order
	Page    *Page
}

// Changes 定义按 Schema 字段名更新的值。
type Changes map[string]any
