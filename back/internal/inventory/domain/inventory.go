package domain

import (
	"context"
	"time"
)

type Integration struct {
	ID            string         `json:"id"`
	Name          string         `json:"name"`
	Provider      string         `json:"provider"`
	Status        string         `json:"status"`
	Config        map[string]any `json:"config"`
	LastCheckedAt *time.Time     `json:"last_checked_at,omitempty"`
	CreatedAt     time.Time      `json:"created_at"`
}
type Host struct {
	ID            string         `json:"id"`
	IntegrationID *string        `json:"integration_id,omitempty"`
	Name          string         `json:"name"`
	Hostname      string         `json:"hostname"`
	Scope         string         `json:"scope"`
	Status        string         `json:"status"`
	SelfProtected bool           `json:"self_protected"`
	Labels        map[string]any `json:"labels"`
	Facts         map[string]any `json:"facts"`
	LastSeenAt    *time.Time     `json:"last_seen_at,omitempty"`
	CreatedAt     time.Time      `json:"created_at"`
}
type Repository interface {
	ListIntegrations(context.Context) ([]Integration, error)
	CreateIntegration(context.Context, Integration, string) (Integration, error)
	UpsertIntegration(ctx context.Context, name, provider, status string, config map[string]any) error
	ListHosts(context.Context) ([]Host, error)
	CreateHost(context.Context, Host, string) (Host, error)
}
