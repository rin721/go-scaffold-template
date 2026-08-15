// Package service 实现 Ops module 的探针、构建信息与诊断用例。
package service

import (
	"context"
	"fmt"

	"github.com/rin721/go-scaffold-template/internal/module/ops/model"
)

// RuntimeSource 是 Ops 使用方定义的只读进程状态端口。
type RuntimeSource interface {
	Snapshot(context.Context) (model.RuntimeSnapshot, error)
	Readiness(context.Context) (authReady bool, databaseReady bool, err error)
}

// Service 只解释运行状态，不拥有 listener 或进程生命周期。
type Service struct {
	source RuntimeSource
	build  model.BuildInfo
}

// New 构造 Ops 用例。
func New(source RuntimeSource, build model.BuildInfo) (*Service, error) {
	if source == nil {
		return nil, fmt.Errorf("ops runtime source is nil")
	}
	if build.Version == "" || build.Commit == "" || build.BuildTime == "" || build.GoVersion == "" {
		return nil, fmt.Errorf("ops build information is incomplete")
	}
	return &Service{source: source, build: build}, nil
}

// Probe 计算 startup、liveness 或 readiness 的最小状态。
func (s *Service) Probe(ctx context.Context, kind model.ProbeKind) (model.Probe, bool, error) {
	snapshot, err := s.snapshot(ctx)
	if err != nil {
		return model.Probe{}, false, err
	}
	var passing bool
	switch kind {
	case model.ProbeStartup:
		passing = snapshot.Started
	case model.ProbeLiveness:
		passing = snapshot.Live
	case model.ProbeReady:
		authReady, databaseReady, readinessErr := s.source.Readiness(ctx)
		if readinessErr != nil {
			return model.Probe{Status: "fail"}, false, nil
		}
		passing = snapshot.Ready && authReady && databaseReady
	default:
		return model.Probe{}, false, fmt.Errorf("unsupported ops probe %q", kind)
	}
	status := "fail"
	if passing {
		status = "pass"
	}
	return model.Probe{Status: status}, passing, nil
}

// Diagnostics 返回已经由 composition 投影和脱敏的 typed 状态。
func (s *Service) Diagnostics(ctx context.Context) (model.RuntimeSnapshot, error) {
	snapshot, err := s.snapshot(ctx)
	if err != nil {
		return model.RuntimeSnapshot{}, err
	}
	snapshot.AuthReady, snapshot.DatabaseReady, _ = s.source.Readiness(ctx)
	return snapshot, nil
}

// Build 返回构造时冻结的构建元数据。
func (s *Service) Build() model.BuildInfo { return s.build }

func (s *Service) snapshot(ctx context.Context) (model.RuntimeSnapshot, error) {
	if ctx == nil {
		return model.RuntimeSnapshot{}, fmt.Errorf("ops context is nil")
	}
	if err := ctx.Err(); err != nil {
		return model.RuntimeSnapshot{}, err
	}
	return s.source.Snapshot(ctx)
}
