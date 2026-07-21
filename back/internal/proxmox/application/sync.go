package application

import (
	"context"
	"fmt"
	adapter "github.com/webcreations/wc-hub/back/internal/adapters/proxmox"
	jobs "github.com/webcreations/wc-hub/back/internal/jobs/domain"
	repo "github.com/webcreations/wc-hub/back/internal/proxmox/repository"
)

type SyncHandler struct {
	clients []*adapter.Client
	repo    *repo.Postgres
}

func NewSyncHandler(clients []*adapter.Client, repository *repo.Postgres) *SyncHandler {
	return &SyncHandler{clients: clients, repo: repository}
}
func (h *SyncHandler) Kind() string { return "proxmox.sync" }
func (h *SyncHandler) Handle(ctx context.Context, job jobs.Job) error {
	if len(h.clients) == 0 {
		return fmt.Errorf("Proxmox is not configured")
	}
	runID, err := h.repo.StartRun(ctx, job.ID)
	if err != nil {
		return err
	}
	resources := 0
	for _, client := range h.clients {
		if client == nil {
			continue
		}
		snapshot, snapshotErr := client.Snapshot(ctx)
		if snapshotErr != nil {
			_ = h.repo.MarkError(ctx, snapshotErr.Error())
			_ = h.repo.FinishRun(ctx, runID, "failed", resources, snapshotErr)
			return snapshotErr
		}
		synced, syncErr := h.repo.Sync(ctx, snapshot, client.ID())
		resources += synced
		if syncErr != nil {
			_ = h.repo.FinishRun(ctx, runID, "failed", resources, syncErr)
			return syncErr
		}
	}
	return h.repo.FinishRun(ctx, runID, "succeeded", resources, nil)
}
