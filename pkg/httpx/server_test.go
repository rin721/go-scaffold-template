package httpx

import (
	"context"
	"net/http"
	"strings"
	"testing"
)

func TestNewServerWithDefaultsAndShutdown(t *testing.T) {
	router := NewRouter(nil)
	server, err := NewServer(nil, router)
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}

	if err := server.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown returned error: %v", err)
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
