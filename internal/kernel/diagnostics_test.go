package kernel

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/rin721/go-scaffold-template/internal/kernel/app"
	"github.com/rin721/go-scaffold-template/pkg/supervisor"
)

func TestComposeProcessDiagnosticsMapsTypedResponsibilitiesAndPendingUnits(t *testing.T) {
	now := time.Now()
	kernelState := CoordinatorDiagnostics{
		State: LifecycleCleanupPending, Ready: false,
		ConfigGeneration: 3, ConfigDigest: "sha256:abc",
		ConfigProvenance: []string{"config.yaml", "APP_"},
		RestartRequired:  true, CleanupRequired: true,
		LastFailureType: "*kernel.DrainIncompleteError", Since: now,
		Ownerships: []app.OwnershipSnapshot{
			{
				ComponentID: "database", InstanceGeneration: 4,
				Phase: app.FinalizationPhaseCurrent, State: app.OwnershipWaitingForDrain,
				Policy: app.DrainThenTerminalClose, Verification: app.VerificationNotProven,
				Since: now,
			},
			{
				ComponentID: "logger", InstanceGeneration: 2,
				Phase: app.FinalizationPhaseRetired, State: app.OwnershipTerminalFailed,
				Policy: app.DrainThenTerminalClose, Attempt: 1,
				Verification: app.VerificationNotProven, LastErrorType: "*errors.errorString", Since: now,
			},
		},
	}
	processState := supervisor.Snapshot{
		State: supervisor.StateFailed, LastErrorType: "*errors.joinError", Since: now.Add(time.Second),
		Budget: supervisor.ShutdownBudgetSnapshot{Phase: supervisor.ShutdownComplete},
		Units: []supervisor.UnitSnapshot{
			{Owner: "http-server", Kind: supervisor.UnitParticipant, Phase: supervisor.UnitPhaseForce, State: supervisor.UnitForced, ExitPolicy: supervisor.ExitGracefulThenForce, Attempt: 1, Since: now},
			{Owner: "consumer", Kind: supervisor.UnitTask, Phase: supervisor.UnitPhaseStop, State: supervisor.UnitPending, ExitPolicy: supervisor.ExitCancelAndWait, Since: now},
		},
	}

	diagnostics := composeProcessDiagnostics(kernelState, processState)
	if diagnostics.Ready || diagnostics.ConfigGeneration != 3 || diagnostics.ProcessState != supervisor.StateFailed || diagnostics.KernelState != LifecycleCleanupPending {
		t.Fatalf("diagnostics = %#v", diagnostics)
	}
	if len(diagnostics.Responsibilities) != 4 || len(diagnostics.PendingUnits) != 2 {
		t.Fatalf("responsibilities/pending = %#v / %#v", diagnostics.Responsibilities, diagnostics.PendingUnits)
	}
	if diagnostics.PendingUnits[0] != (ResponsibilityRef{Owner: "database", Kind: OwnerCapability, Generation: 4}) ||
		diagnostics.PendingUnits[1] != (ResponsibilityRef{Owner: "consumer", Kind: OwnerTask}) {
		t.Fatalf("pending = %#v", diagnostics.PendingUnits)
	}
	forced := findResponsibility(t, diagnostics, OwnerParticipant, "http-server")
	if forced.State != ResponsibilityForced || forced.ExitPolicy != ExitGracefulThenForce || forced.ReleaseVerification != ReleaseVerificationNotApplicable {
		t.Fatalf("forced responsibility = %#v", forced)
	}
	failed := findResponsibility(t, diagnostics, OwnerCapability, "logger")
	if failed.State != ResponsibilityTerminalFailed || failed.Attempt != 1 || responsibilityPending(failed.State) {
		t.Fatalf("failed responsibility = %#v", failed)
	}
	if diagnostics.Since != processState.Since {
		t.Fatalf("Since = %s, want %s", diagnostics.Since, processState.Since)
	}
}

func TestComposeProcessDiagnosticsCopiesInputsAndOmitsRawSecrets(t *testing.T) {
	secret := "postgres://user:secret-password@example.invalid/database"
	kernelState := CoordinatorDiagnostics{
		State: LifecycleFailed, ConfigProvenance: []string{"config.yaml"},
		Ownerships: []app.OwnershipSnapshot{{
			ComponentID: "database", InstanceGeneration: 1,
			State: app.OwnershipTerminalFailed, Policy: app.DrainThenTerminalClose,
			Verification: app.VerificationNotProven, LastErrorType: "*errors.errorString",
		}},
	}
	processState := supervisor.Snapshot{
		State: supervisor.StateFailed,
		Units: []supervisor.UnitSnapshot{{Owner: "kernel", Kind: supervisor.UnitParticipant, State: supervisor.UnitFailed, ExitPolicy: supervisor.ExitGracefulShutdown}},
	}
	diagnostics := composeProcessDiagnostics(kernelState, processState)
	kernelState.ConfigProvenance[0] = secret
	kernelState.Ownerships[0].LastErrorType = secret
	processState.Units[0].Owner = secret
	if diagnostics.ConfigProvenance[0] != "config.yaml" || diagnostics.Responsibilities[0].LastErrorType != "*errors.errorString" || diagnostics.Responsibilities[1].Owner != "kernel" {
		t.Fatalf("diagnostics changed with inputs: %#v", diagnostics)
	}
	if strings.Contains(fmt.Sprintf("%#v", diagnostics), secret) {
		t.Fatalf("diagnostics leaked secret: %#v", diagnostics)
	}
}

func TestNilHostReturnsTypedFailedDiagnostics(t *testing.T) {
	var host *Host
	diagnostics := host.Diagnostics()
	if diagnostics.ProcessState != supervisor.StateFailed || diagnostics.KernelState != LifecycleFailed || diagnostics.ProcessErrorType != "" {
		t.Fatalf("Diagnostics() = %#v", diagnostics)
	}
}
