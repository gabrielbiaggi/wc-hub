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

### ⚠️ P0.3 - Self-protection não está sendo usado server-side
**Status: Pendente**

**Problema:**
- `security.Engine.Evaluate()` existe e funciona
- Endpoint `/api/v1/security/evaluate` chama o engine (confirmação browser-side)
- **MAS** operações destrutivas no server-side não chamam `Evaluate()` antes de executar

**Operações críticas sem proteção server-side:**
1. **Proxmox:**
   - `proxmoxPowerAction` - shutdown, reboot, etc
   - `proxmoxDeleteGuest` - delete VM/LXC
   - `proxmoxDeleteSnapshot` - delete snapshot
   - `proxmoxRollbackSnapshot` - rollback snapshot
   - `proxmoxMigrateGuest` - migrate VM
   - `proxmoxResizeDisk` - resize disk

2. **Docker** (via plugin dockerapp):
   - `ContainerAction` - stop, restart, kill
   - `Exec` - execute commands

3. **Kubernetes** (via plugin kubernetesapp):
   - `DeploymentAction` - scale, restart
   - `PodExec` - execute commands

4. **Terraform** (via plugin terraformapp):
   - Apply/Destroy operations

5. **Storage/OCI:**
   - Delete operations

**Solução planejada:**
1. Criar método helper `App.enforcePolicy(w, r, ActionRequest) bool`
2. Integrar nas operações críticas ANTES da execução
3. Retornar 403 se policy bloquear
4. Registrar decisão no audit log

**Exemplo de integração:**
```go
func (a *App) proxmoxDeleteGuest(w http.ResponseWriter, r *http.Request) {
    // ... parse request ...
    
    // ANTES de chamar client.DeleteGuest:
    if !a.enforcePolicy(w, r, security.ActionRequest{
        Action: "delete_vm",
        Scope: security.ScopeRemote,
        TargetName: node + "/" + kind + "/" + vmid,
        Confirmation: r.Header.Get("X-Confirmation"),
        TOTPCode: r.Header.Get("X-TOTP-Code"),
    }) {
        return // enforcePolicy já escreveu a resposta
    }
    
    // Agora pode executar a operação
    if err := client.DeleteGuest(...); err != nil {
        // handle error
    }
}
```

**Impacto:**
- CRÍTICO: Sem isso, operações destrutivas podem ser executadas sem confirmação forte
- Browser pode validar, mas usuário malicioso pode chamar API diretamente
- Self-protection flag é ignorado no server-side

---

## Próximos Passos (Ordem de Prioridade)

### P0.3 - Integrar self-protection server-side
1. Implementar `App.enforcePolicy()` helper
2. Integrar em handlers Proxmox críticos
3. Integrar nos plugins (Docker, Kubernetes, Terraform)
4. Testar com e sem TOTP habilitado
5. Verificar audit logs

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

