package domain

import "testing"

func TestSelfProtectedHostCannotBeDestroyed(t *testing.T) {
	engine := NewEngine([]string{"uptime"})
	decision := engine.Evaluate(ActionRequest{Action: "shutdown", Scope: ScopeLocal, TargetName: "wc-hub-local", TargetSelfProtected: true, Confirmation: "wc-hub-local", TOTPVerified: true})
	if decision.Allowed {
		t.Fatal("self-protected target must never allow shutdown")
	}
}

func TestLocalExecutorUsesAllowlist(t *testing.T) {
	engine := NewEngine([]string{"uptime"})
	if engine.Evaluate(ActionRequest{Action: "execute", Command: "curl example.com", Scope: ScopeLocal}).Allowed {
		t.Fatal("non-allowlisted binary was allowed")
	}
	if !engine.Evaluate(ActionRequest{Action: "execute", Command: "uptime", Scope: ScopeLocal}).Allowed {
		t.Fatal("allowlisted command was blocked")
	}
}

func TestRemoteCriticalActionNeedsStrongConfirmation(t *testing.T) {
	engine := NewEngine(nil)
	req := ActionRequest{Action: "destroy", Scope: ScopeCloud, TargetName: "vm-prod", Confirmation: "vm-prod", TOTPVerified: true}
	if !engine.Evaluate(req).Allowed {
		t.Fatal("fully confirmed remote action should be authorized")
	}
}

func TestSnapshotActionsAreDestructive(t *testing.T) {
	engine := NewEngine(nil)

	// Delete snapshot sem confirmação deve bloquear
	req := ActionRequest{Action: "delete_snapshot", Scope: ScopeRemote, TargetName: "snap-001"}
	decision := engine.Evaluate(req)
	if decision.Allowed {
		t.Fatal("delete_snapshot without confirmation should be blocked")
	}
	if !decision.RequiresConfirmation {
		t.Fatal("delete_snapshot should require confirmation")
	}

	// Rollback snapshot sem confirmação deve bloquear
	req = ActionRequest{Action: "rollback_snapshot", Scope: ScopeRemote, TargetName: "snap-001"}
	decision = engine.Evaluate(req)
	if decision.Allowed {
		t.Fatal("rollback_snapshot without confirmation should be blocked")
	}

	// Com confirmação + TOTP deve permitir
	req = ActionRequest{Action: "delete_snapshot", Scope: ScopeRemote, TargetName: "snap-001", Confirmation: "snap-001", TOTPVerified: true}
	decision = engine.Evaluate(req)
	if !decision.Allowed {
		t.Fatal("delete_snapshot with confirmation+TOTP should be allowed")
	}
}

func TestStopActionIsDestructive(t *testing.T) {
	engine := NewEngine(nil)

	// Stop sem confirmação deve bloquear
	req := ActionRequest{Action: "stop", Scope: ScopeRemote, TargetName: "vm-prod"}
	decision := engine.Evaluate(req)
	if decision.Allowed {
		t.Fatal("stop without confirmation should be blocked")
	}

	// Com confirmação + TOTP deve permitir
	req = ActionRequest{Action: "stop", Scope: ScopeRemote, TargetName: "vm-prod", Confirmation: "vm-prod", TOTPVerified: true}
	decision = engine.Evaluate(req)
	if !decision.Allowed {
		t.Fatal("stop with confirmation+TOTP should be allowed")
	}
}
