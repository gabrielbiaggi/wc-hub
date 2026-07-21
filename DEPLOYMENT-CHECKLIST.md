# WC Hub - Checklist de Deploy para Produção

## ✅ Pré-Deploy - Verificações de Código

### Backend
- [x] Backend compila sem erros: `go build ./...`
- [x] Sem rotas duplicadas
- [x] Self-protection integrado em operações Proxmox críticas
- [x] Audit logs completos em operações Proxmox
- [x] Security engine testado
- [x] Testes existentes passando
- [ ] **Vulnerabilidades de dependências resolvidas** ⚠️ VERIFICAR DEPENDABOT

### Frontend
- [ ] Frontend builda sem erros: `npm run build`
- [ ] Sem vulnerabilidades críticas: `npm audit`
- [ ] Bundle size aceitável

### Infraestrutura
- [x] Docker Compose configurado
- [x] Dockerfile multi-stage otimizado
- [x] Health checks configurados
- [x] Migrations database
- [x] Read-only containers onde possível
- [x] Security opts (no-new-privileges)

## ✅ Pré-Deploy - Configuração

### Variáveis de Ambiente Obrigatórias
```bash
# Database
[ ] WC_HUB_DATABASE_URL
[ ] POSTGRES_PASSWORD

# Auth & Security
[ ] WC_HUB_ENCRYPTION_KEY (32+ chars random)
[ ] WC_HUB_SELF_PROTECTED=true
[ ] WC_HUB_SECURE_COOKIES=true (se HTTPS)
[ ] WC_HUB_SESSION_TTL=12h

# Integrações (se usadas)
[ ] PROXMOX_API_URL
[ ] PROXMOX_API_TOKEN_ID
[ ] PROXMOX_API_TOKEN_SECRET
[ ] DOCKER_PROXY_URL (opcional)
[ ] KUBERNETES_API_URL (opcional)
[ ] GITHUB_TOKEN (opcional)
```

### Secrets Montados
```bash
[ ] /run/secrets/proxmox_ca.pem
[ ] /run/secrets/docker_ca.pem (se Docker remoto)
[ ] /run/secrets/kubernetes_ca.pem (se K8s)
[ ] /run/secrets/ssh_private_key (se SSH)
[ ] /run/secrets/ssh_known_hosts (se SSH)
```

### Permissões e Allowlists
```bash
[ ] WC_HUB_LOCAL_TARGET_NAME configurado
[ ] WC_HUB_LOCAL_COMMAND_ALLOWLIST definido
[ ] PROXMOX_ADDITIONAL_CONFIG_PATHS (se múltiplos clusters)
[ ] Allowlists específicas configuradas (GitHub repos, CF zones, etc)
```

## ✅ Deploy - Execução

### Passo 1: Backup
```bash
[ ] Backup do banco de dados PostgreSQL
[ ] Backup de secrets e configs
[ ] Backup de volumes persistentes
[ ] Anotar commit atual: git rev-parse HEAD
```

### Passo 2: Database Migrations
```bash
cd /home/bill/dev/wc-hub
[ ] docker compose run --rm migrate
[ ] Verificar migrations aplicadas com sucesso
[ ] Verificar logs: docker compose logs migrate
```

### Passo 3: Deploy da Aplicação
```bash
[ ] docker compose build --no-cache
[ ] docker compose up -d
[ ] Aguardar health checks: docker compose ps
[ ] Verificar logs: docker compose logs -f back
```

### Passo 4: Smoke Tests
```bash
# Health check
[ ] curl http://localhost:8088/healthz
[ ] Resposta: {"status":"ok","self_protected":true}

# Login UI
[ ] Abrir http://localhost:8088
[ ] Fazer login com credenciais admin
[ ] Verificar dashboard carrega

# Bootstrap (se primeira instalação)
[ ] POST /api/v1/auth/bootstrap
[ ] Guardar credenciais admin geradas
```

## ✅ Pós-Deploy - Validação

### Funcionalidade Básica
```bash
[ ] Login funciona
[ ] Dashboard carrega
[ ] Proxmox inventory aparece
[ ] Métricas são coletadas
[ ] Logs são escritos
```

### Self-Protection e Audit
```bash
# Testar self-protection
[ ] Tentar ação destrutiva sem X-Confirmation → deve bloquear
[ ] Tentar ação destrutiva sem X-TOTP-Code → deve bloquear
[ ] Ação com confirmação correta + TOTP → deve permitir

# Verificar audit logs
[ ] GET /api/v1/audit
[ ] Ver decisões "allowed" e "denied"
[ ] Ver risk levels corretos (safe, dangerous, critical)
```

