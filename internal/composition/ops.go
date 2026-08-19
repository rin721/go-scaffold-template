package composition

import (
	"context"
	"fmt"
	"net/http"
	"sync"

	"github.com/rin721/go-scaffold-template/internal/kernel"
	"github.com/rin721/go-scaffold-template/internal/module/auth"
	authmodel "github.com/rin721/go-scaffold-template/internal/module/auth/model"
	httpbinding "github.com/rin721/go-scaffold-template/internal/module/ops/binding/http"
	opsmodel "github.com/rin721/go-scaffold-template/internal/module/ops/model"
	todohttp "github.com/rin721/go-scaffold-template/internal/module/todo/binding/http"
	pkgobservability "github.com/rin721/go-scaffold-template/pkg/observability"
	"github.com/rin721/go-scaffold-template/pkg/supervisor"
)

type opsAccessAdapter struct{ auth auth.Module }

func (a opsAccessAdapter) Authenticate(next http.Handler) http.Handler {
	return a.auth.HTTPMiddleware(next)
}
func (a opsAccessAdapter) Authorize(ctx context.Context, operation string) error {
	principal, ok := authmodel.PrincipalFromContext(ctx)
	if !ok {
		return authmodel.ErrUnauthenticated
	}
	return a.auth.Service.EnforceOperation(ctx, principal, operation)
}

type opsRuntimeSource struct {
	mu          sync.RWMutex
	coordinator *kernel.GenerationCoordinator
	supervisor  *supervisor.Supervisor
}

func (s *opsRuntimeSource) connect(coordinator *kernel.GenerationCoordinator, process *supervisor.Supervisor) error {
	if coordinator == nil || process == nil {
		return fmt.Errorf("ops runtime source dependencies are incomplete")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.coordinator != nil || s.supervisor != nil {
		return fmt.Errorf("ops runtime source is already connected")
	}
	s.coordinator, s.supervisor = coordinator, process
	return nil
}

func (s *opsRuntimeSource) Snapshot(ctx context.Context) (opsmodel.RuntimeSnapshot, error) {
	if ctx == nil {
		return opsmodel.RuntimeSnapshot{}, fmt.Errorf("ops runtime context is nil")
	}
	s.mu.RLock()
	coordinator, process := s.coordinator, s.supervisor
	s.mu.RUnlock()
	if coordinator == nil || process == nil {
		return opsmodel.RuntimeSnapshot{}, fmt.Errorf("ops runtime source is not connected")
	}
	generationDiagnostics := coordinator.Diagnostics()
	processDiagnostics := process.Snapshot()
	result := opsmodel.RuntimeSnapshot{
		Started:      generationDiagnostics.CurrentGeneration != 0,
		Live:         processDiagnostics.State != supervisor.StateFailed && processDiagnostics.State != supervisor.StateStopped,
		Ready:        generationDiagnostics.Ready && processDiagnostics.Ready,
		ProcessState: string(processDiagnostics.State), GenerationState: string(generationDiagnostics.State),
		Generation: generationDiagnostics.CurrentGeneration, Phase: generationDiagnostics.Phase,
		ConfiguredAddress: generationDiagnostics.ConfiguredAddress, BoundAddress: generationDiagnostics.BoundAddress,
		ActiveRequests: generationDiagnostics.ActiveRequests, ActiveConnections: generationDiagnostics.ActiveConnections,
		CleanupRequired:  generationDiagnostics.CleanupRequired,
		LastFailurePhase: generationDiagnostics.LastFailurePhase, LastFailureOwner: generationDiagnostics.LastFailureOwner,
		LastFailureType: generationDiagnostics.LastFailureType, Since: generationDiagnostics.Since,
	}
	return result, nil
}

type generationOpsSource struct {
	process    *opsRuntimeSource
	generation *applicationGeneration
}

func (s generationOpsSource) Snapshot(ctx context.Context) (opsmodel.RuntimeSnapshot, error) {
	result, err := s.process.Snapshot(ctx)
	if err != nil {
		return opsmodel.RuntimeSnapshot{}, err
	}
	result.AuthReady = s.generation.authModule.Service.Ready()
	telemetry, diagnosticsErr := s.generation.telemetry.output.Diagnostics(ctx)
	if diagnosticsErr != nil {
		return opsmodel.RuntimeSnapshot{}, fmt.Errorf("read telemetry diagnostics: %w", diagnosticsErr)
	}
	result.Telemetry = telemetry
	return result, nil
}

func (s generationOpsSource) Readiness(ctx context.Context) (bool, bool, error) {
	if ctx == nil {
		return false, false, fmt.Errorf("ops readiness context is nil")
	}
	if s.generation == nil {
		return false, false, nil
	}
	authReady := s.generation.authModule.Service.Ready()
	if err := s.generation.database.value().Ping(ctx); err != nil {
		return authReady, false, nil
	}
	return authReady, true, nil
}

func opsOperations() []pkgobservability.Operation {
	module := todohttp.ModuleContract()
	result := make([]pkgobservability.Operation, 0, len(module.Operations))
	for _, operation := range module.Operations {
		result = append(result, pkgobservability.Operation{ID: string(operation.ID), Method: string(operation.Method), Path: operation.Path})
	}
	return result
}

var _ httpbinding.Access = opsAccessAdapter{}
