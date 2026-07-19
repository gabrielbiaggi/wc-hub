package app

import "strings"

func developmentMasterAllowed(environment string) bool {
	switch strings.ToLower(strings.TrimSpace(environment)) {
	case "development", "local", "test":
		return true
	default:
		return false
	}
}
