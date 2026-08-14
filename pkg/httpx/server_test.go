package httpx

import (
	"context"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestNewServerWithDefaultsAndShutdown(t *testing.T) {
	router := NewRouter(nil)
	server, err := NewServer(&ServerConfig{Addr: "127.0.0.1:0"}, router)
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}

	if err := server.Start(t.Context()); err != nil {
		t.Fatalf("Start returned error: %v", err)
	}
	done := make(chan error, 1)
	go func() { done <- server.Run(t.Context()) }()
	<-server.Running()
	if err := server.Stop(context.Background()); err != nil {
		t.Fatalf("Stop returned error: %v", err)
	}
	if err := <-done; err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
}

func TestServerStartReportsBindFailureSynchronously(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve listener: %v", err)
	}
	defer listener.Close()
	server, err := NewServer(&ServerConfig{Addr: listener.Addr().String()}, http.NotFoundHandler())
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}
	if err := server.Start(t.Context()); err == nil {
		t.Fatal("Start(port conflict) error = nil")
	}
}

func TestServerStopWaitsForActiveRequest(t *testing.T) {
	requestStarted := make(chan struct{})
	releaseRequest := make(chan struct{})
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		close(requestStarted)
		<-releaseRequest
		_, _ = io.WriteString(w, "done")
	})
	server, err := NewServer(&ServerConfig{Addr: "127.0.0.1:0"}, handler)
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}
	if err := server.Start(t.Context()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	runDone := make(chan error, 1)
	go func() { runDone <- server.Run(t.Context()) }()
	<-server.Running()
	address, _ := server.Addr()
	responseDone := make(chan error, 1)
	go func() {
		response, requestErr := http.Get("http://" + address.String())
		if requestErr == nil {
			_ = response.Body.Close()
		}
		responseDone <- requestErr
	}()
	<-requestStarted
	stopCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	stopDone := make(chan error, 1)
	go func() { stopDone <- server.Stop(stopCtx) }()
	select {
	case err := <-stopDone:
		t.Fatalf("Stop() returned before active request drained: %v", err)
	default:
	}
	close(releaseRequest)
	if err := <-responseDone; err != nil {
		t.Fatalf("HTTP request error = %v", err)
	}
	if err := <-stopDone; err != nil {
		t.Fatalf("Stop() error = %v", err)
	}
	if err := <-runDone; err != nil {
		t.Fatalf("Run() error = %v", err)
	}
}

func TestServerConcurrentStopSharesOneCompletion(t *testing.T) {
	server, err := NewServer(&ServerConfig{Addr: "127.0.0.1:0"}, http.NotFoundHandler())
	if err != nil {
		t.Fatalf("new server: %v", err)
	}
	if err := server.Start(t.Context()); err != nil {
		t.Fatalf("start server: %v", err)
	}
	runDone := make(chan error, 1)
	go func() { runDone <- server.Run(t.Context()) }()
	<-server.Running()

	stopResults := make(chan error, 2)
	for range 2 {
		go func() { stopResults <- server.Stop(t.Context()) }()
	}
	for range 2 {
		if err := <-stopResults; err != nil {
			t.Fatalf("concurrent stop: %v", err)
		}
	}
	if err := <-runDone; err != nil {
		t.Fatalf("run after concurrent stop: %v", err)
	}
}

func TestNewServerRejectsInvalidConfig(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *ServerConfig
		handler http.Handler
		want    string
	}{
		{name: "nil handler", handler: nil, want: "handler"},
		{name: "negative timeout", cfg: &ServerConfig{ReadTimeout: -1}, handler: http.NewServeMux(), want: "timeouts"},
		{name: "negative max header", cfg: &ServerConfig{MaxHeaderBytes: -1}, handler: http.NewServeMux(), want: "max header"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewServer(tt.cfg, tt.handler)
			if err == nil {
				t.Fatal("NewServer returned nil error")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error %q does not contain %q", err.Error(), tt.want)
			}
		})
	}
}
