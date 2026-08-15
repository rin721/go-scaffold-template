// Package model 定义 Ops module 对外稳定的管理与观测契约。
package model

import (
	"time"

	"github.com/rin721/go-scaffold-template/pkg/observability"
)

// ProbeKind 是管理端点支持的有限探针集合。
type ProbeKind string

const (
	ProbeStartup  ProbeKind = "startup"
	ProbeLiveness ProbeKind = "liveness"
	ProbeReady    ProbeKind = "readiness"
)

const (
	// OperationDiagnostics 是完整诊断读取的稳定审计 identity。
	OperationDiagnostics = "ops.diagnostics"
	// OperationMetrics 是受保护 metrics 读取的稳定审计 identity。
	OperationMetrics = "ops.metrics"
)

// BuildInfo 是允许通过管理面公开的非敏感构建元数据。
type BuildInfo struct {
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	BuildTime string `json:"buildTime"`
	GoVersion string `json:"goVersion"`
	Dirty     bool   `json:"dirty"`
}

// RuntimeSnapshot 是 composition 向 Ops module 提供的只读进程视图。
type RuntimeSnapshot struct {
	Started           bool                      `json:"started"`
	Live              bool                      `json:"live"`
	Ready             bool                      `json:"ready"`
	ProcessState      string                    `json:"processState"`
	GenerationState   string                    `json:"generationState"`
	Generation        uint64                    `json:"generation"`
	Phase             string                    `json:"phase"`
	ConfiguredAddress string                    `json:"configuredAddress,omitempty"`
	BoundAddress      string                    `json:"boundAddress,omitempty"`
	ActiveRequests    int64                     `json:"activeRequests"`
	ActiveConnections int64                     `json:"activeConnections"`
	AuthReady         bool                      `json:"authReady"`
	DatabaseReady     bool                      `json:"databaseReady"`
	CleanupRequired   bool                      `json:"cleanupRequired"`
	LastFailurePhase  string                    `json:"lastFailurePhase,omitempty"`
	LastFailureOwner  string                    `json:"lastFailureOwner,omitempty"`
	LastFailureType   string                    `json:"lastFailureType,omitempty"`
	Telemetry         observability.Diagnostics `json:"telemetry"`
	Since             time.Time                 `json:"since"`
}

// Probe 是公开探针的最小响应，不包含内部失败细节。
type Probe struct {
	Status string `json:"status"`
}
