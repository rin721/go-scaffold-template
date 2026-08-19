package contract

import (
	"context"
)

// validationContext 提供文档验证所需的 context。
func validationContext() context.Context { return context.Background() }
