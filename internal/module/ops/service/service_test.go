package service

import (
	"context"
	"runtime"
	"testing"

	"github.com/rin721/go-scaffold-template/internal/module/ops/model"
)

type sourceFunc func(context.Context) (model.RuntimeSnapshot, error)

func (f sourceFunc) Snapshot(ctx context.Context) (model.RuntimeSnapshot, error) { return f(ctx) }
func (f sourceFunc) Readiness(context.Context) (bool, bool, error)               { return true, false, nil }

func TestProbeSemanticsRemainDistinct(t *testing.T) {
	snapshot := model.RuntimeSnapshot{Started: true, Live: true, Ready: true, AuthReady: true, DatabaseReady: false}
	service, err := New(sourceFunc(func(context.Context) (model.RuntimeSnapshot, error) { return snapshot, nil }), model.BuildInfo{Version: "v1", Commit: "abc", BuildTime: "now", GoVersion: runtime.Version()})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	for _, test := range []struct {
		kind model.ProbeKind
		want bool
	}{{model.ProbeStartup, true}, {model.ProbeLiveness, true}, {model.ProbeReady, false}} {
		probe, passing, err := service.Probe(t.Context(), test.kind)
		if err != nil || passing != test.want {
			t.Fatalf("Probe(%s) = %#v, %t, %v", test.kind, probe, passing, err)
		}
	}
}
