package httpbinding

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"runtime"
	"testing"

	configbinding "github.com/rin721/go-scaffold-template/internal/module/ops/binding/config"
	"github.com/rin721/go-scaffold-template/internal/module/ops/model"
	"github.com/rin721/go-scaffold-template/internal/module/ops/service"
	"github.com/rin721/go-scaffold-template/pkg/logger"
)

type testSource struct{}

func (testSource) Snapshot(context.Context) (model.RuntimeSnapshot, error) {
	return model.RuntimeSnapshot{Started: true, Live: true, Ready: true, AuthReady: true, DatabaseReady: true}, nil
}
func (testSource) Readiness(context.Context) (bool, bool, error) { return true, true, nil }

type unhealthySource struct{}

func (unhealthySource) Snapshot(context.Context) (model.RuntimeSnapshot, error) {
	return model.RuntimeSnapshot{Started: true, Live: true, Ready: false}, nil
}
func (unhealthySource) Readiness(context.Context) (bool, bool, error) {
	return false, false, fmt.Errorf("readiness dependency unavailable")
}

type failingSource struct{}

func (failingSource) Snapshot(context.Context) (model.RuntimeSnapshot, error) {
	return model.RuntimeSnapshot{}, fmt.Errorf("runtime snapshot unavailable")
}
func (failingSource) Readiness(context.Context) (bool, bool, error) { return false, false, nil }

type testAccess struct{ allowed bool }

func (a testAccess) Authenticate(next http.Handler) http.Handler { return next }
func (a testAccess) Authorize(context.Context, string) error {
	if !a.allowed {
		return context.Canceled
	}
	return nil
}

func TestManagementRoutesExcludePprofAndProtectDiagnostics(t *testing.T) {
	service, _ := service.New(testSource{}, model.BuildInfo{Version: "v1", Commit: "abc", BuildTime: "now", GoVersion: runtime.Version()})
	handler, err := New(service, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("metric")) }), testAccess{}, configbinding.AccessDisabled, logger.Noop())
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	for _, test := range []struct {
		path string
		want int
	}{{"/startupz", 200}, {"/livez", 200}, {"/readyz", 200}, {"/diagnostics", 403}, {"/metrics", 404}, {"/debug/pprof/", 404}} {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, test.path, nil))
		if recorder.Code != test.want {
			t.Fatalf("GET %s status = %d, want %d", test.path, recorder.Code, test.want)
		}
	}
}

func TestManagementLogsRejectedAndFailedBoundaries(t *testing.T) {
	service, _ := service.New(failingSource{}, model.BuildInfo{Version: "v1", Commit: "abc", BuildTime: "now", GoVersion: runtime.Version()})
	logs := logger.NewTestLogger()
	handler, err := New(service, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("metric")) }), testAccess{}, configbinding.AccessProtected, logs)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	for _, test := range []struct {
		path string
		want int
	}{{"/diagnostics", 403}, {"/startupz", 503}} {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, test.path, nil))
		if recorder.Code != test.want {
			t.Fatalf("GET %s status = %d, want %d", test.path, recorder.Code, test.want)
		}
	}

	entries := logs.Entries()
	if len(entries) != 2 {
		t.Fatalf("entries = %d, want 2", len(entries))
	}
	if entries[0].Level != "warn" || entries[0].Message != "management operation rejected" {
		t.Fatalf("first entry = %#v", entries[0])
	}
	if entries[1].Level != "error" || entries[1].Message != "management operation failed" {
		t.Fatalf("second entry = %#v", entries[1])
	}
}

func TestManagementLogsUnhealthyProbeAsWarn(t *testing.T) {
	service, _ := service.New(unhealthySource{}, model.BuildInfo{Version: "v1", Commit: "abc", BuildTime: "now", GoVersion: runtime.Version()})
	logs := logger.NewTestLogger()
	handler, err := New(service, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("metric")) }), testAccess{allowed: true}, configbinding.AccessDisabled, logs)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("GET /readyz status = %d, want %d", recorder.Code, http.StatusServiceUnavailable)
	}

	entries := logs.Entries()
	if len(entries) != 1 {
		t.Fatalf("entries = %d, want 1", len(entries))
	}
	if entries[0].Level != "warn" || entries[0].Message != "management probe failed" {
		t.Fatalf("entry = %#v", entries[0])
	}
}
