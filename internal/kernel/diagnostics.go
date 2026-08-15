package kernel

import (
	"time"

	"github.com/rin721/go-scaffold-template/internal/kernel/app"
	"github.com/rin721/go-scaffold-template/pkg/supervisor"
)

// OwnerKind 表示统一诊断中的责任所有者种类。
type OwnerKind string

const (
	OwnerCapability  OwnerKind = "capability"
	OwnerParticipant OwnerKind = "participant"
	OwnerTask        OwnerKind = "task"
)

// ResponsibilityPhase 是底层 typed phase 在进程视图中的稳定投影。
type ResponsibilityPhase string

// ResponsibilityState 是底层 typed state 在进程视图中的稳定投影。
type ResponsibilityState string

const (
	ResponsibilityServing             ResponsibilityState = "serving"
	ResponsibilityWaitingForDrain     ResponsibilityState = "waiting-for-drain"
	ResponsibilityFinalizationPending ResponsibilityState = "finalization-pending"
	ResponsibilityFinalizing          ResponsibilityState = "finalizing"
	ResponsibilityFinalized           ResponsibilityState = "finalized"
	ResponsibilityTerminalFailed      ResponsibilityState = "terminal-failed"
	ResponsibilityPending             ResponsibilityState = "pending"
	ResponsibilityRunning             ResponsibilityState = "running"
	ResponsibilityReady               ResponsibilityState = "ready"
	ResponsibilityStopped             ResponsibilityState = "stopped"
	ResponsibilityForced              ResponsibilityState = "forced"
	ResponsibilityFailed              ResponsibilityState = "failed"
)

// ExitPolicy 是资源或运行责任真实契约确定的退出政策。
type ExitPolicy string

const (
	ExitNoFinalization         ExitPolicy = "no-finalization"
	ExitDrainThenTerminalClose ExitPolicy = "drain-then-terminal-close"
	ExitGracefulShutdown       ExitPolicy = "graceful-shutdown"
	ExitGracefulThenForce      ExitPolicy = "graceful-then-force"
	ExitCancelAndWait          ExitPolicy = "cancel-and-wait"
)

// ReleaseVerification 表示责任是否需要或已经额外证明物理资源释放。
type ReleaseVerification string

const (
	ReleaseVerificationNotApplicable ReleaseVerification = "not-applicable"
	ReleaseVerificationNotRequired   ReleaseVerification = "not-required"
	ReleaseVerificationNotProven     ReleaseVerification = "not-proven"
)

// ResponsibilitySnapshot 是一项 capability、participant 或 task 的脱敏责任快照。
type ResponsibilitySnapshot struct {
	Owner               string
	Kind                OwnerKind
	Generation          uint64
	Phase               ResponsibilityPhase
	State               ResponsibilityState
	ExitPolicy          ExitPolicy
	Attempt             uint32
	ReleaseVerification ReleaseVerification
	LastErrorType       string
	Since               time.Time
}

// ResponsibilityRef 是 pending 索引使用的稳定责任身份。
type ResponsibilityRef struct {
	Owner      string
	Kind       OwnerKind
	Generation uint64
}

// ProcessDiagnostics 是 Host 对 management consumer 提供的唯一进程诊断 authority。
type ProcessDiagnostics struct {
	ProcessState     supervisor.ProcessState
	KernelState      LifecycleState
	Ready            bool
	ConfigGeneration uint64
	ConfigDigest     string
	ConfigProvenance []string
	RestartRequired  bool
	CleanupRequired  bool
	KernelErrorType  string
	ProcessErrorType string
	ShutdownBudget   supervisor.ShutdownBudgetSnapshot
	Responsibilities []ResponsibilitySnapshot
	PendingUnits     []ResponsibilityRef
	Since            time.Time
}

func composeProcessDiagnostics(kernelState CoordinatorDiagnostics, processState supervisor.Snapshot) ProcessDiagnostics {
	result := ProcessDiagnostics{
		ProcessState: processState.State, KernelState: kernelState.State,
		Ready:            kernelState.Ready && processState.Ready,
		ConfigGeneration: kernelState.ConfigGeneration,
		ConfigDigest:     kernelState.ConfigDigest,
		ConfigProvenance: append([]string(nil), kernelState.ConfigProvenance...),
		RestartRequired:  kernelState.RestartRequired,
		CleanupRequired:  kernelState.CleanupRequired,
		KernelErrorType:  kernelState.LastFailureType,
		ProcessErrorType: processState.LastErrorType,
		ShutdownBudget:   processState.Budget,
		Since:            laterTime(kernelState.Since, processState.Since),
	}
	result.Responsibilities = make([]ResponsibilitySnapshot, 0, len(kernelState.Ownerships)+len(processState.Units))
	for _, ownership := range kernelState.Ownerships {
		responsibility := mapCapabilityOwnership(ownership)
		result.Responsibilities = append(result.Responsibilities, responsibility)
		if responsibilityPending(responsibility.State) {
			result.PendingUnits = append(result.PendingUnits, responsibilityRef(responsibility))
		}
	}
	for _, unit := range processState.Units {
		responsibility := mapSupervisorUnit(unit)
		result.Responsibilities = append(result.Responsibilities, responsibility)
		if responsibilityPending(responsibility.State) {
			result.PendingUnits = append(result.PendingUnits, responsibilityRef(responsibility))
		}
	}
	return result
}

func mapCapabilityOwnership(snapshot app.OwnershipSnapshot) ResponsibilitySnapshot {
	return ResponsibilitySnapshot{
		Owner: string(snapshot.ComponentID), Kind: OwnerCapability,
		Generation: snapshot.InstanceGeneration,
		Phase:      ResponsibilityPhase(snapshot.Phase), State: ResponsibilityState(snapshot.State),
		ExitPolicy: ExitPolicy(snapshot.Policy), Attempt: snapshot.Attempt,
		ReleaseVerification: ReleaseVerification(snapshot.Verification),
		LastErrorType:       snapshot.LastErrorType, Since: snapshot.Since,
	}
}

func mapSupervisorUnit(snapshot supervisor.UnitSnapshot) ResponsibilitySnapshot {
	kind := OwnerParticipant
	if snapshot.Kind == supervisor.UnitTask {
		kind = OwnerTask
	}
	return ResponsibilitySnapshot{
		Owner: snapshot.Owner, Kind: kind,
		Phase: ResponsibilityPhase(snapshot.Phase), State: ResponsibilityState(snapshot.State),
		ExitPolicy: ExitPolicy(snapshot.ExitPolicy), Attempt: snapshot.Attempt,
		ReleaseVerification: ReleaseVerificationNotApplicable,
		LastErrorType:       snapshot.LastErrorType, Since: snapshot.Since,
	}
}

func responsibilityPending(state ResponsibilityState) bool {
	switch state {
	case ResponsibilityWaitingForDrain, ResponsibilityFinalizationPending, ResponsibilityFinalizing, ResponsibilityPending:
		return true
	default:
		return false
	}
}

func responsibilityRef(snapshot ResponsibilitySnapshot) ResponsibilityRef {
	return ResponsibilityRef{Owner: snapshot.Owner, Kind: snapshot.Kind, Generation: snapshot.Generation}
}

func laterTime(first, second time.Time) time.Time {
	if first.After(second) {
		return first
	}
	return second
}
