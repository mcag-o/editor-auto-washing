package domain

import "testing"

func TestNewSystemControlStateDefaultsStopped(t *testing.T) {
	state := NewSystemControlState("operator")
	if state.State != SystemStateStopped {
		t.Fatalf("expected stopped, got %s", state.State)
	}
	if state.UpdatedBy != "operator" {
		t.Fatalf("expected updated by to be operator, got %s", state.UpdatedBy)
	}
	if err := state.Validate(); err != nil {
		t.Fatalf("expected new system control state to validate: %v", err)
	}
}

func TestAuditLogValidateRequiresActorAndAction(t *testing.T) {
	entry := AuditLog{}
	if err := entry.Validate(); err == nil {
		t.Fatal("expected validate to reject empty audit log")
	}
}

func TestAuditLogValidateAcceptsRequiredFields(t *testing.T) {
	entry := AuditLog{
		Actor:  "operator",
		Action: "control_plane.started",
	}
	if err := entry.Validate(); err != nil {
		t.Fatalf("expected audit log to validate: %v", err)
	}
}
