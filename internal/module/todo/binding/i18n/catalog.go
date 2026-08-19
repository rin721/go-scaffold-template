// Package i18nbinding 提供 Todo 模块的 i18n binding（033）。
//
// i18n 是业务模块的正式 binding 契约：业务模块按统一方式提供自身的 i18n 语言资源，
// 并由 internal/composition 显式聚合到 Kernel I18n 装配，再按模块注入 pkg/i18n.Translator；
// 业务 handler 只消费注入的 Translator，不直接读取 pkg/i18n 默认配置。
package i18nbinding

// MessageFiles 返回 Todo 模块自有的语言资源路径（相对进程工作目录）。
// 新增或修改 Todo 语言内容在此处声明的模块语言文件（./internal/module/todo/binding/i18n/locales/）中维护。
// composition 在装配 i18n 时把本返回值并入 i18n.messageFiles。
func MessageFiles() []string {
	return []string{
		"./internal/module/todo/binding/i18n/locales/messages.zh-CN.yaml",
	}
}
