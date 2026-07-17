package domain

import "time"

type Event struct {
	ID           string    `json:"id"`
	ActorID      string    `json:"actor_id"`
	Action       string    `json:"action"`
	ResourceType string    `json:"resource_type"`
	ResourceID   string    `json:"resource_id"`
	Decision     string    `json:"decision"`
	Reason       string    `json:"reason"`
	OccurredAt   time.Time `json:"occurred_at"`
}
