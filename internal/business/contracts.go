// Package business 定义业务模块向应用组合根贡献的最小完成品契约。
package business

import (
	"fmt"
	"net/url"
	"path"
	"reflect"
	"strings"

	"github.com/rin721/go-scaffold2/pkg/httpx"
	"github.com/rin721/go-scaffold2/pkg/supervisor"
)

// ModuleID 是业务模块在单个进程内的稳定唯一标识。
type ModuleID string

// Route 是已经绑定 Handler 的 HTTP 路由贡献。
type Route struct {
	Method      httpx.Method
	Path        string
	Handler     httpx.Handler
	Middlewares []httpx.Middleware
}

// Contribution 是业务模块交给应用组合根集中安装的完成品。
type Contribution struct {
	ID           ModuleID
	Routes       []Route
	Participants []supervisor.Participant
}

// ValidateContributions 在任何 listener 或 participant 启动前校验模块贡献。
func ValidateContributions(contributions ...Contribution) error {
	modules := make(map[ModuleID]struct{}, len(contributions))
	routes := make(map[string]ModuleID)
	owners := make(map[string]ModuleID)
	for index, contribution := range contributions {
		if contribution.ID == "" {
			return fmt.Errorf("business contribution %d module id is required", index)
		}
		if _, exists := modules[contribution.ID]; exists {
			return fmt.Errorf("business module %q is duplicated", contribution.ID)
		}
		modules[contribution.ID] = struct{}{}
		for routeIndex, route := range contribution.Routes {
			key, err := routeKey(route)
			if err != nil {
				return fmt.Errorf("business module %s route %d: %w", contribution.ID, routeIndex, err)
			}
			if owner, exists := routes[key]; exists {
				return fmt.Errorf("business route %s is shared by modules %s and %s", key, owner, contribution.ID)
			}
			routes[key] = contribution.ID
		}
		for participantIndex, participant := range contribution.Participants {
			if nilInterface(participant) {
				return fmt.Errorf("business module %s participant %d is nil", contribution.ID, participantIndex)
			}
			name := participant.Name()
			if name == "" {
				return fmt.Errorf("business module %s participant %d name is required", contribution.ID, participantIndex)
			}
			if owner, exists := owners[name]; exists {
				return fmt.Errorf("business participant %q is shared by modules %s and %s", name, owner, contribution.ID)
			}
			owners[name] = contribution.ID
		}
	}
	return nil
}

func routeKey(route Route) (string, error) {
	if !supportedMethod(route.Method) {
		return "", fmt.Errorf("unsupported HTTP method %q", route.Method)
	}
	if route.Handler == nil {
		return "", fmt.Errorf("HTTP handler is nil")
	}
	if route.Path == "" || !strings.HasPrefix(route.Path, "/") {
		return "", fmt.Errorf("HTTP path %q must be absolute", route.Path)
	}
	parsed, err := url.ParseRequestURI(route.Path)
	if err != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", fmt.Errorf("HTTP path %q is invalid", route.Path)
	}
	canonical := path.Clean(route.Path)
	if canonical != route.Path {
		return "", fmt.Errorf("HTTP path %q is not canonical; want %q", route.Path, canonical)
	}
	for index, middleware := range route.Middlewares {
		if middleware == nil {
			return "", fmt.Errorf("HTTP middleware %d is nil", index)
		}
	}
	return string(route.Method) + " " + route.Path, nil
}

func supportedMethod(method httpx.Method) bool {
	switch method {
	case httpx.MethodGet, httpx.MethodPost, httpx.MethodPut, httpx.MethodPatch,
		httpx.MethodDelete, httpx.MethodHead, httpx.MethodOptions:
		return true
	default:
		return false
	}
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
