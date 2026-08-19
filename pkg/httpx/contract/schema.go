package contract

// schemaKind 标识 Schema 的类别。
type schemaKind int

const (
	kindObject schemaKind = iota
	kindString
	kindInteger
	kindNumber
	kindBoolean
	kindArray
	kindRef
)

// Property 描述 object schema 的一个属性。
type Property struct {
	Name   string
	Schema *Schema
}

// Schema 是项目自有的 OpenAPI schema 描述树。模块在契约声明中以本类型显式描述 schema，
// 生成器把它渲染为 OpenAPI 3.0 schema 对象；不暴露任何第三方类型。运行时绑定也据此派生
// typed 编解码。
type Schema struct {
	Kind schemaKind

	// RefName 是 component 引用名（例如 "Todo"）；仅 kindRef 有效，也用于 component 命名。
	RefName string

	Description string

	EnumValues []any
	FormatName string
	PatternStr string
	MinLen     *uint64
	MaxLen     *uint64
	MinNumeric *float64
	MaxNumeric *float64
	IsNullable bool
	DefaultVal any

	RequiredNames []string
	Properties    []*Property
	AllowExtra    bool

	Items *Schema
}

// Ref 生成对命名 component 的引用。
func Ref(name string) *Schema { return &Schema{Kind: kindRef, RefName: name} }

// Named 把一个 schema 命名为 component（进入 components/schemas/<name>），返回自身。
func (s *Schema) Named(name string) *Schema { s.RefName = name; return s }

// String 生成 string schema。
func String() *Schema { return &Schema{Kind: kindString} }

// Integer 生成 integer schema。
func Integer() *Schema { return &Schema{Kind: kindInteger} }

// Int64 生成 integer schema 并带 int64 format。
func Int64() *Schema { return Integer().Format("int64") }

// Default 设置默认值。
func (s *Schema) Default(v any) *Schema { s.DefaultVal = v; return s }

// Number 生成 number schema。
func Number() *Schema { return &Schema{Kind: kindNumber} }

// Boolean 生成 boolean schema。
func Boolean() *Schema { return &Schema{Kind: kindBoolean} }

// Array 生成数组 schema。
func Array(items *Schema) *Schema { return &Schema{Kind: kindArray, Items: items} }

// Object 生成 object schema。
func Object() *Schema { return &Schema{Kind: kindObject} }

// Describe 设置描述，返回自身。
func (s *Schema) Describe(text string) *Schema { s.Description = text; return s }

// Enum 设置枚举取值。
func (s *Schema) Enum(values ...any) *Schema { s.EnumValues = values; return s }

// Format 设置格式。
func (s *Schema) Format(format string) *Schema { s.FormatName = format; return s }

// Pattern 设置正则。
func (s *Schema) Pattern(pattern string) *Schema { s.PatternStr = pattern; return s }

// MinLength 设置 string 最小长度。
func (s *Schema) MinLength(n uint64) *Schema { s.MinLen = &n; return s }

// MaxLength 设置 string 最大长度。
func (s *Schema) MaxLength(n uint64) *Schema { s.MaxLen = &n; return s }

// Min 设置数值下界。
func (s *Schema) Min(n float64) *Schema { s.MinNumeric = &n; return s }

// Max 设置数值上界。
func (s *Schema) Max(n float64) *Schema { s.MaxNumeric = &n; return s }

// Nullable 标记可空。
func (s *Schema) Nullable() *Schema { s.IsNullable = true; return s }

// Required 把属性名标记为必填。
func (s *Schema) Required(names ...string) *Schema {
	s.RequiredNames = append(s.RequiredNames, names...)
	return s
}

// Prop 添加一个属性；默认 additionalProperties: false。
func (s *Schema) Prop(name string, schema *Schema) *Schema {
	s.Properties = append(s.Properties, &Property{Name: name, Schema: schema})
	return s
}

// AllowAdditional 允许额外属性（默认 false）。
func (s *Schema) AllowAdditional() *Schema { s.AllowExtra = true; return s }
