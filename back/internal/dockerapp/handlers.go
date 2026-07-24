package dockerapp

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	dockeradapter "github.com/webcreations/wc-hub/back/internal/adapters/docker"
)

type Reader interface {
	Health(context.Context) (dockeradapter.Health, error)
	Inventory(context.Context) (dockeradapter.Inventory, error)
	ListContainers(context.Context) ([]dockeradapter.Container, error)
	ListImages(context.Context) ([]dockeradapter.Image, error)
	Stats(context.Context, []dockeradapter.Container) ([]dockeradapter.ContainerStats, error)
}

type Controller interface {
	ContainerAction(context.Context, string, string) error
	Exec(context.Context, string, []string) (string, error)
	PullImage(context.Context, string) error
	DeleteImage(context.Context, string) error
	PruneImages(context.Context) error
	CreateVolume(context.Context, string, string) error
	DeleteVolume(context.Context, string) error
	PruneVolumes(context.Context) error
	CreateNetwork(context.Context, string, string) error
	DeleteNetwork(context.Context, string) error
	PruneNetworks(context.Context) error
}

func (h *Handler) Exec(w http.ResponseWriter, r *http.Request) {
	if !h.available(w) {
		return
	}
	controller, ok := h.reader.(Controller)
	if !ok {
		writeError(w, 503, "docker_exec_unavailable", "O terminal Docker não está configurado.")
		return
	}
	var input struct {
		Command []string `json:"command"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		writeError(w, 400, "invalid_request", "O comando Docker é inválido.")
		return
	}

	id := r.PathValue("id")

	// Self-protection: validate exec command
	if h.policyEnforcer != nil && len(input.Command) > 0 {
		if !h.policyEnforcer(w, r, PolicyRequest{
			Action:       "docker_exec",
			Scope:        "remote",
			TargetName:   "docker/container/" + id,
			Confirmation: r.Header.Get("X-Confirmation"),
			TOTPCode:     r.Header.Get("X-TOTP-Code"),
		}) {
			return // enforcer already wrote response
		}
	}

	output, err := controller.Exec(r.Context(), id, input.Command)
	if err != nil {
		writeError(w, 502, "docker_exec_failed", "O Docker rejeitou a execução: "+err.Error())
		return
	}
	writeJSON(w, 200, map[string]string{"output": output})
}

type Handler struct {
	reader         Reader
	policyEnforcer PolicyEnforcer
	initErr        string
}

func NewHandler(reader Reader, policyEnforcer PolicyEnforcer, initErr ...string) *Handler {
	message := ""
	if len(initErr) > 0 {
		message = initErr[0]
	}
	return &Handler{reader: reader, policyEnforcer: policyEnforcer, initErr: message}
}

func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	if !h.available(w) {
		return
	}
	item, err := h.reader.Health(r.Context())
	if err != nil {
		writeError(w, http.StatusBadGateway, "docker_unreachable", "O Docker não respondeu: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (h *Handler) Inventory(w http.ResponseWriter, r *http.Request) {
	if !h.available(w) {
		return
	}
	item, err := h.reader.Inventory(r.Context())
	if err != nil {
		writeError(w, http.StatusBadGateway, "docker_inventory_failed", "O inventário Docker não pôde ser carregado: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (h *Handler) Containers(w http.ResponseWriter, r *http.Request) {
	if !h.available(w) {
		return
	}
	items, err := h.reader.ListContainers(r.Context())
	if err != nil {
		writeError(w, http.StatusBadGateway, "docker_containers_failed", "Os containers Docker não puderam ser carregados: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) Images(w http.ResponseWriter, r *http.Request) {
	if !h.available(w) {
		return
	}
	items, err := h.reader.ListImages(r.Context())
	if err != nil {
		writeError(w, http.StatusBadGateway, "docker_images_failed", "As imagens Docker não puderam ser carregadas: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) Stats(w http.ResponseWriter, r *http.Request) {
	if !h.available(w) {
		return
	}
	containers, err := h.reader.ListContainers(r.Context())
	if err != nil {
		writeError(w, http.StatusBadGateway, "docker_stats_failed", "Os containers necessários para as estatísticas Docker não puderam ser carregados: "+err.Error())
		return
	}
	items, err := h.reader.Stats(r.Context(), containers)
	if err != nil {
		writeError(w, http.StatusBadGateway, "docker_stats_failed", "As estatísticas Docker não puderam ser carregadas: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "warnings": []string{}})
}

func (h *Handler) ContainerAction(w http.ResponseWriter, r *http.Request) {
	if !h.available(w) {
		return
	}
	controller, ok := h.reader.(Controller)
	if !ok {
		writeError(w, http.StatusServiceUnavailable, "docker_control_unavailable", "Docker control is not configured.")
		return
	}
	id, action := r.PathValue("id"), r.PathValue("action")

	// Self-protection: validate destructive actions
	if h.policyEnforcer != nil && isDestructiveAction(action) {
		if !h.policyEnforcer(w, r, PolicyRequest{
			Action:       "docker_" + action,
			Scope:        "remote",
			TargetName:   "docker/container/" + id,
			Confirmation: r.Header.Get("X-Confirmation"),
			TOTPCode:     r.Header.Get("X-TOTP-Code"),
		}) {
			return // enforcer already wrote response
		}
	}

	if err := controller.ContainerAction(r.Context(), id, action); err != nil {
		writeError(w, http.StatusBadGateway, "docker_action_failed", "O Docker rejeitou a ação no container: "+err.Error())
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"container_id": id, "action": action, "status": "accepted"})
}

func isDestructiveAction(action string) bool {
	destructive := map[string]bool{
		"stop": true, "kill": true, "remove": true, "restart": true,
	}
	return destructive[action]
}

func (h *Handler) available(w http.ResponseWriter) bool {
	if h.reader != nil {
		return true
	}
	message := "O adaptador Docker não está configurado."
	if h.initErr != "" {
		message += " " + h.initErr
	}
	writeError(w, http.StatusServiceUnavailable, "docker_unconfigured", message)
	return false
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]any{"error": map[string]string{"code": code, "message": message}})
}

type StreamController interface {
	PullImageStream(ctx context.Context, image string, logWriter func(string)) error
	InspectContainer(ctx context.Context, id string) (*dockeradapter.ContainerInspect, error)
	CloneStack(ctx context.Context, containerID, suffix string) (string, error)
}

func (h *Handler) UpdateStream(w http.ResponseWriter, r *http.Request) {
	if !h.available(w) {
		return
	}
	client, ok := h.reader.(StreamController)
	if !ok {
		writeError(w, http.StatusServiceUnavailable, "docker_stream_unavailable", "Docker streaming client is not configured.")
		return
	}

	id := r.PathValue("id")
	targetImage := r.URL.Query().Get("image")

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "sse_unsupported", "Streaming SSE não é suportado pelo servidor.")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	sendLog := func(text, msgType string) {
		payload, _ := json.Marshal(map[string]string{"text": text, "type": msgType})
		_, _ = w.Write([]byte("event: log\ndata: " + string(payload) + "\n\n"))
		flusher.Flush()
	}

	sendLog("Iniciando processo de auto-update do container "+id+"...", "info")

	inspect, err := client.InspectContainer(r.Context(), id)
	if err != nil {
		sendLog("Falha ao inspecionar container: "+err.Error(), "error")
		return
	}

	if targetImage == "" {
		targetImage = inspect.Config.Image
	}

	oldImage := inspect.Config.Image
	sendLog("Baixando nova imagem: "+targetImage, "info")

	err = client.PullImageStream(r.Context(), targetImage, func(logMsg string) {
		sendLog(logMsg, "info")
	})
	if err != nil {
		sendLog("Erro durante o pull da imagem: "+err.Error(), "error")
		return
	}

	sendLog("Imagem baixada com sucesso. Reiniciando container...", "info")

	if controller, ok := h.reader.(Controller); ok {
		if err := controller.ContainerAction(r.Context(), id, "restart"); err != nil {
			sendLog("Erro ao reiniciar container: "+err.Error(), "error")
			return
		}
	}

	// Healthcheck verification phase: confirm the container is running after restart.
	healthOK := false
	for i := 0; i < 5; i++ {
		time.Sleep(2 * time.Second)
		if ins, err := client.InspectContainer(r.Context(), id); err == nil {
			if ins.State.Running {
				healthOK = true
				break
			}
		}
	}

	if !healthOK {
		sendLog("ALERTA: Healthcheck falhou nos primeiros 10s. Executando ROLLBACK AUTOMÁTICO para "+oldImage+"...", "warning")
		if err := client.PullImageStream(r.Context(), oldImage, nil); err == nil {
			if controller, ok := h.reader.(Controller); ok {
				_ = controller.ContainerAction(r.Context(), id, "restart")
			}
			sendLog("Rollback concluído com sucesso. Container restaurado para a versão anterior.", "warning")
		} else {
			sendLog("CRÍTICO: Falha no rollback automático: "+err.Error(), "error")
		}
		return
	}

	sendLog("Auto-update concluído com sucesso! Container saudável.", "success")
	_, _ = w.Write([]byte("event: complete\ndata: {\"status\":\"ok\"}\n\n"))
	flusher.Flush()
}

func (h *Handler) CloneStack(w http.ResponseWriter, r *http.Request) {
	if !h.available(w) {
		return
	}
	client, ok := h.reader.(StreamController)
	if !ok {
		writeError(w, http.StatusServiceUnavailable, "docker_clone_unavailable", "Docker clone client is not configured.")
		return
	}

	var req struct {
		ContainerID string `json:"container_id"`
		Suffix      string `json:"suffix"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.ContainerID) == "" {
		writeError(w, http.StatusBadRequest, "invalid_payload", "Payload inválido. container_id é obrigatório.")
		return
	}

	if req.Suffix == "" {
		req.Suffix = "staging"
	}

	newContainerName, err := client.CloneStack(r.Context(), req.ContainerID, req.Suffix)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "clone_failed", "Falha ao clonar stack: "+err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"status":          "created",
		"cloned_from":     req.ContainerID,
		"new_stack_name":  newContainerName,
		"environment":     req.Suffix,
	})
}

