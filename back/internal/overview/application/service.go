package application

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

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
	db            *pgxpool.Pool
	environment   string
	selfProtected bool
}

func New(db *pgxpool.Pool, environment string, selfProtected bool) *Service {
	return &Service{db: db, environment: environment, selfProtected: selfProtected}
}

func (s *Service) Snapshot(ctx context.Context) (Snapshot, error) {
	result := Snapshot{GeneratedAt: time.Now().UTC(), Environment: s.environment, SelfProtected: s.selfProtected, Metrics: []Metric{}, Activity: []Activity{}, Series: []float64{}}
	var nodes, workloads, samples, alerts int
	err := s.db.QueryRow(ctx, `
		SELECT
		  (SELECT count(*) FROM nodes),
		  (SELECT count(*) FROM virtual_machines WHERE status='online') + (SELECT count(*) FROM containers WHERE status='online'),
		  (SELECT count(*) FROM metrics_snapshots WHERE captured_at > now() - interval '1 hour'),
		  (SELECT count(*) FROM alerts WHERE status='open')`).Scan(&nodes, &workloads, &samples, &alerts)
	if err != nil {
		return result, err
	}
	alertStatus := "healthy"
	if alerts > 0 {
		alertStatus = "warning"
	}
	result.Metrics = []Metric{
		{Label: "Compute nodes", Value: float64(nodes), Unit: "discovered", Status: "healthy"},
		{Label: "Active workloads", Value: float64(workloads), Unit: "online", Status: "healthy"},
		{Label: "Telemetry samples", Value: float64(samples), Unit: "last hour", Status: "healthy"},
		{Label: "Open alerts", Value: float64(alerts), Unit: "signals", Status: alertStatus},
	}
	rows, err := s.db.Query(ctx, `SELECT id::text,action,COALESCE(target_name,resource_type),decision,occurred_at FROM audit_logs ORDER BY occurred_at DESC LIMIT 8`)
	if err != nil {
		return result, err
	}
	for rows.Next() {
		var item Activity
		var action, target, decision string
		if err = rows.Scan(&item.ID, &action, &target, &decision, &item.At); err != nil {
			rows.Close()
			return result, err
		}
		item.Source = action
		item.Message = target
		item.Severity = "info"
		if decision == "allowed" || decision == "succeeded" {
			item.Severity = "success"
		} else if decision == "denied" || decision == "failed" {
			item.Severity = "warning"
		}
		result.Activity = append(result.Activity, item)
	}
	rows.Close()
	seriesRows, err := s.db.Query(ctx, `SELECT avg(value) FROM metrics_snapshots WHERE captured_at > now() - interval '24 hours' GROUP BY date_trunc('hour',captured_at) ORDER BY date_trunc('hour',captured_at)`)
	if err != nil {
		return result, err
	}
	defer seriesRows.Close()
	for seriesRows.Next() {
		var value float64
		if err = seriesRows.Scan(&value); err != nil {
			return result, err
		}
		result.Series = append(result.Series, value)
	}
	return result, seriesRows.Err()
}
