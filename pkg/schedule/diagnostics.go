package schedule

import "time"

// RuntimeState 是任务在当前 Generation 中的低敏运行状态。
type RuntimeState string

const (
	StateDisabled   RuntimeState = "disabled"
	StatePrepared   RuntimeState = "prepared"
	StateLocal      RuntimeState = "local"
	StateContending RuntimeState = "contending"
	StateLeader     RuntimeState = "leader"
	StateStandby    RuntimeState = "standby"
	StateDegraded   RuntimeState = "degraded"
	StatePaused     RuntimeState = "paused"
	StateWeakened   RuntimeState = "weakened"
	StateFailed     RuntimeState = "failed"
	StateStopping   RuntimeState = "stopping"
)

// TaskSnapshot 是一个任务的并发安全只读诊断快照。
type TaskSnapshot struct {
	ID              TaskID            `json:"id"`
	Trigger         TriggerKind       `json:"trigger"`
	Coordination    CoordinationMode  `json:"coordination"`
	Unavailable     UnavailablePolicy `json:"unavailablePolicy,omitempty"`
	State           RuntimeState      `json:"state"`
	Ready           bool              `json:"ready"`
	Active          int               `json:"active"`
	Queued          int               `json:"queued"`
	Runs            uint64            `json:"runs"`
	Skipped         uint64            `json:"skipped"`
	LastScheduledAt time.Time         `json:"lastScheduledAt,omitempty"`
	LastStartedAt   time.Time         `json:"lastStartedAt,omitempty"`
	LastCompletedAt time.Time         `json:"lastCompletedAt,omitempty"`
	LastErrorType   string            `json:"lastErrorType,omitempty"`
}

// Diagnostics 是当前 Generation 调度能力的聚合只读视图。
type Diagnostics struct {
	Enabled    bool           `json:"enabled"`
	Ready      bool           `json:"ready"`
	Degraded   bool           `json:"degraded"`
	Generation uint64         `json:"generation"`
	Tasks      []TaskSnapshot `json:"tasks"`
}
