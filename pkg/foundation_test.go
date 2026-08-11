package pkg_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rin721/go-scaffold2/pkg/health"
	"github.com/rin721/go-scaffold2/pkg/httpx"
	"github.com/rin721/go-scaffold2/pkg/idgen"
	"github.com/rin721/go-scaffold2/pkg/logger"
	"github.com/rin721/go-scaffold2/pkg/storage"
	"github.com/rin721/go-scaffold2/pkg/supervisor"
)

func TestFoundationLibrariesCanBeCreatedIndependently(t *testing.T) {
	log, err := logger.New(nil)
	if err != nil {
		t.Fatalf("logger.New() error = %v", err)
	}
	t.Cleanup(func() {
		if err := log.Close(); err != nil {
			t.Errorf("logger.Close() error = %v", err)
		}
	})

	requestID, err := idgen.UUID().New()
	if err != nil {
		t.Fatalf("idgen.New() error = %v", err)
	}
	log.Info("foundation libraries created", logger.String("request_id", requestID))

	router := httpx.NewRouter(nil)
	router.Use(httpx.RequestID(nil))
	router.Handle(httpx.MethodGet, "/ok", func(ctx *httpx.Context) error {
		if requestID, ok := httpx.RequestIDFromContext(ctx.Request.Context()); !ok || requestID == "" {
			t.Fatal("request id missing from context")
		}
		return ctx.Text(http.StatusOK, "ok")
	})
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/ok", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("http status = %d", recorder.Code)
	}

	fileStorage, err := storage.New(&storage.Config{Local: storage.LocalConfig{FSType: storage.FSTypeMemory, EnableWatch: false}})
	if err != nil {
		t.Fatalf("storage.New() error = %v", err)
	}
	defer fileStorage.Close()
	if err := fileStorage.WriteFile("health.txt", []byte("ok"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	registry := health.New(time.Second)
	if err := registry.Register("storage", func(context.Context) health.Result {
		exists, err := fileStorage.Exists("health.txt")
		if err != nil || !exists {
			return health.Result{Error: err}
		}
		return health.Result{Status: health.StatusPass}
	}); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	if got := registry.Snapshot(context.Background()).Status; got != health.StatusPass {
		t.Fatalf("health status = %q, want pass", got)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	processSupervisor := supervisor.New(supervisor.Config{ShutdownTimeout: time.Second})
	if err := processSupervisor.Run(ctx); err != nil {
		t.Fatalf("empty supervisor Run() error = %v", err)
	}
}
