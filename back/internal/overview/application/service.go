package application

import "time"

type Metric struct {
	Label  string  `json:"label"`
	Value  float64 `json:"value"`
	Unit   string  `json:"unit"`
	Delta  float64 `json:"delta"`
	Status string  `json:"status"`
}
type Activity struct {
	ID       string    `json:"id"`
	Source   string    `json:"source"`
	Message  string    `json:"message"`
	Severity string    `json:"severity"`
	At       time.Time `json:"at"`
}
type Snapshot struct {
	GeneratedAt   time.Time  `json:"generated_at"`
	Environment   string     `json:"environment"`
	SelfProtected bool       `json:"self_protected"`
	Metrics       []Metric   `json:"metrics"`
	Activity      []Activity `json:"activity"`
	Series        []float64  `json:"series"`
}

type Service struct {
	environment   string
	selfProtected bool
}

func New(environment string, selfProtected bool) *Service {
	return &Service{environment: environment, selfProtected: selfProtected}
}
func (s *Service) Snapshot() Snapshot {
	now := time.Now().UTC()
	return Snapshot{GeneratedAt: now, Environment: s.environment, SelfProtected: s.selfProtected,
		Metrics:  []Metric{{"Compute nodes", 8, "online", 1.2, "healthy"}, {"Running workloads", 47, "active", 4.8, "healthy"}, {"Storage pool", 68, "% used", 2.1, "warning"}, {"Open alerts", 3, "signals", -12, "warning"}},
		Activity: []Activity{{"evt-1", "policy", "Self-protection policy loaded", "success", now.Add(-2 * time.Minute)}, {"evt-2", "telemetry", "Metrics collectors awaiting adapters", "info", now.Add(-8 * time.Minute)}, {"evt-3", "control-plane", "WC Hub API is operational", "success", now.Add(-15 * time.Minute)}},
		Series:   []float64{31, 35, 32, 41, 43, 48, 45, 52, 49, 58, 55, 61, 57, 64, 62, 68, 63, 66, 71, 68, 73, 69, 72, 76},
	}
}
