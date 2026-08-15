// Package api 保存由公开 OpenAPI authority 生成的 HTTP DTO 与 server binding。
//
//go:generate go tool oapi-codegen --config ../../../../api/oapi-codegen.yaml ../../../../api/openapi.yaml
//go:generate go run ../../../tools/openapi-inventory -input ../../../../api/openapi.yaml -output operation_inventory.gen.go -package api
package api
