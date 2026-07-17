package domain

import "time"

type User struct {
	ID          string     `json:"id"`
	Email       string     `json:"email"`
	DisplayName string     `json:"display_name"`
	TOTPEnabled bool       `json:"totp_enabled"`
	DisabledAt  *time.Time `json:"disabled_at,omitempty"`
	LastLoginAt *time.Time `json:"last_login_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	Roles       []string   `json:"roles"`
}

type Role struct {
	ID          string   `json:"id"`
	Slug        string   `json:"slug"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Permissions []string `json:"permissions"`
	UserCount   int      `json:"user_count"`
}

type Permission struct {
	ID          string `json:"id"`
	Slug        string `json:"slug"`
	Description string `json:"description"`
	Risk        string `json:"risk"`
}

type Alert struct {
	ID             string     `json:"id"`
	ResourceType   string     `json:"resource_type"`
	ResourceID     *string    `json:"resource_id,omitempty"`
	Severity       string     `json:"severity"`
	Title          string     `json:"title"`
	Description    string     `json:"description"`
	Status         string     `json:"status"`
	AcknowledgedBy *string    `json:"acknowledged_by,omitempty"`
	AcknowledgedAt *time.Time `json:"acknowledged_at,omitempty"`
	ResolvedAt     *time.Time `json:"resolved_at,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
}
