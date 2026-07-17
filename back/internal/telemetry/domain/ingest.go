package domain

import "time"

type IngestSample struct {
	Name   string            `json:"name"`
	Labels map[string]string `json:"labels"`
	Value  float64           `json:"value"`
	Unit   string            `json:"unit,omitempty"`
}
type Batch struct {
	CapturedAt time.Time      `json:"captured_at"`
	Samples    []IngestSample `json:"samples"`
}
type HostMetric struct {
	HostID     string    `json:"host_id"`
	HostName   string    `json:"host_name"`
	Metric     string    `json:"metric"`
	Value      float64   `json:"value"`
	Unit       string    `json:"unit"`
	CapturedAt time.Time `json:"captured_at"`
}
