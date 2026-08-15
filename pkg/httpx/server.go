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
	serverForced
	serverFailed
)

// Server 统一拥有 listener、http.Server、Serve 运行结果和停止等待。
type Server struct {
	server *http.Server

	mu       sync.Mutex
	state    serverState
	listener net.Listener
	runDone  chan struct{}
	running  chan struct{}
	runErr   error

	gracefulOnce sync.Once
	gracefulDone chan struct{}
	gracefulErr  error
	forceOnce    sync.Once
	forceDone    chan struct{}
	forceErr     error
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
			Addr: resolved.Addr, Handler: handler,
			ReadHeaderTimeout: resolved.ReadHeaderTimeout,
			ReadTimeout:       resolved.ReadTimeout, WriteTimeout: resolved.WriteTimeout,
			IdleTimeout: resolved.IdleTimeout, MaxHeaderBytes: resolved.MaxHeaderBytes,
			ErrorLog: resolved.ErrorLog,
		},
		runDone: make(chan struct{}), running: make(chan struct{}),
		gracefulDone: make(chan struct{}), forceDone: make(chan struct{}),
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

// Run 阻塞执行预绑定 listener 上的 Serve；非停止意图导致的退出视为失败。
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
	intentional := s.state == serverStopping || s.state == serverStopped || s.state == serverForced
	if errors.Is(err, http.ErrServerClosed) && intentional {
		err = nil
	} else if err != nil {
		err = fmt.Errorf("serve http server: %w", err)
		s.state = serverFailed
	} else if !intentional {
		err = fmt.Errorf("http server serve completed unexpectedly")
		s.state = serverFailed
	}
	s.runErr = err
	close(s.runDone)
	s.mu.Unlock()
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

// Stop 只执行 graceful shutdown；失败或超时不会暗中调用 Close。
func (s *Server) Stop(ctx context.Context) error {
	if s == nil || s.server == nil {
		return fmt.Errorf("http server is nil")
	}
	if ctx == nil {
		return fmt.Errorf("http server stop context is nil")
	}
	s.gracefulOnce.Do(func() {
		s.gracefulErr = s.gracefulStop(ctx)
		close(s.gracefulDone)
	})
	select {
	case <-s.gracefulDone:
		return s.gracefulErr
	case <-ctx.Done():
		return fmt.Errorf("wait concurrent http server stop: %w", ctx.Err())
	}
}

func (s *Server) gracefulStop(ctx context.Context) error {
	s.mu.Lock()
	state := s.state
	listener := s.listener
	switch state {
	case serverCreated:
		s.state = serverStopped
		s.mu.Unlock()
		return nil
	case serverStopped:
		err := s.gracefulErr
		s.mu.Unlock()
		return err
	case serverForced:
		s.mu.Unlock()
		return fmt.Errorf("http server was force stopped")
	case serverFailed:
		err := s.runErr
		s.mu.Unlock()
		return err
	case serverStarted, serverRunning:
		s.state = serverStopping
	case serverStopping:
	}
	s.mu.Unlock()

	if state == serverStarted {
		if err := listener.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
			return fmt.Errorf("close unserved http listener: %w", err)
		}
		s.mu.Lock()
		s.state = serverStopped
		s.mu.Unlock()
		return nil
	}
	if err := s.server.Shutdown(ctx); err != nil {
		return fmt.Errorf("shutdown http server: %w", err)
	}
	select {
	case <-s.runDone:
		s.mu.Lock()
		err := s.runErr
		if err == nil && s.state != serverForced {
			s.state = serverStopped
		}
		s.mu.Unlock()
		return err
	case <-ctx.Done():
		return fmt.Errorf("wait http server graceful stop: %w", ctx.Err())
	}
}

// ForceStop 显式中断 HTTP 连接，并在给定剩余预算内确认 Serve 已结束。
func (s *Server) ForceStop(ctx context.Context) error {
	if s == nil || s.server == nil {
		return fmt.Errorf("http server is nil")
	}
	if ctx == nil {
		return fmt.Errorf("http server force stop context is nil")
	}
	s.forceOnce.Do(func() {
		s.forceErr = s.forceStop(ctx)
		close(s.forceDone)
	})
	select {
	case <-s.forceDone:
		return s.forceErr
	case <-ctx.Done():
		return fmt.Errorf("wait concurrent http server force stop: %w", ctx.Err())
	}
}

func (s *Server) forceStop(ctx context.Context) error {
	s.mu.Lock()
	state := s.state
	listener := s.listener
	if state == serverCreated {
		s.state = serverStopped
		s.mu.Unlock()
		return nil
	}
	if state == serverStopped || state == serverForced {
		s.mu.Unlock()
		return nil
	}
	s.state = serverStopping
	s.mu.Unlock()
	if state == serverStarted {
		if err := listener.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
			return fmt.Errorf("force close unserved http listener: %w", err)
		}
		s.mu.Lock()
		s.state = serverForced
		s.mu.Unlock()
		return nil
	}
	closeErr := s.server.Close()
	if closeErr != nil && !errors.Is(closeErr, http.ErrServerClosed) {
		closeErr = fmt.Errorf("force close http server: %w", closeErr)
	} else {
		closeErr = nil
	}
	select {
	case <-s.runDone:
		s.mu.Lock()
		runErr := s.runErr
		if closeErr == nil && runErr == nil {
			s.state = serverForced
		}
		s.mu.Unlock()
		return errors.Join(closeErr, runErr)
	case <-ctx.Done():
		return errors.Join(closeErr, fmt.Errorf("wait http server force stop: %w", ctx.Err()))
	}
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

var _ interface {
	ForceStop(context.Context) error
} = (*Server)(nil)
