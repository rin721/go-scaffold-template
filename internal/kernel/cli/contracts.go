// Package cli 负责组合仅在 Kernel 启动前运行的可选命令契约。
package cli

import (
	"fmt"
	"reflect"

	pkgcli "github.com/rin721/go-scaffold2/pkg/cli"
)

// Contract 向启动前 CLI 贡献一个或多个项目自有命令声明。
type Contract interface {
	Commands() ([]pkgcli.CommandSpec, error)
}

// ContractFunc 把函数适配为 CLI Contract。
type ContractFunc func() ([]pkgcli.CommandSpec, error)

// Commands 调用被适配的命令声明函数。
func (f ContractFunc) Commands() ([]pkgcli.CommandSpec, error) { return f() }

// NewApp 按 Contract 和 CommandSpec 声明顺序构造完整 CLI App。
func NewApp(cfg pkgcli.Config, contracts ...Contract) (pkgcli.App, error) {
	app, err := pkgcli.NewApp(cfg)
	if err != nil {
		return nil, err
	}
	for index, contract := range contracts {
		if isNilContract(contract) {
			return nil, fmt.Errorf("CLI contract %d is nil", index)
		}
		commands, err := contract.Commands()
		if err != nil {
			return nil, fmt.Errorf("load CLI contract %d: %w", index, err)
		}
		if len(commands) == 0 {
			return nil, fmt.Errorf("CLI contract %d returned no commands", index)
		}
		for commandIndex, command := range commands {
			if err := app.AddCommand(command); err != nil {
				return nil, fmt.Errorf("register CLI contract %d command %d: %w", index, commandIndex, err)
			}
		}
	}
	return app, nil
}

func isNilContract(contract Contract) bool {
	if contract == nil {
		return true
	}
	value := reflect.ValueOf(contract)
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return value.IsNil()
	default:
		return false
	}
}
