package composition

import (
	"fmt"

	"github.com/rin721/go-scaffold-template/internal/kernel/app"
	executionapp "github.com/rin721/go-scaffold-template/internal/kernel/app/execution"
)

// composeExecution 装配 Execution 组件，供业务模块消费幂等/重试/执行记录能力。
func composeExecution(plan *app.Plan) (app.Added[executionapp.Access], error) {
	definition, err := executionapp.Definition()
	if err != nil {
		return app.Added[executionapp.Access]{}, fmt.Errorf("define execution app: %w", err)
	}
	added, err := app.Add(plan, definition)
	if err != nil {
		return app.Added[executionapp.Access]{}, fmt.Errorf("compose execution app: %w", err)
	}
	return added, nil
}
