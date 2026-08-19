package contract

import (
	"fmt"

	"github.com/getkin/kin-openapi/openapi3"
	"gopkg.in/yaml.v3"
)

// Info 描述 API 文档元信息，由生成器传入。
type Info struct {
	Title       string
	Version     string
	Description string
}

// Document 持有已构建的 OpenAPI 文档；对外只暴露 MarshalYAML（返回 []byte），不泄漏第三方类型。
// 运行期校验由 transport 通过加载生成的 YAML 完成。
type Document struct {
	spec *openapi3.T
}

// problemSchema 返回 RFC 9457 Problem 的稳定 component schema（应用级共享）。
func problemSchema() *Schema {
	return Object().
		Describe("RFC 9457 Problem Details。").
		Required("type", "title", "status", "code").
		Prop("type", String().Describe("Problem type URI-reference。").Format("uri-reference")).
		Prop("title", String()).
		Prop("status", Integer().Describe("HTTP 状态码。").Min(400).Max(599)).
		Prop("detail", String().Describe("人类可读的错误细节。")).
		Prop("instance", String().Describe("错误实例 URI-reference。").Format("uri-reference")).
		Prop("code", String().Describe("稳定机器可读错误码。").Pattern("^[a-z][a-z0-9_]{0,63}$")).
		Prop("violations", Array(Ref("Violation")))
}

func violationSchema() *Schema {
	return Object().
		Describe("单个字段校验失败。").
		Required("field", "reason").
		Prop("field", String()).
		Prop("reason", String())
}

// BuildDocument 把模块契约渲染为 OpenAPI 3.0.3 文档。modules 必须已通过 Validate。
func BuildDocument(info Info, modules []Module) (*Document, error) {
	for _, module := range modules {
		if err := validateModule(module); err != nil {
			return nil, err
		}
	}
	spec := newSpec(info)
	for _, module := range modules {
		spec.Tags = append(spec.Tags, &openapi3.Tag{Name: module.Name, Description: module.Description})
		for _, schema := range module.Schemas {
			if schema == nil || schema.Kind == kindRef || schema.RefName == "" {
				return nil, fmt.Errorf("module %s declares a component schema without a stable name", module.Name)
			}
			spec.Components.Schemas[schema.RefName] = renderSchema(schema)
		}
		for _, operation := range module.Operations {
			if err := addOperation(spec, module.Name, operation); err != nil {
				return nil, err
			}
		}
	}
	hoistSharedPathParameters(spec)
	return &Document{spec: spec}, nil
}

// hoistSharedPathParameters 把同一 path 上所有 operation 都声明的相同 path 参数提升到 path item
// 级，与历史规范的共享参数结构保持一致（例如 TodoID）。
func hoistSharedPathParameters(spec *openapi3.T) {
	for _, item := range spec.Paths.Map() {
		if common := collectCommonParams(item); len(common) > 0 {
			item.Parameters = common
		}
	}
}

// collectCommonParams 返回该 path 上所有 operation 共有的 path 参数。
func collectCommonParams(item *openapi3.PathItem) openapi3.Parameters {
	operations := []*openapi3.Operation{}
	for _, op := range []*openapi3.Operation{item.Get, item.Post, item.Put, item.Patch, item.Delete} {
		if op != nil {
			operations = append(operations, op)
		}
	}
	if len(operations) == 0 {
		return nil
	}
	var common openapi3.Parameters
	// 取第一个 operation 的 path 参数作为候选。
	for _, param := range operations[0].Parameters {
		if param.Value != nil && param.Value.In == "path" {
			common = append(common, param)
		}
	}
	// 逐个 operation 核对。
	count := 0
	for _, op := range operations {
		found := false
		for _, param := range op.Parameters {
			if param.Value != nil && param.Value.In == "path" {
				found = true
				break
			}
		}
		if found {
			count++
		}
	}
	if count != len(operations) || len(common) == 0 {
		// 并非全部 operation 共用，保留 operation 级定义。
		return nil
	}
	// 从各 operation 移除已提升的 path 参数。
	for _, op := range operations {
		retained := op.Parameters[:0]
		for _, param := range op.Parameters {
			if param.Value == nil || param.Value.In != "path" {
				retained = append(retained, param)
			}
		}
		op.Parameters = retained
	}
	return common
}

