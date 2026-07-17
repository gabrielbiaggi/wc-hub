package application

import (
	"context"
	jobs "github.com/webcreations/wc-hub/back/internal/jobs/domain"
	repo "github.com/webcreations/wc-hub/back/internal/telemetry/repository"
)

type MaintenanceHandler struct{ repo *repo.Postgres }

func NewMaintenanceHandler(repository *repo.Postgres) *MaintenanceHandler {
	return &MaintenanceHandler{repo: repository}
}
func (h *MaintenanceHandler) Kind() string { return "telemetry.maintenance" }
func (h *MaintenanceHandler) Handle(ctx context.Context, job jobs.Job) error {
	return h.repo.Maintenance(ctx)
}
