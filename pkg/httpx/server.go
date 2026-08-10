package httpx

import (
	"context"
	"errors"
	"fmt"
	"net/http"
)

// Server 封装 HTTP 服务端生命周期。
type Server struct {
	server *http.Server
}

// NewServer 根据配置和 Handler 创建 HTTP 服务端。
func NewServer(cfg *ServerConfig, handler http.Handler) (*Server, error) {
	if handler == nil {
		return nil, fmt.Errorf("http server handler is nil")
	}

	resolved, err := resolveServerConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("resolve http server config: %w", err)
	}

	return &Server{
		server: &http.Server{
			Addr:              resolved.Addr,
			Handler:           handler,
			ReadHeaderTimeout: resolved.ReadHeaderTimeout,
			ReadTimeout:       resolved.ReadTimeout,
			WriteTimeout:      resolved.WriteTimeout,
			IdleTimeout:       resolved.IdleTimeout,
			MaxHeaderBytes:    resolved.MaxHeaderBytes,
			ErrorLog:          resolved.ErrorLog,
		},
	}, nil
}

// Start 启动 HTTP 服务。
func (s *Server) Start() error {
	if err := s.server.ListenAndServe(); err != nil {
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return fmt.Errorf("start http server: %w", err)
	}
	return nil
}

// Shutdown 优雅关闭 HTTP 服务。
func (s *Server) Shutdown(ctx context.Context) error {
	if err := s.server.Shutdown(ctx); err != nil {
		return fmt.Errorf("shutdown http server: %w", err)
	}
	return nil
}
