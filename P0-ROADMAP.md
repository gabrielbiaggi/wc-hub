# WC Hub - Bloqueadores P0

## Status Atual

### ✅ P0.1 - Backend não compila
**Corrigido em commit 2deb63a**

- ✅ Proxmox client: 4 chamadas `request()` com parâmetro nil extra removido
- ✅ Handlers: nomes de campos corrigidos (dockerClient, kubernetesClient, githubClient)
- ✅ Proxmox: `proxmoxClientFor()` método em vez de campo como função
- ✅ Imports não usados removidos

Backend agora compila: `go build ./...` passa sem erros.

### ✅ P0.2 - Rotas duplicadas
**Corrigido em commit 2deb63a**

- ✅ Rotas Docker removidas de handlers globais (mantidas em dockerapp plugin)
- ✅ Rotas Kubernetes removidas de handlers globais (mantidas em kubernetesapp plugin)
- ✅ Rotas GitHub removidas de handlers globais (mantidas em githubapp plugin)
- ✅ Arquivos deletados: handlers_docker.go, handlers_kubernetes.go, handlers_github.go

Plugins oferecem mais funcionalidade que handlers legados.

### ✅ P0.3 - Self-protection integrado server-side (Proxmox)
**Corrigido em commits 207ad12, e7a89cb**

**Implementado:**
- ✅ Método `App.enforcePolicy()` helper
- ✅ Integrado em handlers Proxmox críticos:
  - proxmoxDeleteGuest (delete_vm)
  - proxmoxPowerAction (shutdown/reboot/stop)
  - proxmoxDeleteSnapshot (delete_snapshot)
  - proxmoxRollbackSnapshot (rollback_snapshot)
- ✅ Expandidas ações destrutivas no policy.go
- ✅ Audit log registra decisões (allowed/denied)

**Como funciona:**
- Operações destrutivas requerem X-Confirmation header (must match target name)
- Operações destrutivas requerem X-TOTP-Code header (if TOTP enabled)
- Policy engine valida ANTES da execução
- Usuário malicioso não pode bypass browser

### ✅ P0.4 - Audit logs completos (Proxmox)
**Corrigido em commit e7a89cb**

- ✅ Todas operações Proxmox críticas registram audit
- ✅ Campos: ActorID, Action, Scope, ResourceType, ResourceID
- ✅ TargetName, Risk, Decision, RequestID, SourceIP
- ✅ Payload com detalhes específicos quando relevante

**Operações auditadas:**
- Create/Delete/Rollback snapshots
- Migrate guest
- Resize disk
- Delete guest
- Power actions (shutdown/reboot/stop)

### ⚠️ P0.3b/P0.4b - Plugins (Docker, K8s, Terraform) pendentes
- ⚠️ **Docker** (dockerapp plugin): ContainerAction stop/kill, Exec
- ⚠️ **Kubernetes** (kubernetesapp plugin): DeploymentAction, PodExec
- ⚠️ **Terraform** (terraformapp plugin): Apply/Destroy operations
- ⚠️ **Storage/OCI:** Delete operations

**Como integrar nos plugins:**
Plugins precisam receber enforcePolicy como callback, similar ao audit callback existente.

---

## Próximos Passos (Ordem de Prioridade)

### P0.3b - Integrar self-protection nos plugins restantes
1. Adicionar callback enforcePolicy nos plugins
2. Integrar em dockerapp (stop, kill, exec)
3. Integrar em kubernetesapp (scale down, exec)
4. Integrar em terraformapp (destroy)
5. Testar com e sem TOTP

### P0.4 - Auditoria obrigatória para operações críticas
- Garantir que TODA operação crítica tem audit log
- Verificar campos: ActorID, Action, Scope, Risk, Decision
- Adicionar payload details quando relevante

### P0.5 - RBAC visual no dashboard
- Dashboard deve mostrar permissões do usuário atual
- Botões desabilitados quando usuário não tem permissão
- Mensagem clara: "Você precisa da permissão X"

### P0.6 - Lacunas de paridade API ↔ Dashboard
- Inventariar endpoints da API
- Inventariar features do dashboard
- Documentar o que está disponível só na API ou só no dashboard
- Priorizar features mais críticas

---

## Notas Técnicas

### Security Engine
- **Localização:** `back/internal/security/domain/policy.go`
- **Método principal:** `Engine.Evaluate(ActionRequest) Decision`
- **Destructive actions:** terminate, shutdown, destroy, reboot, poweroff, delete_host, delete_vm, terraform_destroy, wipe_disk
- **Destructive commands:** rm, shutdown, reboot, poweroff, halt, mkfs, dd
- **Regras:**
  - Local + self-protected + destructive → SEMPRE bloqueado
  - Local + command not in allowlist → bloqueado
  - Destructive sem confirmation match → requer confirmação
  - Destructive sem TOTP → requer TOTP
  - Resto → permitido (risk=safe)

### Audit Repository
- **Localização:** `back/internal/audit/repository/postgres.go`
- **Método:** `Append(ctx, Record) error`
- **Campos importantes:**
  - ActorID, Action, Scope, ResourceType, ResourceID
  - TargetName, Risk, Decision, Reason
  - RequestID, SourceIP, Payload

### Config
- **WC_HUB_SELF_PROTECTED:** bool (default true)
- **WC_HUB_LOCAL_TARGET_NAME:** string (default "wc-hub-local")
- **WC_HUB_LOCAL_COMMAND_ALLOWLIST:** comma-separated commands

