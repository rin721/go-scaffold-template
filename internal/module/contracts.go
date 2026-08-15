// Package module 定义应用模块向组合根贡献的最小完成品契约。
package module

import (
	"fmt"
	"reflect"

	"github.com/rin721/go-scaffold-template/pkg/supervisor"
)

// ID 是应用模块在单个进程内的稳定唯一标识。
type ID string

// Contribution 是应用模块交给组合根集中安装的完成品。
type Contribution struct {
	ID           ID
	Participants []supervisor.Participant
}

// ValidateContributions 在任何 listener 或 participant 启动前校验模块贡献。
func ValidateContributions(contributions ...Contribution) error {
	modules := make(map[ID]struct{}, len(contributions))
	owners := make(map[string]ID)
	for index, contribution := range contributions {
		if contribution.ID == "" {
			return fmt.Errorf("module contribution %d module id is required", index)
		}
		if _, exists := modules[contribution.ID]; exists {
			return fmt.Errorf("module %q is duplicated", contribution.ID)
		}
		modules[contribution.ID] = struct{}{}
		for participantIndex, participant := range contribution.Participants {
			if nilInterface(participant) {
				return fmt.Errorf("module %s participant %d is nil", contribution.ID, participantIndex)
			}
			name := participant.Name()
			if name == "" {
				return fmt.Errorf("module %s participant %d name is required", contribution.ID, participantIndex)
			}
			if owner, exists := owners[name]; exists {
				return fmt.Errorf("module participant %q is shared by modules %s and %s", name, owner, contribution.ID)
			}
			owners[name] = contribution.ID
		}
	}
	return nil
}

func nilInterface(value any) bool {
	if value == nil {
		return true
	}
	reflected := reflect.ValueOf(value)
	switch reflected.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return reflected.IsNil()
	default:
		return false
	}
}