func (h *Handler) PullImage(w http.ResponseWriter, r *http.Request) {
	if !h.available(w) {
		return
	}
	controller, ok := h.reader.(Controller)
	if !ok {
		writeError(w, 503, "docker_unavailable", "Docker client not configured.")
		return
	}
	var input struct {
		Image string `json:"image"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil || strings.TrimSpace(input.Image) == "" {
		writeError(w, 400, "invalid_request", "Nome da imagem é obrigatório.")
		return
	}
	if err := controller.PullImage(r.Context(), input.Image); err != nil {
		writeError(w, 502, "pull_failed", err.Error())
		return
	}
	writeJSON(w, 200, map[string]string{"status": "pulled", "image": input.Image})
}

func (h *Handler) DeleteImage(w http.ResponseWriter, r *http.Request) {
	if !h.available(w) {
		return
	}
	controller, ok := h.reader.(Controller)
	if !ok {
		writeError(w, 503, "docker_unavailable", "Docker client not configured.")
		return
	}
	id := r.PathValue("id")
	if err := controller.DeleteImage(r.Context(), id); err != nil {
		writeError(w, 502, "delete_image_failed", err.Error())
		return
	}
	writeJSON(w, 200, map[string]string{"status": "deleted", "image_id": id})
}

func (h *Handler) PruneImages(w http.ResponseWriter, r *http.Request) {
	if !h.available(w) {
		return
	}
	controller, ok := h.reader.(Controller)
	if !ok {
		writeError(w, 503, "docker_unavailable", "Docker client not configured.")
		return
	}
	if err := controller.PruneImages(r.Context()); err != nil {
		writeError(w, 502, "prune_images_failed", err.Error())
		return
	}
	writeJSON(w, 200, map[string]string{"status": "pruned"})
}

func (h *Handler) CreateVolume(w http.ResponseWriter, r *http.Request) {
	if !h.available(w) {
		return
	}
	controller, ok := h.reader.(Controller)
	if !ok {
		writeError(w, 503, "docker_unavailable", "Docker client not configured.")
		return
	}
	var input struct {
		Name   string `json:"name"`
		Driver string `json:"driver"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil || strings.TrimSpace(input.Name) == "" {
		writeError(w, 400, "invalid_request", "Nome do volume é obrigatório.")
		return
	}
	if err := controller.CreateVolume(r.Context(), input.Name, input.Driver); err != nil {
		writeError(w, 502, "create_volume_failed", err.Error())
		return
	}
	writeJSON(w, 201, map[string]string{"status": "created", "name": input.Name})
}

func (h *Handler) DeleteVolume(w http.ResponseWriter, r *http.Request) {
	if !h.available(w) {
		return
	}
	controller, ok := h.reader.(Controller)
	if !ok {
		writeError(w, 503, "docker_unavailable", "Docker client not configured.")
		return
	}
	name := r.PathValue("name")
	if err := controller.DeleteVolume(r.Context(), name); err != nil {
		writeError(w, 502, "delete_volume_failed", err.Error())
		return
	}
	writeJSON(w, 200, map[string]string{"status": "deleted", "name": name})
}

func (h *Handler) PruneVolumes(w http.ResponseWriter, r *http.Request) {
	if !h.available(w) {
		return
	}
	controller, ok := h.reader.(Controller)
	if !ok {
		writeError(w, 503, "docker_unavailable", "Docker client not configured.")
		return
	}
	if err := controller.PruneVolumes(r.Context()); err != nil {
		writeError(w, 502, "prune_volumes_failed", err.Error())
		return
	}
	writeJSON(w, 200, map[string]string{"status": "pruned"})
}

func (h *Handler) CreateNetwork(w http.ResponseWriter, r *http.Request) {
	if !h.available(w) {
		return
	}
	controller, ok := h.reader.(Controller)
	if !ok {
		writeError(w, 503, "docker_unavailable", "Docker client not configured.")
		return
	}
	var input struct {
		Name   string `json:"name"`
		Driver string `json:"driver"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil || strings.TrimSpace(input.Name) == "" {
		writeError(w, 400, "invalid_request", "Nome da rede é obrigatório.")
		return
	}
	if err := controller.CreateNetwork(r.Context(), input.Name, input.Driver); err != nil {
		writeError(w, 502, "create_network_failed", err.Error())
		return
	}
	writeJSON(w, 201, map[string]string{"status": "created", "name": input.Name})
}

func (h *Handler) DeleteNetwork(w http.ResponseWriter, r *http.Request) {
	if !h.available(w) {
		return
	}
	controller, ok := h.reader.(Controller)
	if !ok {
		writeError(w, 503, "docker_unavailable", "Docker client not configured.")
		return
	}
	id := r.PathValue("id")
	if err := controller.DeleteNetwork(r.Context(), id); err != nil {
		writeError(w, 502, "delete_network_failed", err.Error())
		return
	}
	writeJSON(w, 200, map[string]string{"status": "deleted", "network_id": id})
}

func (h *Handler) PruneNetworks(w http.ResponseWriter, r *http.Request) {
	if !h.available(w) {
		return
	}
	controller, ok := h.reader.(Controller)
	if !ok {
		writeError(w, 503, "docker_unavailable", "Docker client not configured.")
		return
	}
	if err := controller.PruneNetworks(r.Context()); err != nil {
		writeError(w, 502, "prune_networks_failed", err.Error())
		return
	}
	writeJSON(w, 200, map[string]string{"status": "pruned"})
}

