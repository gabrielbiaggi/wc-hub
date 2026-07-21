# API ↔ Dashboard Paridade

## Status da Implementação

Documento gerado automaticamente listando todas as rotas da API e seu status correspondente no dashboard Vue.js.

### Legenda

- ✅ **Implementado**: Endpoint tem interface correspondente no dashboard
- ⚠️ **Parcial**: Endpoint existe mas interface incompleta ou limitada
- ❌ **Ausente**: Endpoint não tem interface no dashboard
- 🔧 **Admin-Only**: Endpoints administrativos (sempre via AdminView.vue)

---

## Core API

### Autenticação (`/api/v1/auth/`)

| Método | Rota | Status | Localização Frontend |
|--------|------|--------|---------------------|
| GET | `/auth/bootstrap-status` | ✅ | `LoginView.vue` (bootstrap check) |
| POST | `/auth/bootstrap` | ✅ | `LoginView.vue` (primeira configuração) |
| POST | `/auth/login` | ✅ | `LoginView.vue` (login form) |
| GET | `/auth/session` | ✅ | `stores/auth.ts` (session management) |
| POST | `/auth/logout` | ✅ | Header/Navigation (logout button) |
| POST | `/auth/totp/enroll` | ✅ | `SettingsView.vue` (TOTP setup) |
| POST | `/auth/totp/confirm` | ✅ | `SettingsView.vue` (TOTP verification) |

### Administração (`/api/v1/admin/`)

| Método | Rota | Status | Localização Frontend |
|--------|------|--------|---------------------|
| GET | `/admin/users` | 🔧 | `AdminView.vue` (tab: users) |
| POST | `/admin/users` | 🔧 | `AdminView.vue` (create user dialog) |
| PATCH | `/admin/users/{id}` | 🔧 | `AdminView.vue` (edit user dialog) |
| DELETE | `/admin/users/{id}` | 🔧 | `AdminView.vue` (disable user button) |
| GET | `/admin/roles` | 🔧 | `AdminView.vue` (tab: roles) |
| POST | `/admin/roles` | 🔧 | `AdminView.vue` (create role dialog) |
| PATCH | `/admin/roles/{id}` | 🔧 | `AdminView.vue` (edit role dialog) |
| DELETE | `/admin/roles/{id}` | 🔧 | `AdminView.vue` (delete role button) |
| GET | `/admin/permissions` | 🔧 | `AdminView.vue` (tab: permissions, catalog) |

### Visão Geral & Monitoramento

| Método | Rota | Status | Localização Frontend |
|--------|------|--------|---------------------|
| GET | `/api/v1/overview` | ✅ | `OverviewView.vue` (dashboard principal) |
| GET | `/api/v1/modules` | ✅ | `ModuleView.vue` (módulos disponíveis) |
| GET | `/api/v1/integrations` | ✅ | `IntegrationsView.vue` (lista integrações) |
| POST | `/api/v1/integrations` | ✅ | `IntegrationsView.vue` (criar integração) |
| GET | `/api/v1/hosts` | ✅ | `InventoryView.vue` (hosts inventory) |
| POST | `/api/v1/hosts` | ✅ | `InventoryView.vue` (adicionar host) |
| GET | `/api/v1/alerts` | ✅ | `NotificationsView.vue` (alertas) |
| PATCH | `/api/v1/alerts/{id}` | ✅ | `NotificationsView.vue` (atualizar alerta) |
| GET | `/api/v1/audit` | ✅ | `AuditView.vue` (audit log) |

### Segurança

| Método | Rota | Status | Localização Frontend |
|--------|------|--------|---------------------|
| POST | `/api/v1/security/evaluate` | ⚠️ | Usado internamente para policy enforcement, sem UI direto |

---

## Proxmox Plugin (`/api/v1/proxmox/`)

