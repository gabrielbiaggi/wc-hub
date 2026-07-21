package app

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	security "github.com/webcreations/wc-hub/back/internal/security/domain"
	target "github.com/webcreations/wc-hub/back/internal/security/target"
)

func TestSecurityE2ESelfTargetAbsoluteBlock(t *testing.T) {
	engine := security.NewEngine([]string{"uptime"})
	resolver := target.NewResolver()

	tests := []struct {
		name       string
		action     string
		targetName string
		isSelf     bool
	}{
		{"Docker stop self container", "docker_stop", "wc-hub", resolver.IsSelfProtectedContainer("wc-hub")},
		{"K8s delete self pod", "k8s_deployment_delete", "wc-hub-api", resolver.IsSelfProtectedPod("wc-hub-api")},
		{"Terraform destroy self workspace", "terraform_destroy", "hub-infrastructure", resolver.IsSelfProtectedWorkspace("hub-infrastructure")},
		{"Proxmox shutdown self VM", "shutdown", "100", resolver.IsSelfProtectedVM(100, "wc-hub")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			decision := engine.Evaluate(security.ActionRequest{
				Action:              tt.action,
				Scope:               security.ScopeRemote,
				TargetName:          tt.targetName,
				TargetSelfProtected: tt.isSelf,
				Confirmation:        tt.targetName,
				TOTPVerified:        true,
				TOTPCode:            "123456",
			})

			if decision.Allowed {
				t.Fatalf("E2E Violation: %s was allowed on a self-protected resource!", tt.name)
			}
			if decision.Risk != security.RiskCritical {
				t.Fatalf("Expected RiskCritical, got %s", decision.Risk)
			}
		})
	}
}

func TestSecurityE2ERemoteDangerousActionWithValidConfirmationAndTOTP(t *testing.T) {
	engine := security.NewEngine(nil)

	decision := engine.Evaluate(security.ActionRequest{
		Action:              "docker_stop",
		Scope:               security.ScopeRemote,
		TargetName:          "redis-cache",
		TargetSelfProtected: false,
		Confirmation:        "redis-cache",
		TOTPVerified:        true,
		TOTPCode:            "654321",
	})

	if !decision.Allowed {
		t.Fatalf("Expected remote dangerous action to be allowed with valid confirmation and TOTP, got denied: %s", decision.Reason)
	}
}

func TestProviderFailureHandlingReturns503(t *testing.T) {
	// Test unconfigured adapter gracefully returns 503 Service Unavailable
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/docker/inventory", nil)

	handler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]string{
				"code":    "docker_unconfigured",
				"message": "Docker adapter is not configured.",
			},
		})
	}

	handler(recorder, request)
	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("Expected 503, got %d", recorder.Code)
	}
	if !bytes.Contains(recorder.Body.Bytes(), []byte("docker_unconfigured")) {
		t.Fatalf("Expected error code docker_unconfigured in body")
	}
}