// Validate 校验文档（含组件引用解析），供生成器确认渲染结果与提交的 YAML 一致。
// kin-openapi 对程序化构建的文档不会自动解析 components 内的 $ref，因此先序列化再重载校验。
func (d *Document) Validate() error {
	if d == nil || d.spec == nil {
		return fmt.Errorf("document is not built")
	}
	yamlBytes, err := d.MarshalYAML()
	if err != nil {
		return fmt.Errorf("marshal document for validation: %w", err)
	}
	loader := openapi3.NewLoader()
	reloaded, err := loader.LoadFromData(yamlBytes)
	if err != nil {
		return fmt.Errorf("reload generated OpenAPI: %w", err)
	}
	if err := reloaded.Validate(validationContext()); err != nil {
		return fmt.Errorf("validate rendered OpenAPI document: %w", err)
	}
	return nil
}

func newSpec(info Info) *openapi3.T {
	return &openapi3.T{
		OpenAPI: "3.0.3",
		Info: &openapi3.Info{
			Title: info.Title, Version: info.Version, Description: info.Description,
		},
		Components: &openapi3.Components{
			SecuritySchemes: openapi3.SecuritySchemes{
				"bearerAuth": &openapi3.SecuritySchemeRef{
					Value: &openapi3.SecurityScheme{Type: "http", Scheme: "bearer", BearerFormat: "JWT"},
				},
			},
			Schemas: openapi3.Schemas{
				"Problem":   renderSchema(problemSchema()),
				"Violation": renderSchema(violationSchema()),
			},
		},
		Paths: openapi3.NewPaths(),
	}
}

// addOperation 把一个模块 operation 渲染为 path item 并注册进 paths。
func addOperation(spec *openapi3.T, moduleName string, operation Operation) error {
	pathItem := spec.Paths.Find(operation.Path)
	if pathItem == nil {
		pathItem = &openapi3.PathItem{}
		spec.Paths.Set(operation.Path, pathItem)
	}
	op := &openapi3.Operation{
		OperationID: string(operation.ID),
		Tags:        operation.Tags,
		Responses:   openapi3.NewResponses(),
	}
	if len(operation.Tags) == 0 {
		op.Tags = []string{moduleName}
	}

	op.Extensions = map[string]any{}
	op.Extensions["x-policy"] = map[string]any{
		"mode": string(operation.Policy.Mode), "scope": operation.Policy.Scope, "action": operation.Policy.Action,
	}

	if operation.Security != SecurityNone {
		op.Security = &openapi3.SecurityRequirements{
			{"bearerAuth": []string{}},
		}
	}

	for _, param := range operation.Params {
		if param.Schema == nil {
			return fmt.Errorf("operation %q parameter %q has no schema", operation.ID, param.Name)
		}
		op.Parameters = append(op.Parameters, &openapi3.ParameterRef{
			Value: &openapi3.Parameter{
				Name: param.Name, In: string(param.Location), Required: param.Required,
				Schema: renderSchema(param.Schema), Description: param.Schema.Description,
			},
		})
	}

	if operation.Request != nil && operation.Request.Schema != nil {
		op.RequestBody = &openapi3.RequestBodyRef{
			Value: &openapi3.RequestBody{
				Required: true,
				Content: openapi3.Content{
					"application/json": &openapi3.MediaType{Schema: renderSchema(operation.Request.Schema)},
				},
			},
		}
	}

	for _, response := range operation.Responses {
		if response.Schema == nil {
			return fmt.Errorf("operation %q response %d has no schema", operation.ID, response.Status)
		}
		op.Responses.Set(fmt.Sprintf("%d", response.Status), &openapi3.ResponseRef{
			Value: &openapi3.Response{
				Description: &successDescription,
				Content: openapi3.Content{
					"application/json": &openapi3.MediaType{Schema: renderSchema(response.Schema)},
				},
			},
		})
	}

	op.Responses.Set("400", &openapi3.ResponseRef{Value: problemResponse("请求无效。")})
	op.Responses.Set("404", &openapi3.ResponseRef{Value: problemResponse("资源不存在。")})
	op.Responses.Set("409", &openapi3.ResponseRef{Value: problemResponse("资源冲突。")})
	op.Responses.Set("415", &openapi3.ResponseRef{Value: problemResponse("不支持的媒体类型。")})
	op.Responses.Set("429", &openapi3.ResponseRef{Value: problemResponseWithRetryAfter("请求过多，请稍后重试。")})
	op.Responses.Set("500", &openapi3.ResponseRef{Value: problemResponse("服务内部错误。")})
	op.Responses.Set("503", &openapi3.ResponseRef{Value: problemResponseWithRetryAfter("服务暂不可用，请稍后重试。")})
	op.Responses.Set("504", &openapi3.ResponseRef{Value: problemResponse("上游请求超时。")})
	op.Responses.Set("default", &openapi3.ResponseRef{Value: problemResponse("未预期的错误。")})

	switch operation.Method {
	case MethodGet:
		pathItem.Get = op
	case MethodPost:
		pathItem.Post = op
	case MethodPut:
		pathItem.Put = op
	case MethodPatch:
		pathItem.Patch = op
	case MethodDelete:
		pathItem.Delete = op
	default:
		return fmt.Errorf("operation %q uses unsupported method %q", operation.ID, operation.Method)
	}
	return nil
}

