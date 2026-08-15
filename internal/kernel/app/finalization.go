package app

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// FinalizationPhase 表示实例终结责任所在的生命周期位置。
type FinalizationPhase string

const (
	FinalizationPhaseCandidate FinalizationPhase = "candidate"
	FinalizationPhaseRetired   FinalizationPhase = "retired"
	FinalizationPhaseCurrent   FinalizationPhase = "current"
)

// OwnershipState 表示实例所有权责任的安全诊断状态。
type OwnershipState string

const (
	OwnershipServing             OwnershipState = "serving"
	OwnershipWaitingForDrain     OwnershipState = "waiting-for-drain"
	OwnershipFinalizationPending OwnershipState = "finalization-pending"
	OwnershipFinalizing          OwnershipState = "finalizing"
	OwnershipFinalized           OwnershipState = "finalized"
	OwnershipTerminalFailed      OwnershipState = "terminal-failed"
)

// FinalizationPolicy 表示一个实例由 Definition 冻结的终结场景。
type FinalizationPolicy string

const (
	NoFinalization         FinalizationPolicy = "no-finalization"
	DrainThenTerminalClose FinalizationPolicy = "drain-then-terminal-close"
)

// ReleaseVerification 表示运行时是否额外证明了物理资源释放。
type ReleaseVerification string

const (
	VerificationNotRequired ReleaseVerification = "not-required"
	VerificationNotProven   ReleaseVerification = "not-proven"
)

// OwnershipSnapshot 是不暴露实例、配置值和原始错误文本的所有权快照。
type OwnershipSnapshot struct {
	ComponentID        ID
	InstanceGeneration uint64
	Phase              FinalizationPhase
	State              OwnershipState
	Policy             FinalizationPolicy
	Attempt            uint32
	Verification       ReleaseVerification
	LastErrorType      string
	Since              time.Time
}

type instanceSlot[I any] struct {
	mu sync.Mutex

	generation   uint64
	instance     I
	phase        FinalizationPhase
	state        OwnershipState
	policy       FinalizationPolicy
	verification ReleaseVerification
	attempts     uint32
	result       error
	deactivated  bool
	since        time.Time
}

func newInstanceSlot[I any](generation uint64, instance I, phase FinalizationPhase, policy FinalizationPolicy) *instanceSlot[I] {
	if policy != NoFinalization && policy != DrainThenTerminalClose {
		panic("component finalization policy is invalid")
	}
	verification := VerificationNotProven
	if policy == NoFinalization {
		verification = VerificationNotRequired
	}
	return &instanceSlot[I]{
		generation:   generation,
		instance:     instance,
		phase:        phase,
		state:        OwnershipFinalizationPending,
		policy:       policy,
		verification: verification,
		since:        time.Now(),
	}
}

func (s *instanceSlot[I]) snapshot(componentID ID) OwnershipSnapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	snapshot := OwnershipSnapshot{
		ComponentID:        componentID,
		InstanceGeneration: s.generation,
		Phase:              s.phase,
		State:              s.state,
		Policy:             s.policy,
		Attempt:            s.attempts,
		Verification:       s.verification,
		Since:              s.since,
	}
	if s.result != nil {
		snapshot.LastErrorType = fmt.Sprintf("%T", s.result)
	}
	return snapshot
}

func (s *instanceSlot[I]) transition(phase FinalizationPhase, state OwnershipState) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.phase == phase && s.state == state {
		return
	}
	s.phase = phase
	s.state = state
	s.since = time.Now()
}

func (s *instanceSlot[I]) beginFinalization() (I, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state == OwnershipFinalized || s.state == OwnershipTerminalFailed {
		return s.instance, false, s.result
	}
	if s.policy == NoFinalization {
		s.state = OwnershipFinalized
		s.since = time.Now()
		return s.instance, false, nil
	}
	s.state = OwnershipFinalizing
	s.attempts++
	s.since = time.Now()
	return s.instance, true, nil
}

func (s *instanceSlot[I]) finishFinalization(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.result = err
	if err != nil {
		s.state = OwnershipTerminalFailed
	} else {
		s.state = OwnershipFinalized
	}
	s.since = time.Now()
}

func (s *instanceSlot[I]) markDeactivated() (I, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.deactivated {
		return s.instance, false
	}
	s.deactivated = true
	return s.instance, true
}

func finalizeSlot[I any](ctx context.Context, slot *instanceSlot[I], finalizer TerminalFinalizer[I]) error {
	if slot == nil {
		return nil
	}
	instance, run, result := slot.beginFinalization()
	if !run {
		return result
	}
	result = finalizer(ctx, instance)
	slot.finishFinalization(result)
	return result
}