| Método | Rota | Status | Localização Frontend |
|--------|------|--------|---------------------|
| GET | `/proxmox/summary` | ✅ | `ProxmoxView.vue` (resumo clusters) |
| POST | `/proxmox/sync` | ✅ | `ProxmoxView.vue` (sync button) |
| GET | `/proxmox/inventory` | ✅ | `ProxmoxView.vue` (inventário VMs/LXC) |
| POST | `/proxmox/nodes/{node}/{kind}/{vmid}/{action}` | ✅ | `ProxmoxView.vue` (power actions: start/stop/restart) |
| POST | `/proxmox/qemu` | ✅ | `ProxmoxView.vue` (criar VM) |
| POST | `/proxmox/lxc` | ✅ | `ProxmoxView.vue` (criar container) |
| POST | `/proxmox/clone` | ✅ | `ProxmoxView.vue` (clonar VM/CT) |
| DELETE | `/proxmox/nodes/{node}/{kind}/{vmid}` | ✅ | `ProxmoxView.vue` (deletar guest) |
| GET | `/proxmox/nodes/{node}/{kind}/{vmid}/snapshots` | ✅ | `ProxmoxView.vue` (listar snapshots) |
| POST | `/proxmox/nodes/{node}/{kind}/{vmid}/snapshots` | ✅ | `ProxmoxView.vue` (criar snapshot) |
| POST | `/proxmox/nodes/{node}/{kind}/{vmid}/snapshots/{name}/rollback` | ✅ | `ProxmoxView.vue` (rollback snapshot) |
| DELETE | `/proxmox/nodes/{node}/{kind}/{vmid}/snapshots/{name}` | ✅ | `ProxmoxView.vue` (deletar snapshot) |
| POST | `/proxmox/nodes/{node}/{kind}/{vmid}/migrate` | ✅ | `ProxmoxView.vue` (migrar guest) |
| PUT | `/proxmox/nodes/{node}/{kind}/{vmid}/resize` | ✅ | `ProxmoxView.vue` (resize disk) |
| PUT | `/proxmox/nodes/{node}/{kind}/{vmid}/config` | ✅ | `ProxmoxView.vue` (update config) |
| GET | `/proxmox/nodes/{node}/network` | ✅ | `ProxmoxView.vue` (network info) |
| GET | `/proxmox/nodes/{node}/{kind}/{vmid}/firewall/rules` | ✅ | `ProxmoxView.vue` (firewall rules) |
| POST | `/proxmox/nodes/{node}/{kind}/{vmid}/firewall/rules` | ✅ | `ProxmoxView.vue` (criar regra firewall) |
| DELETE | `/proxmox/nodes/{node}/{kind}/{vmid}/firewall/rules/{pos}` | ✅ | `ProxmoxView.vue` (deletar regra firewall) |
| GET | `/proxmox/nodes/{node}/backups` | ✅ | `ProxmoxView.vue` (backups disponíveis) |
| POST | `/proxmox/nodes/{node}/{kind}/{vmid}/backup` | ✅ | `ProxmoxView.vue` (criar backup) |

---

## Docker Plugin (`/api/v1/docker/`)

| Método | Rota | Status | Localização Frontend |
|--------|------|--------|---------------------|
| GET | `/docker/health` | ✅ | `DockerView.vue` (health check) |
| GET | `/docker/inventory` | ✅ | `DockerView.vue` (inventário) |
| GET | `/docker/containers` | ✅ | `docker/ContainersView.vue` (lista containers) |
| GET | `/docker/images` | ✅ | `docker/ImagesView.vue` (lista imagens) |
| GET | `/docker/stats` | ✅ | `DockerView.vue` (estatísticas) |
| POST | `/docker/containers/{id}/{action}` | ✅ | `docker/ContainersView.vue` (start/stop/restart/kill/remove) |
| POST | `/docker/containers/{id}/exec` | ✅ | `docker/ContainersView.vue` (exec terminal) |

---

## Kubernetes Plugin (`/api/v1/kubernetes/`)

| Método | Rota | Status | Localização Frontend |
|--------|------|--------|---------------------|
| GET | `/kubernetes/overview` | ✅ | `KubernetesView.vue` (overview clusters) |
| POST | `/kubernetes/namespaces/{namespace}/deployments/{name}/{action}` | ✅ | `kubernetes/DeploymentsView.vue` (scale/restart/delete) |
| GET | `/kubernetes/namespaces/{namespace}/pods/{pod}/logs` | ✅ | `kubernetes/PodsView.vue` (pod logs) |
| POST | `/kubernetes/namespaces/{namespace}/pods/{pod}/exec` | ✅ | `kubernetes/PodsView.vue` (pod exec) |

---

## Terraform Plugin (`/api/v1/terraform/`)

| Método | Rota | Status | Localização Frontend |
|--------|------|--------|---------------------|
| GET | `/terraform/runs` | ✅ | `terraform/RunsView.vue` (lista runs) |
| POST | `/terraform/validate` | ✅ | `terraform/RunsView.vue` (validar workspace) |
| POST | `/terraform/plan` | ✅ | `terraform/RunsView.vue` (plan workspace) |
| POST | `/terraform/apply` | ✅ | `terraform/RunsView.vue` (apply workspace) |
| POST | `/terraform/destroy` | ✅ | `terraform/RunsView.vue` (destroy workspace) |
| POST | `/terraform/output` | ✅ | `terraform/RunsView.vue` (output workspace) |

---

## GitHub Plugin (`/api/v1/github/`)

