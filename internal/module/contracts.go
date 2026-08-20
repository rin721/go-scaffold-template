// Package module 定义应用模块向组合根贡献的最小完成品契约。
package module

import (
	"fmt"
	"reflect"
	"sort"

	pkgmessaging "github.com/rin721/go-scaffold-template/pkg/messaging"
	pkgschedule "github.com/rin721/go-scaffold-template/pkg/schedule"
	"github.com/rin721/go-scaffold-template/pkg/supervisor"
)

// ID 是应用模块在单个进程内的稳定唯一标识。
type ID string

// Contribution 是应用模块交给组合根集中安装的完成品。
type Contribution struct {
	ID           ID
	Participants []supervisor.Participant
	Schedules    []pkgschedule.Binding
	Messages     pkgmessaging.Contribution
}

// ValidateContributions 在任何 listener 或 participant 启动前校验模块贡献。
func ValidateContributions(contributions ...Contribution) error {
	modules := make(map[ID]struct{}, len(contributions))
	owners := make(map[string]ID)
	scheduleOwners := make(map[pkgschedule.TaskID]ID)
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
		for scheduleIndex, binding := range contribution.Schedules {
			if err := binding.Validate(); err != nil {
				return fmt.Errorf("module %s schedule %d: %w", contribution.ID, scheduleIndex, err)
			}
			if owner, exists := scheduleOwners[binding.ID()]; exists {
				return fmt.Errorf("module schedule %q is shared by modules %s and %s", binding.ID(), owner, contribution.ID)
			}
			scheduleOwners[binding.ID()] = contribution.ID
		}
	}
	return nil
}

// ScheduleBindings 校验并按 Task ID 返回全部模块的不可变任务声明副本。
func ScheduleBindings(contributions ...Contribution) ([]pkgschedule.Binding, error) {
	if err := ValidateContributions(contributions...); err != nil {
		return nil, err
	}
	total := 0
	for _, contribution := range contributions {
		total += len(contribution.Schedules)
	}
	bindings := make([]pkgschedule.Binding, 0, total)
	for _, contribution := range contributions {
		bindings = append(bindings, contribution.Schedules...)
	}
	sort.Slice(bindings, func(left, right int) bool {
		return bindings[left].ID() < bindings[right].ID()
	})
	return bindings, nil
}

// MessageBindings 校验模块贡献并返回全局不可变消息 Catalog。
func MessageBindings(contributions ...Contribution) (pkgmessaging.Catalog, error) {
	if err := ValidateContributions(contributions...); err != nil {
		return pkgmessaging.Catalog{}, err
	}
	messages := make([]pkgmessaging.Contribution, 0, len(contributions))
	for _, contribution := range contributions {
		messages = append(messages, contribution.Messages)
	}
	catalog, err := pkgmessaging.BuildCatalog(messages...)
	if err != nil {
		return pkgmessaging.Catalog{}, fmt.Errorf("module message bindings: %w", err)
	}
	return catalog, nil
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