### Operações Críticas (Proxmox)
```bash
[ ] Listar VMs/LXCs
[ ] Criar snapshot (deve auditar)
[ ] Deletar snapshot (deve exigir confirmação + TOTP)
[ ] Power action: start VM (permitido sem confirmação)
[ ] Power action: shutdown VM (deve exigir confirmação + TOTP)
[ ] Deletar VM (deve exigir confirmação + TOTP)
```

### RBAC
```bash
# Com usuário sem permissões
[ ] GET /api/v1/overview → 403
[ ] POST /api/v1/proxmox/sync → 403

# Com usuário com permissões
[ ] GET /api/v1/overview → 200
[ ] POST /api/v1/proxmox/sync → 202
```

### Integrations (se configuradas)
```bash
[ ] Docker: listar containers
[ ] Kubernetes: listar pods
[ ] GitHub: listar repos
[ ] Cloudflare: listar zones
[ ] Terraform: listar workspaces
```

## ✅ Pós-Deploy - Monitoramento

### Logs (primeiros 30min)
```bash
[ ] Monitorar: docker compose logs -f back
[ ] Sem erros críticos
[ ] Audit logs sendo gerados
[ ] Health checks passando
```

### Métricas
```bash
[ ] Prometheus metrics: http://localhost:8080/metrics (se exposto)
[ ] Node exporter: http://node-exporter:9100/metrics
[ ] DCGM exporter: (se GPU presente)
```

### Database
```bash
[ ] Connection pool saudável
[ ] Queries não lentas (< 100ms para reads)
[ ] Sem connection leaks
```

### Recursos
```bash
[ ] CPU < 50% em idle
[ ] Memória estável (sem memory leak)
[ ] Disk I/O aceitável
[ ] Network sem erros
```

## ⚠️ Rollback Plan

### Se algo der errado:

```bash
cd /home/bill/dev/wc-hub

# 1. Parar aplicação
docker compose down

# 2. Voltar código
git checkout <commit-anterior-estavel>
# OU
git revert HEAD  # se preferir

# 3. Restaurar database (SE migrations falharam)
# Conectar ao PostgreSQL e restore do backup
docker exec -i wc-hub-postgres-1 psql -U wc_hub -d wc_hub < backup.sql

# 4. Rebuild e restart
docker compose build --no-cache
docker compose up -d

# 5. Verificar
docker compose ps
docker compose logs -f back
curl http://localhost:8088/healthz
```

### Commits Estáveis Conhecidos
- `758595f` - P0.1-P0.4 concluídos (ATUAL)
- `c06076d` - Antes das correções P0 (ESTÁVEL ANTERIOR)

## 📊 Métricas de Sucesso

### Deploy é considerado bem-sucedido se:
- [x] Health check responde OK
- [x] Login funciona
- [x] Dashboard carrega
- [x] Self-protection bloqueia ações sem confirmação
- [x] Audit logs são gerados
- [x] Sem erros críticos nos logs por 30min
- [x] RBAC funciona corretamente
- [x] Operações Proxmox funcionam end-to-end

### Deploy deve ser revertido se:
- [ ] Health check falha consistentemente
- [ ] Erros críticos nos logs
- [ ] Database migrations falham
- [ ] Self-protection não funciona
- [ ] Memory leak detectado
- [ ] Vulnerabilidades críticas não resolvidas

## 📞 Contatos de Emergência

### Time
- DevOps: [definir]
- Backend: [definir]
- Frontend: [definir]
- Infra Proxmox: [definir]

### Escalation
1. Verificar logs e métricas
2. Tentar rollback
3. Se rollback falha, contatar DevOps lead
4. Se crítico, contatar time de segurança

## 📝 Notas de Produção

### Após Deploy Bem-Sucedido
1. Atualizar documentação de versão
2. Comunicar time sobre mudanças
3. Agendar review de audit logs (próxima semana)
4. Planejar próximo sprint (P0.3b - plugins)

### Observações Conhecidas
- Plugins (Docker/K8s/Terraform) ainda dependem de validação browser-side
- Dependabot reportou 2 vulnerabilidades (verificar severity)
- Frontend pode ter features que só funcionam em API ou só em Dashboard
- TOTP enrollment pode falhar se system clock dessincronizado

---

**Última atualização:** 2026-07-20
**Versão do checklist:** 1.0
**Commit de referência:** 758595f
