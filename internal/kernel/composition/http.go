package composition

import (
	"context"
	"fmt"

	"github.com/rin721/go-scaffold2/internal/kernel/config"
	pkghttpx "github.com/rin721/go-scaffold2/pkg/httpx"
)

const (
	httpSectionID  = "application.http"
	httpConfigPath = "http"
)

// HTTPConfiguration 返回 application-owned HTTP 配置节契约。
func HTTPConfiguration() config.Binding {
	return config.Binding{
		CapabilityID: httpSectionID,
		ConfigPath:   httpConfigPath,
		Contract:     httpDefaults{},
		Validate: func(snapshot config.Snapshot) error {
			_, err := HTTPServerConfig(snapshot)
			return err
		},
	}
}

// HTTPServerConfig 从同一 application candidate 严格绑定 HTTP 配置。
func HTTPServerConfig(snapshot config.Snapshot) (pkghttpx.ServerConfig, error) {
	resolved := pkghttpx.DefaultServerConfig()
	if err := snapshot.DecodeSection(httpConfigPath, &resolved); err != nil {
		return pkghttpx.ServerConfig{}, fmt.Errorf("decode HTTP configuration: %w", err)
	}
	if err := pkghttpx.ValidateServerConfig(&resolved); err != nil {
		return pkghttpx.ServerConfig{}, err
	}
	return resolved, nil
}

type httpDefaults struct{}

func (httpDefaults) Defaults(ctx context.Context) (config.Object, config.Control, error) {
	if ctx == nil {
		return nil, config.Continue, fmt.Errorf("HTTP defaults context is nil")
	}
	if err := ctx.Err(); err != nil {
		return nil, config.Continue, err
	}
	value := pkghttpx.DefaultServerConfig()
	maxHeaderBytes, err := config.Number(fmt.Sprintf("%d", value.MaxHeaderBytes))
	if err != nil {
		return nil, config.Continue, err
	}
	return config.Object{
		config.FieldOf("addr", config.String(value.Addr)),
		config.FieldOf("readHeaderTimeout", config.Duration(value.ReadHeaderTimeout)),
		config.FieldOf("readTimeout", config.Duration(value.ReadTimeout)),
		config.FieldOf("writeTimeout", config.Duration(value.WriteTimeout)),
		config.FieldOf("idleTimeout", config.Duration(value.IdleTimeout)),
		config.FieldOf("maxHeaderBytes", maxHeaderBytes),
	}, config.Continue, nil
}

var _ config.DefaultContract = httpDefaults{}
