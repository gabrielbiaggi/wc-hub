package domain

import (
	"path/filepath"
	"strings"
)

type Scope string

const (
	ScopeLocal  Scope = "local"
	ScopeRemote Scope = "remote"
	ScopeCloud  Scope = "cloud"
)

type Risk string

const (
	RiskSafe      Risk = "safe"
	RiskDangerous Risk = "dangerous"
	RiskCritical  Risk = "critical"
)

type ActionRequest struct {
	Action              string `json:"action"`
	Command             string `json:"command,omitempty"`
	Scope               Scope  `json:"scope"`
	TargetName          string `json:"target_name"`
	TargetSelfProtected bool   `json:"target_self_protected"`
	Confirmation        string `json:"confirmation,omitempty"`
	TOTPVerified        bool   `json:"totp_verified"`
}

type Decision struct {
	Allowed              bool   `json:"allowed"`
	Risk                 Risk   `json:"risk"`
	Reason               string `json:"reason"`
	RequiresConfirmation bool   `json:"requires_confirmation"`
	RequiresTOTP         bool   `json:"requires_totp"`
}

type Engine struct{ allowlist map[string]struct{} }

func NewEngine(commands []string) *Engine {
	allowed := make(map[string]struct{}, len(commands))
	for _, command := range commands {
		allowed[command] = struct{}{}
	}
	return &Engine{allowlist: allowed}
}

var destructiveActions = map[string]struct{}{
	"terminate": {}, "shutdown": {}, "destroy": {}, "reboot": {}, "poweroff": {},
	"delete_host": {}, "delete_vm": {}, "terraform_destroy": {}, "wipe_disk": {},
}

var destructiveCommands = map[string]struct{}{
	"rm": {}, "shutdown": {}, "reboot": {}, "poweroff": {}, "halt": {}, "mkfs": {}, "dd": {},
}

func (e *Engine) Evaluate(req ActionRequest) Decision {
	action := strings.ToLower(strings.TrimSpace(req.Action))
	_, destructiveAction := destructiveActions[action]
	command := filepath.Base(firstToken(req.Command))
	_, destructiveCommand := destructiveCommands[command]

	if req.Scope == ScopeLocal && (req.TargetSelfProtected || destructiveAction || destructiveCommand) {
		if destructiveAction || destructiveCommand {
			return Decision{Risk: RiskCritical, Reason: "blocked: destructive operations can never target the local self-protected host"}
		}
	}
	if req.Scope == ScopeLocal && command != "" {
		if _, ok := e.allowlist[command]; !ok {
			return Decision{Risk: RiskCritical, Reason: "blocked: command is not in the local executor allowlist"}
		}
	}
	if destructiveAction || destructiveCommand {
		if req.Confirmation != req.TargetName {
			return Decision{Risk: RiskCritical, Reason: "strong confirmation does not match the target name", RequiresConfirmation: true, RequiresTOTP: true}
		}
		if !req.TOTPVerified {
			return Decision{Risk: RiskCritical, Reason: "a verified TOTP is required", RequiresConfirmation: true, RequiresTOTP: true}
		}
		return Decision{Allowed: true, Risk: RiskCritical, Reason: "critical remote action authorized", RequiresConfirmation: true, RequiresTOTP: true}
	}
	return Decision{Allowed: true, Risk: RiskSafe, Reason: "action satisfies the active policy"}
}

func firstToken(command string) string {
	fields := strings.Fields(command)
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}
