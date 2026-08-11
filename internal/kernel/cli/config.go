package cli

import (
	"fmt"
	"reflect"

	"github.com/rin721/go-scaffold2/internal/kernel/config"
	pkgcli "github.com/rin721/go-scaffold2/pkg/cli"
)

const (
	configCommandName = "config"
	initCommandName   = "init"
	outputFlagName    = "output"
	forceFlagName     = "force"
	defaultOutputPath = "config.yaml"
)

// ConfigCommands 返回调用 DefaultManager 的 config init 启动前命令契约。
func ConfigCommands(manager config.DefaultManager) Contract {
	return ContractFunc(func() ([]pkgcli.CommandSpec, error) {
		if isNilDefaultManager(manager) {
			return nil, fmt.Errorf("default configuration manager is nil")
		}
		return []pkgcli.CommandSpec{{
			Name:        configCommandName,
			Description: "管理项目配置",
			Commands: []pkgcli.CommandSpec{{
				Name:        initCommandName,
				Description: "生成当前项目的默认配置文件",
				Flags: []pkgcli.FlagSpec{
					{
						Name:        outputFlagName,
						Shorthand:   "o",
						Type:        pkgcli.FlagTypeString,
						Default:     defaultOutputPath,
						Description: "默认配置文件输出路径",
					},
					{
						Name:        forceFlagName,
						Shorthand:   "f",
						Type:        pkgcli.FlagTypeBool,
						Default:     false,
						Description: "替换已经存在的目标文件",
					},
				},
				Args: func(ctx *pkgcli.Context) error {
					if len(ctx.Args) != 0 {
						return fmt.Errorf("config init accepts no positional arguments")
					}
					return nil
				},
				Run: func(ctx *pkgcli.Context) error {
					result, err := manager.Generate(ctx.Context, config.GenerateRequest{
						Path:  ctx.GetString(outputFlagName),
						Force: ctx.GetBool(forceFlagName),
					})
					if err != nil {
						return err
					}
					if _, err := fmt.Fprintf(ctx.Stdout, "created default configuration: %s\n", result.Path); err != nil {
						return fmt.Errorf("write config init result: %w", err)
					}
					return nil
				},
			}},
		}}, nil
	})
}

func isNilDefaultManager(manager config.DefaultManager) bool {
	if manager == nil {
		return true
	}
	value := reflect.ValueOf(manager)
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return value.IsNil()
	default:
		return false
	}
}