| Método | Rota | Status | Localização Frontend |
|--------|------|--------|---------------------|
| GET | `/github/overview` | ✅ | `GitHubView.vue` (overview repos) |
| POST | `/github/repos/{owner}/{repo}/actions/runs/{run_id}/{action}` | ✅ | `github/ActionsView.vue` (run actions) |
| GET | `/github/repos/{owner}/{repo}/commits` | ✅ | `github/CommitsView.vue` (lista commits) |
| GET | `/github/repos/{owner}/{repo}/commits/{sha}` | ✅ | `github/CommitsView.vue` (detalhes commit) |
| GET | `/github/repos/{owner}/{repo}/workflows` | ✅ | `github/WorkflowsView.vue` (lista workflows) |
| POST | `/github/repos/{owner}/{repo}/workflows/{workflow_id}/{action}` | ✅ | `github/WorkflowsView.vue` (dispatch workflow) |
| GET | `/github/repos/{owner}/{repo}/workflow-file` | ✅ | `github/WorkflowsView.vue` (conteúdo workflow) |

---

## Cloudflare Plugin (`/api/v1/cloudflare/`)

| Método | Rota | Status | Localização Frontend |
|--------|------|--------|---------------------|
| GET | `/cloudflare/overview` | ✅ | `cloudflare/OverviewView.vue` (overview accounts) |
| GET | `/cloudflare/accounts/{account_id}/tunnels` | ✅ | `cloudflare/TunnelsView.vue` (lista tunnels) |
| POST | `/cloudflare/accounts/{account_id}/tunnels` | ✅ | `cloudflare/TunnelsView.vue` (criar tunnel) |
| PATCH | `/cloudflare/accounts/{account_id}/tunnels/{tunnel_id}` | ✅ | `cloudflare/TunnelsView.vue` (atualizar tunnel) |
| GET | `/cloudflare/zones/{zone_id}/dns-records` | ✅ | `cloudflare/DNSView.vue` (lista DNS records) |

---

## Storage Plugin (`/api/v1/storage/`)

| Método | Rota | Status | Localização Frontend |
|--------|------|--------|---------------------|
| GET | `/storage/browse` | ✅ | `storage/BrowseView.vue` (navegar diretórios) |
| GET | `/storage/index` | ✅ | `storage/IndexView.vue` (index completo) |
| GET | `/storage/stream` | ✅ | `storage/BrowseView.vue` (stream arquivo) |
| POST | `/storage/directories` | ✅ | `storage/BrowseView.vue` (criar diretório) |
| POST | `/storage/upload` | ✅ | `storage/BrowseView.vue` (upload arquivo) |
| PATCH | `/storage/entry` | ✅ | `storage/BrowseView.vue` (renomear arquivo/diretório) |
| DELETE | `/storage/entry` | ✅ | `storage/BrowseView.vue` (deletar arquivo/diretório) |

---

## OCI Plugin (`/api/v1/oci/`)

| Método | Rota | Status | Localização Frontend |
|--------|------|--------|---------------------|
| GET | `/oci/overview` | ✅ | `oci/OverviewView.vue` (overview tenancy) |
| POST | `/oci/instances/{action}` | ✅ | `oci/InstancesView.vue` (start/stop/terminate instance) |
| POST | `/oci/instances` | ✅ | `oci/InstancesView.vue` (launch instance) |
| POST | `/oci/autonomous-databases` | ✅ | `oci/DatabasesView.vue` (criar ADB) |

---

## Monitor Plugin (`/api/v1/monitor/`)

| Método | Rota | Status | Localização Frontend |
|--------|------|--------|---------------------|
| GET | `/monitor/targets` | ✅ | `monitor/TargetsView.vue` (lista targets) |
| POST | `/monitor/targets` | ✅ | `monitor/TargetsView.vue` (criar target) |
| PATCH | `/monitor/targets/{id}` | ✅ | `monitor/TargetsView.vue` (atualizar target) |
| DELETE | `/monitor/targets/{id}` | ✅ | `monitor/TargetsView.vue` (deletar target) |
| GET | `/monitor/webhook` | ✅ | `monitor/WebhookView.vue` (webhook config) |
| PUT | `/monitor/webhook` | ✅ | `monitor/WebhookView.vue` (atualizar webhook) |

---

## Backup Plugin (`/api/v1/backups/`)

| Método | Rota | Status | Localização Frontend |
|--------|------|--------|---------------------|
| GET | `/backups/overview` | ✅ | `backup/OverviewView.vue` (overview PBS) |

---

## Power Plugin (`/api/v1/power/`)

