package supervisor

import (
	"fmt"
	"time"
)

// UnitKind 表示进程监督责任的种类。
type UnitKind string

const (
	UnitParticipant UnitKind = "participant"
	UnitTask        UnitKind = "task"
)

// UnitPhase 表示责任当前执行的阶段。
type UnitPhase string

const (
	UnitPhaseStart UnitPhase = "start"
	UnitPhaseReady UnitPhase = "ready"
	UnitPhaseRun   UnitPhase = "run"
	UnitPhaseStop  UnitPhase = "stop"
	UnitPhaseForce UnitPhase = "force"
)

// UnitState 表示责任当前是否仍运行、已完成或已明确失败。
type UnitState string

const (
	UnitPending UnitState = "pending"
	UnitRunning UnitState = "running"
	UnitReady   UnitState = "ready"
	UnitStopped UnitState = "stopped"
	UnitForced  UnitState = "forced"
	UnitFailed  UnitState = "failed"
)

// ExitPolicy 表示责任由真实契约确定的退出政策。
type ExitPolicy string

const (
	ExitGracefulShutdown  ExitPolicy = "graceful-shutdown"
	ExitGracefulThenForce ExitPolicy = "graceful-then-force"
	ExitCancelAndWait     ExitPolicy = "cancel-and-wait"
)

// ShutdownPhase 表示共享关闭预算当前所处阶段。
type ShutdownPhase string

const (
	ShutdownNotStarted ShutdownPhase = "not-started"
	ShutdownGraceful   ShutdownPhase = "graceful"
	ShutdownForce      ShutdownPhase = "force"
	ShutdownComplete   ShutdownPhase = "complete"
)

// UnitSnapshot 是不包含用户对象、配置和原始错误文本的责任快照。
type UnitSnapshot struct {
	Owner         string
	Kind          UnitKind
	Phase         UnitPhase
	State         UnitState
	ExitPolicy    ExitPolicy
	Attempt       uint32
	LastErrorType string
	Since         time.Time
}

// ShutdownBudgetSnapshot 描述一次进程关闭共享的绝对期限。
type ShutdownBudgetSnapshot struct {
	Phase            ShutdownPhase
	StartedAt        time.Time
	GracefulDeadline time.Time
	FinalDeadline    time.Time
	Exhausted        bool
}

func (s *Supervisor) initializeUnitsLocked(participants []Participant, tasks []Task) {
	s.diagnostics.Units = make([]UnitSnapshot, 0, len(participants)+len(tasks))
	s.unitIndexes = make(map[string]int, len(participants)+len(tasks))
	now := time.Now()
	for _, participant := range participants {
		policy := ExitGracefulShutdown
		if _, ok := participant.(ForceStopper); ok {
			policy = ExitGracefulThenForce
		}
		s.unitIndexes[participant.Name()] = len(s.diagnostics.Units)
		s.diagnostics.Units = append(s.diagnostics.Units, UnitSnapshot{
			Owner: participant.Name(), Kind: UnitParticipant, Phase: UnitPhaseStart,
			State: UnitPending, ExitPolicy: policy, Since: now,
		})
	}
	for _, task := range tasks {
		s.unitIndexes[task.Name] = len(s.diagnostics.Units)
		s.diagnostics.Units = append(s.diagnostics.Units, UnitSnapshot{
			Owner: task.Name, Kind: UnitTask, Phase: UnitPhaseRun,
			State: UnitPending, ExitPolicy: ExitCancelAndWait, Since: now,
		})
	}
}

func (s *Supervisor) transitionUnit(owner string, phase UnitPhase, state UnitState, incrementAttempt bool, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.transitionUnitLocked(owner, phase, state, incrementAttempt, err)
}

func (s *Supervisor) transitionUnitLocked(owner string, phase UnitPhase, state UnitState, incrementAttempt bool, err error) {
	index, ok := s.unitIndexes[owner]
	if !ok {
		panic(fmt.Sprintf("supervisor diagnostics owner %q is not registered", owner))
	}
	unit := &s.diagnostics.Units[index]
	if unit.Phase != phase {
		unit.Phase = phase
		unit.Attempt = 0
	}
	if incrementAttempt {
		unit.Attempt++
	}
	unit.State = state
	unit.Since = time.Now()
	if err != nil {
		unit.LastErrorType = fmt.Sprintf("%T", err)
	}
}

func (s *Supervisor) settleUnstartedUnits() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for index := range s.diagnostics.Units {
		unit := &s.diagnostics.Units[index]
		if unit.State == UnitPending && unit.Attempt == 0 {
			unit.State = UnitStopped
			unit.Since = time.Now()
		}
	}
}

func cloneUnits(units []UnitSnapshot) []UnitSnapshot {
	return append([]UnitSnapshot(nil), units...)
}

func unitsClean(units []UnitSnapshot) bool {
	for _, unit := range units {
		if unit.State != UnitStopped {
			return false
		}
	}
	return true
}
