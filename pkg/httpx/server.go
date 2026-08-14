package httpx

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"sync"
)

type serverState uint8

const (
	serverCreated serverState = iota
	serverStarted
	serverRunning
	serverStopping
	serverStopped
)

// Server 统一拥有 listener、http.Server、Serve 运行结果和停止等待。
type Server struct {
	server *http.Server

	mu       sync.Mutex
	state    serverState
	listener net.Listener
	done     chan error
	running  chan struct{}
	stopDone chan struct{}
	stopErr  error
}

// NewServer 根据配置和 Handler 创建尚未绑定端口的 HTTP 服务端。
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
		done:     make(chan error, 1),
		running:  make(chan struct{}),
		stopDone: make(chan struct{}),
	}, nil
}

// ValidateServerConfig 校验服务端配置且不创建 listener 或 goroutine。
func ValidateServerConfig(cfg *ServerConfig) error {
	if _, err := resolveServerConfig(cfg); err != nil {
		return fmt.Errorf("resolve http server config: %w", err)
	}
	return nil
}

// Name 返回进程监督使用的稳定 owner ID。
func (*Server) Name() string { return "http-server" }

// Start 在启动阶段同步预绑定 listener，使地址错误在进程 ready 前返回。
func (s *Server) Start(ctx context.Context) error {
	if s == nil || s.server == nil {
		return fmt.Errorf("http server is nil")
	}
	if ctx == nil {
		return fmt.Errorf("http server start context is nil")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state != serverCreated {
		return fmt.Errorf("http server already started")
	}
	listener, err := net.Listen("tcp", s.server.Addr)
	if err != nil {
		return fmt.Errorf("bind http server %s: %w", s.server.Addr, err)
	}
	s.listener = listener
	s.state = serverStarted
	return nil
}

// Run 阻塞执行预绑定 listener 上的 Serve；任何非终止意图导致的退出都视为失败。
func (s *Server) Run(ctx context.Context) error {
	if s == nil || s.server == nil {
		return fmt.Errorf("http server is nil")
	}
	if ctx == nil {
		return fmt.Errorf("http server run context is nil")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	if s.state != serverStarted {
		s.mu.Unlock()
		return fmt.Errorf("http server is not started")
	}
	s.state = serverRunning
	close(s.running)
	listener := s.listener
	s.mu.Unlock()

	err := s.server.Serve(listener)
	s.mu.Lock()
	stopping := s.state == serverStopping || s.state == serverStopped
	if !stopping {
		s.state = serverStopped
	}
	s.mu.Unlock()
	if errors.Is(err, http.ErrServerClosed) && stopping {
		err = nil
	} else if err != nil {
		err = fmt.Errorf("serve http server: %w", err)
	} else if !stopping {
		err = fmt.Errorf("http server serve completed unexpectedly")
	}
	s.done <- err
	return err
}

// Running 在 Serve 已经取得预绑定 listener 后关闭，供进程 readiness 汇总。
func (s *Server) Running() <-chan struct{} {
	if s == nil {
		closed := make(chan struct{})
		close(closed)
		return closed
	}
	return s.running
}

// Stop 停止接收新连接、排空活跃请求并等待 Serve 返回。
func (s *Server) Stop(ctx context.Context) error {
	if s == nil || s.server == nil {
		return fmt.Errorf("http server is nil")
	}
	if ctx == nil {
		return fmt.Errorf("http server stop context is nil")
	}
	s.mu.Lock()
	state := s.state
	if state == serverStopped {
		err := s.stopErr
		s.mu.Unlock()
		return err
	}
	if state == serverStopping {
		done := s.stopDone
		s.mu.Unlock()
		select {
		case <-done:
			s.mu.Lock()
			err := s.stopErr
			s.mu.Unlock()
			return err
		case <-ctx.Done():
			return fmt.Errorf("wait concurrent http server stop: %w", ctx.Err())
		}
	}
	if state == serverCreated {
		s.state = serverStopped
		close(s.stopDone)
		s.mu.Unlock()
		return nil
	}
	s.state = serverStopping
	listener := s.listener
	s.mu.Unlock()

	if state == serverStarted {
		var stopErr error
		if err := listener.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
			stopErr = fmt.Errorf("close unserved http listener: %w", err)
		}
		return s.completeStop(stopErr)
	}

	shutdownErr := s.server.Shutdown(ctx)
	if shutdownErr != nil {
		shutdownErr = fmt.Errorf("shutdown http server: %w", shutdownErr)
		if closeErr := s.server.Close(); closeErr != nil && !errors.Is(closeErr, http.ErrServerClosed) {
			shutdownErr = errors.Join(shutdownErr, fmt.Errorf("force close http server: %w", closeErr))
		}
	}
	select {
	case runErr := <-s.done:
		return s.completeStop(errors.Join(shutdownErr, runErr))
	case <-ctx.Done():
		return s.completeStop(errors.Join(shutdownErr, fmt.Errorf("wait http server: %w", ctx.Err())))
	}
}

func (s *Server) completeStop(err error) error {
	s.mu.Lock()
	s.stopErr = err
	s.state = serverStopped
	close(s.stopDone)
	s.mu.Unlock()
	return err
}

// Addr 返回启动后实际绑定的地址，支持端口 0 的测试与嵌入式场景。
func (s *Server) Addr() (net.Addr, bool) {
	if s == nil {
		return nil, false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.listener == nil {
		return nil, false
	}
	return s.listener.Addr(), true
}
