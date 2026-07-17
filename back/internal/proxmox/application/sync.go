package application

import (
	"context"
	"fmt"
	adapter "github.com/webcreations/wc-hub/back/internal/adapters/proxmox"
	jobs "github.com/webcreations/wc-hub/back/internal/jobs/domain"
	repo "github.com/webcreations/wc-hub/back/internal/proxmox/repository"
)

type SyncHandler struct {
	client  *adapter.Client
	repo    *repo.Postgres
	baseURL string
}

func NewSyncHandler(client *adapter.Client, repository *repo.Postgres, baseURL string) *SyncHandler {
	return &SyncHandler{client: client, repo: repository, baseURL: baseURL}
}
func (h *SyncHandler) Kind() string { return "proxmox.sync" }
func (h *SyncHandler) Handle(ctx context.Context, job jobs.Job) error {
	if h.client == nil {
		return fmt.Errorf("Proxmox is not configured")
	}
	runID, err := h.repo.StartRun(ctx, job.ID)
	if err != nil {
		return err
	}
	snapshot, err := h.client.Snapshot(ctx)
	if err != nil {
		_ = h.repo.MarkError(ctx, err.Error())
		_ = h.repo.FinishRun(ctx, runID, "failed", 0, err)
		return err
	}
	resources, err := h.repo.Sync(ctx, snapshot, h.baseURL)
	status := "succeeded"
	if err != nil {
		status = "failed"
	}
	_ = h.repo.FinishRun(ctx, runID, status, resources, err)
	return err
}
