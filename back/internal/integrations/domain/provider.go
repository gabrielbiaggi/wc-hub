package domain

import "context"

type Health struct {
	Name    string `json:"name"`
	Kind    string `json:"kind"`
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}
type Provider interface {
	Name() string
	Health(context.Context) Health
}
type ComputeProvider interface {
	Provider
	ListNodes(context.Context) ([]Node, error)
}
type Node struct {
	ExternalID string  `json:"external_id"`
	Name       string  `json:"name"`
	Status     string  `json:"status"`
	CPU        float64 `json:"cpu"`
	Memory     float64 `json:"memory"`
}
