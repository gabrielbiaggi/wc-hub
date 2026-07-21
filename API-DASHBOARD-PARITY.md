# Paridade API ↔ Dashboard

Atualizado em 21/07/2026 a partir das rotas registradas em `back/internal` e
das telas realmente roteadas pelo Vue. A regra é simples: toda operação que um
usuário autenticado pode solicitar pela API possui uma entrada equivalente no
dashboard, com o mesmo RBAC, CSRF, auditoria e — quando crítica — confirmação
forte/TOTP.

## Resultado

| Escopo | Situação | Tela ou consumidor |
|---|---|---|
| Autenticação, sessão e TOTP | Completo | `LoginView`, `SettingsView`, store de auth |
| Administração de usuários, papéis e permissões | Completo | `AdminView` |
| Inventário, hosts e integrações | Completo | `InventoryView`, `IntegrationsView` |
| Alertas, auditoria, overview e catálogo de módulos | Completo | `NotificationsView`, `AuditView`, `OverviewView`, `OperationsView` |
| Proxmox | Completo | `ProxmoxView` |
| Docker | Completo | `views/docker/DockerView` |
| Kubernetes | Completo | `views/kubernetes/KubernetesView` |
| OCI | Completo | `views/oci/OracleCloudView` |
| GitHub | Completo | `views/github/GitHubView` |
| Cloudflare | Completo | `views/cloudflare/CloudflareView` |
| Terraform | Completo | `views/terraform/TerraformView` |
| Storage, backup, energia, VNC e uptime | Completo | respectivas telas operacionais |
| Jobs, telemetria e terminal | Completo | `JobsView`, `TelemetryView`, `TerminalView` |
| Provisionamento de token de agente | Completo | `InventoryView` |

## Matriz de operações

| Provider | Leituras expostas | Operações expostas no dashboard |
|---|---|---|
| Proxmox | resumo, inventário, rede, snapshots, firewall, backups | sync, start/stop/shutdown/reboot/reset, criar QEMU/LXC, clone, delete, snapshot/rollback, migrate, resize, config, regras firewall, backup |
| Docker | health, inventário, containers, imagens, stats | start, stop, restart, kill, remove, exec |
| Kubernetes | nodes, deployments, pods, eventos, logs | scale, restart, delete deployment, exec pod |
| OCI | regiões, tenancy, compute e Autonomous DB | start/stop/reset/terminate, launch instance, criar ADB |
| GitHub | overview, commits, workflows, arquivo de workflow | cancel/rerun de run, dispatch/enable/disable workflow, editar workflow |
| Cloudflare | health, tunnels, DNS, rotas privadas, settings, rulesets | criar/editar/remover tunnel, configurar ingress, criar DNS/rota privada, atualizar settings, purge cache |
| Terraform | runs e outputs | validate, plan, apply, destroy, output |
| Storage | browse, index, stream | mkdir, upload, rename, delete |
| Monitor | targets e webhook | create, update, delete target; configurar webhook |
| Core | hosts, integrações, alertas, RBAC, jobs, policy, sessões | create host/integration, alert status, RBAC CRUD, enqueue allowlisted job, avaliar policy, emitir token de agente, emitir ticket SSH |

## Proteções invariantes

- Docker `stop`, `restart`, `kill`, `remove` e `exec` exigem confirmação exata
  e TOTP no dashboard e no backend.
- Kubernetes `scale`, `restart`, `delete` e `exec` seguem o mesmo guard.
- Proxmox `stop`, `shutdown`, `reboot`, `reset`, delete, rollback, resize,
  alteração de configuração e migração passam pelo policy engine; ações sobre
  uma VM self-protected são negadas pelo servidor mesmo com TOTP válido.
- Terraform destroy/apply, terminal remoto, tokens de agentes e as demais
  mutações continuam auditados.

## Endpoints que deliberadamente não recebem botão no navegador

| Endpoint | Motivo |
|---|---|
| `GET /healthz` | Probe de infraestrutura, exibido por monitoramento externo; não é uma operação de usuário. |
| `POST /agent/v1/metrics` | Ingestão autenticada agente → Hub; o dashboard apresenta a telemetria em `TelemetryView`. |
| `POST /agent/v1/events` | Ingestão autenticada agente → Hub; os eventos aparecem na auditoria. |

Esses endpoints não são capacidades administrativas e expô-los como botões no
browser quebraria a separação agente/controle. O endpoint administrativo de
provisionamento do token do agente, por outro lado, está disponível no
inventário com confirmação forte e revelação única do token.

## Verificação automatizada

```bash
python3 scripts/check_openapi_coverage.py
```

O verificador compara as rotas HTTP públicas do backend com `openapi.yaml`.
`GET /healthz` é incluído na especificação e tratado como probe intencional.
