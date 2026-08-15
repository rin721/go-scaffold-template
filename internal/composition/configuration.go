package composition

import (
	composed "github.com/rin721/go-scaffold-template/internal/kernel/composition"
	"github.com/rin721/go-scaffold-template/internal/kernel/config"
	authconfig "github.com/rin721/go-scaffold-template/internal/module/auth/binding/config"
	migrationconfig "github.com/rin721/go-scaffold-template/internal/module/migration/binding/config"
	opsconfig "github.com/rin721/go-scaffold-template/internal/module/ops/binding/config"
	todoconfig "github.com/rin721/go-scaffold-template/internal/module/todo/binding/config"
)

// applicationOwnedConfigurationBindings 返回统一正式配置中由应用模块拥有的 section。
// Bootstrap、Service 与 one-shot CLI 必须调用本函数，避免各入口复制集合后发生漂移。
func applicationOwnedConfigurationBindings() []config.Binding {
	return []config.Binding{
		authconfig.Binding(),
		migrationconfig.Binding(),
		todoconfig.Binding(),
		opsconfig.Binding(),
		composed.ObservabilityConfiguration(),
	}
}