| Método | Rota | Status | Localização Frontend |
|--------|------|--------|---------------------|
| GET | `/power/status` | ✅ | `power/StatusView.vue` (status dispositivos) |
| GET | `/power/targets` | ✅ | `power/TargetsView.vue` (lista targets) |
| POST | `/power/wake/{target}` | ✅ | `power/TargetsView.vue` (Wake-on-LAN) |

---

## VNC Plugin (`/api/v1/vnc/`)

| Método | Rota | Status | Localização Frontend |
|--------|------|--------|---------------------|
| GET | `/vnc/targets` | ✅ | `vnc/TargetsView.vue` (lista targets VNC) |

---

## Operations Plugin (`/api/v1/operations/`)

| Método | Rota | Status | Localização Frontend |
|--------|------|--------|---------------------|
| GET | `/operations/catalog` | ✅ | `OperationsView.vue` (catálogo operações) |

---

## Jobs Plugin (`/api/v1/jobs/`)

| Método | Rota | Status | Localização Frontend |
|--------|------|--------|---------------------|
| GET | `/api/v1/jobs` | ✅ | `JobsView.vue` (lista jobs) |
| POST | `/api/v1/jobs` | ✅ | `JobsView.vue` (criar job) |

---

## Telemetry Plugin (`/api/v1/telemetry/`)

| Método | Rota | Status | Localização Frontend |
|--------|------|--------|---------------------|
| GET | `/telemetry/hosts` | ✅ | `TelemetryView.vue` (métricas hosts) |

---

## Terminal Plugin

| Método | Rota | Status | Localização Frontend |
|--------|------|--------|---------------------|
| POST | `/api/v1/terminal/tickets` | ✅ | `TerminalView.vue` (criar ticket SSH) |
| GET | `/api/v1/terminal/sessions` | ✅ | `TerminalView.vue` (lista sessões ativas) |
| GET | `/ws/terminal` | ✅ | `TerminalView.vue` (WebSocket connection) |

---

## Agent Endpoints (Internal)

| Método | Rota | Status | Localização Frontend |
|--------|------|--------|---------------------|
| POST | `/api/v1/agents/hosts/{host_id}/token` | ❌ | Não exposto no dashboard (operação CLI/backend) |
| POST | `/agent/v1/metrics` | ❌ | Endpoint interno para agentes (não UI) |
| POST | `/agent/v1/events` | ❌ | Endpoint interno para agentes (não UI) |

---

## Resumo Geral

| Categoria | Total Endpoints | ✅ Implementados | ⚠️ Parciais | ❌ Ausentes |
|-----------|----------------|-----------------|-----------|------------|
| **Autenticação** | 7 | 7 | 0 | 0 |
| **Administração (RBAC)** | 9 | 9 | 0 | 0 |
| **Core API** | 8 | 7 | 1 | 0 |
| **Proxmox** | 22 | 22 | 0 | 0 |
| **Docker** | 7 | 7 | 0 | 0 |
| **Kubernetes** | 4 | 4 | 0 | 0 |
| **Terraform** | 6 | 6 | 0 | 0 |
| **GitHub** | 7 | 7 | 0 | 0 |
| **Cloudflare** | 5 | 5 | 0 | 0 |
| **Storage** | 7 | 7 | 0 | 0 |
| **OCI** | 4 | 4 | 0 | 0 |
| **Monitor** | 6 | 6 | 0 | 0 |
| **Backup** | 1 | 1 | 0 | 0 |
| **Power** | 3 | 3 | 0 | 0 |
| **VNC** | 1 | 1 | 0 | 0 |
| **Operations** | 1 | 1 | 0 | 0 |
| **Jobs** | 2 | 2 | 0 | 0 |
| **Telemetry** | 1 | 1 | 0 | 0 |
| **Terminal** | 3 | 3 | 0 | 0 |
| **Agent (Internal)** | 3 | 0 | 0 | 3 |
| **TOTAL** | **107** | **103** | **1** | **3** |

### Taxa de Paridade: **96.3%** ✅

---

## Observações

1. **RBAC Visual**: Totalmente implementado em `AdminView.vue` com gestão completa de usuários, papéis e permissões
2. **Self-Protection**: Implementado nos plugins Docker, Kubernetes e Terraform (P2 completo)
3. **Endpoints Internos**: 3 endpoints são exclusivamente para comunicação agent↔server (não requerem UI)
4. **Endpoint Parcial**: `/api/v1/security/evaluate` é usado internamente pelo policy enforcement, sem interface direta necessária

---

**Gerado em**: 2026-07-21  
**Projeto**: WC-Hub Multi-Cloud Control Plane  
**Versão**: P3 (Paridade API/Dashboard + OpenAPI Docs)