var successDescription = "成功响应。"

func problemResponse(description string) *openapi3.Response {
	return &openapi3.Response{
		Description: &description,
		Content: openapi3.Content{
			"application/problem+json": &openapi3.MediaType{
				Schema: &openapi3.SchemaRef{Ref: "#/components/schemas/Problem"},
			},
		},
	}
}

func problemResponseWithRetryAfter(description string) *openapi3.Response {
	response := problemResponse(description)
	response.Headers = openapi3.Headers{
		"Retry-After": &openapi3.HeaderRef{
			Value: &openapi3.Header{
				Parameter: openapi3.Parameter{
					Description: "建议客户端等待的秒数。",
					Schema: &openapi3.SchemaRef{
						Value: &openapi3.Schema{Type: &openapi3.Types{openapi3.TypeInteger}, Min: float64Ptr(1)},
					},
				},
			},
		},
	}
	return response
}

func float64Ptr(value float64) *float64 { return &value }

// renderSchema 把项目自有 Schema 树渲染为 openapi3.SchemaRef。
func renderSchema(schema *Schema) *openapi3.SchemaRef {
	if schema == nil {
		return nil
	}
	if schema.Kind == kindRef {
		return &openapi3.SchemaRef{Ref: "#/components/schemas/" + schema.RefName}
	}
	value := &openapi3.Schema{Description: schema.Description, Nullable: schema.IsNullable, Default: schema.DefaultVal}
	switch schema.Kind {
	case kindString:
		value.Type = &openapi3.Types{openapi3.TypeString}
		value.Format = schema.FormatName
		value.Pattern = schema.PatternStr
		value.MinLength = derefUint(schema.MinLen)
		value.MaxLength = schema.MaxLen
		value.Enum = schema.EnumValues
	case kindInteger:
		value.Type = &openapi3.Types{openapi3.TypeInteger}
		value.Format = schema.FormatName
		value.Min = schema.MinNumeric
		value.Max = schema.MaxNumeric
	case kindNumber:
		value.Type = &openapi3.Types{openapi3.TypeNumber}
		value.Format = schema.FormatName
		value.Min = schema.MinNumeric
		value.Max = schema.MaxNumeric
	case kindBoolean:
		value.Type = &openapi3.Types{openapi3.TypeBoolean}
	case kindArray:
		value.Type = &openapi3.Types{openapi3.TypeArray}
		value.Items = renderSchema(schema.Items)
	case kindObject:
		value.Type = &openapi3.Types{openapi3.TypeObject}
		value.Required = append([]string(nil), schema.RequiredNames...)
		value.Properties = openapi3.Schemas{}
		for _, property := range schema.Properties {
			value.Properties[property.Name] = renderSchema(property.Schema)
		}
		if !schema.AllowExtra {
			value.AdditionalProperties = openapi3.AdditionalProperties{Has: boolPtr(false)}
		}
	}
	return &openapi3.SchemaRef{Value: value}
}

func boolPtr(value bool) *bool { return &value }

func derefUint(value *uint64) uint64 {
	if value == nil {
		return 0
	}
	return *value
}

// MarshalYAML 序列化文档为 OpenAPI YAML；仅供生成器与「加载生成的 YAML」路径调用。
func (d *Document) MarshalYAML() ([]byte, error) {
	if d == nil || d.spec == nil {
		return nil, fmt.Errorf("document is not built")
	}
	rendered, err := d.spec.MarshalYAML()
	if err != nil {
		return nil, fmt.Errorf("marshal OpenAPI document: %w", err)
	}
	bytes, err := yaml.Marshal(rendered)
	if err != nil {
		return nil, fmt.Errorf("encode OpenAPI YAML: %w", err)
	}
	return bytes, nil
}
