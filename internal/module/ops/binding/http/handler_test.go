package httpbinding

import (
	"context"
	"net/http"
	"net/http/httptest"
	"runtime"
	"testing"

	configbinding "github.com/rin721/go-scaffold-template/internal/module/ops/binding/config"
	"github.com/rin721/go-scaffold-template/internal/module/ops/model"
	"github.com/rin721/go-scaffold-template/internal/module/ops/service"
)

type testSource struct{}

func (testSource) Snapshot(context.Context) (model.RuntimeSnapshot, error) {
	return model.RuntimeSnapshot{Started: true, Live: true, Ready: true, AuthReady: true, DatabaseReady: true}, nil
}
func (testSource) Readiness(context.Context) (bool, bool, error) { return true, true, nil }

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
	handler, err := New(service, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("metric")) }), testAccess{}, configbinding.AccessDisabled)
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
