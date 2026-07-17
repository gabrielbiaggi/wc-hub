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
