package app

import (
	"context"
	"fmt"
)

// FinalizationPhase 表示实例终结责任所在的生命周期位置。
type FinalizationPhase string

const (
	FinalizationPhaseCandidate FinalizationPhase = "candidate"
	FinalizationPhaseRetired   FinalizationPhase = "retired"
	FinalizationPhaseCurrent   FinalizationPhase = "current"
)

// FinalizationState 表示实例终结责任的安全诊断状态。
type FinalizationState string

const (
	FinalizationWaitingForDrain FinalizationState = "waiting-for-drain"
	FinalizationPending         FinalizationState = "pending"
	FinalizationRunning         FinalizationState = "running"
	FinalizationSucceeded       FinalizationState = "finalized"
	FinalizationTerminalFailed  FinalizationState = "terminal-failed"
)

// FinalizationSnapshot 是不暴露实例、配置值和原始错误文本的终结快照。
type FinalizationSnapshot struct {
	ComponentID        ID
	InstanceGeneration uint64
	Phase              FinalizationPhase
	State              FinalizationState
	Attempts           uint32
	LastErrorType      string
}

type instanceSlot[I any] struct {
	generation  uint64
	instance    I
	phase       FinalizationPhase
	state       FinalizationState
	attempts    uint32
	result      error
	deactivated bool
}

func (s *instanceSlot[I]) snapshot(componentID ID) FinalizationSnapshot {
	snapshot := FinalizationSnapshot{
		ComponentID:        componentID,
		InstanceGeneration: s.generation,
		Phase:              s.phase,
		State:              s.state,
		Attempts:           s.attempts,
	}
	if s.result != nil {
		snapshot.LastErrorType = fmt.Sprintf("%T", s.result)
	}
	return snapshot
}

func finalizeSlot[I any](ctx context.Context, slot *instanceSlot[I], finalizer TerminalFinalizer[I]) error {
	if slot == nil {
		return nil
	}
	if slot.state == FinalizationSucceeded || slot.state == FinalizationTerminalFailed {
		return slot.result
	}
	if finalizer == nil {
		slot.state = FinalizationSucceeded
		return nil
	}
	slot.state = FinalizationRunning
	slot.attempts++
	slot.result = finalizer(ctx, slot.instance)
	if slot.result != nil {
		slot.state = FinalizationTerminalFailed
		return slot.result
	}
	slot.state = FinalizationSucceeded
	return nil
}
