package composition

import (
	"fmt"

	"github.com/rin721/go-scaffold-template/internal/kernel/app"
	executionapp "github.com/rin721/go-scaffold-template/internal/kernel/app/execution"
	"github.com/rin721/go-scaffold-template/internal/kernel/logging"
)

// composeExecution 装配 Execution 组件，注入结构化 Logger 依赖；
// 供业务模块消费幂等/重试/执行记录能力与恢复治理观测。
func composeExecution(plan *app.Plan, logger app.Binding[logging.Target]) (app.Added[executionapp.Access], error) {
	definition, err := executionapp.Definition(app.InputOf(logger))
	if err != nil {
		return app.Added[executionapp.Access]{}, fmt.Errorf("define execution app: %w", err)
	}
	added, err := app.Add(plan, definition)
	if err != nil {
		return app.Added[executionapp.Access]{}, fmt.Errorf("compose execution app: %w", err)
	}
	return added, nil
}
