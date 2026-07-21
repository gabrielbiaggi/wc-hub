# WC Hub - Próximos Passos Recomendados

## 🎯 Bloqueadores P0 Resolvidos
- ✅ P0.1: Backend compila
- ✅ P0.2: Rotas duplicadas removidas
- ✅ P0.3: Self-protection server-side (Proxmox)
- ✅ P0.4: Audit logs completos (Proxmox)

## 📋 Itens Pendentes por Prioridade

### P1 - Prioridade Alta (Segurança)

#### 1. Dependências com Vulnerabilidades
**Status:** Identificado pelo GitHub Dependabot
- 1 vulnerabilidade HIGH
- 1 vulnerabilidade MODERATE
- **URL:** https://github.com/gabrielbiaggi/wc-hub/security/dependabot

**Ação recomendada:**
```bash
# Verificar vulnerabilidades
cd /home/bill/dev/wc-hub
# Backend (Go)
cd back && go list -m -u all | grep '\[' || echo "No outdated Go deps"
# Frontend (NPM)
cd ../front && npm audit
```

**Prioridade:** ALTA - Resolver antes de deploy em produção

#### 2. Self-protection nos Plugins (P0.3b)
**Status:** Pendente
- ⚠️ Docker: ContainerAction (stop/kill), Exec
- ⚠️ Kubernetes: DeploymentAction, PodExec
- ⚠️ Terraform: Destroy operations

**Complexidade:** MÉDIA - Requer refatoração de assinaturas dos plugins

**Abordagem:**
1. Adicionar callback `enforcePolicy` em `dockerapp.MountRoutes()`
2. Adicionar callback `enforcePolicy` em `kubernetesapp.MountRoutes()`
3. Adicionar callback `enforcePolicy` em `terraformapp.MountRoutes()`
4. Integrar nos handlers críticos de cada plugin

**Prioridade:** MÉDIA - Browser ainda valida, mas API pode ser chamada diretamente

### P2 - Prioridade Média (Usabilidade)

#### 3. RBAC Visual no Dashboard (P0.5)
**Status:** Pendente análise

**Objetivo:**
- Dashboard deve mostrar permissões do usuário atual
- Botões desabilitados quando usuário não tem permissão
- Mensagens claras: "Você precisa da permissão X"

**Investigação necessária:**
1. Verificar como o frontend recebe permissões do usuário
2. Identificar componentes de botões críticos
3. Implementar lógica de disable baseada em permissões
4. Adicionar tooltips explicativos

**Prioridade:** MÉDIA - Melhora UX mas não afeta segurança (RBAC já funciona no backend)

#### 4. Paridade API ↔ Dashboard (P0.6)
**Status:** Pendente inventário

**Tarefas:**
1. Inventariar todos endpoints da API (via routes em app.go)
2. Inventariar todas features do dashboard (via componentes React/Vue)
3. Criar matriz de paridade
4. Identificar lacunas críticas
5. Priorizar implementação

**Prioridade:** BAIXA - Não bloqueia funcionalidade principal

### P3 - Prioridade Baixa (Otimizações)

#### 5. Testes Automatizados
**Status:** Verificar cobertura existente

**Verificar:**
- Testes unitários: `go test ./...`
- Testes E2E: `playwright.config.ts` existe
- Coverage: gerar report

**Prioridade:** BAIXA - Não bloqueia produção, mas recomendado para a validação interna pré-push

#### 6. Documentação de API
**Status:** Verificar se existe OpenAPI/Swagger

**Tarefas:**
- Verificar se há spec OpenAPI
- Se não, considerar adicionar anotações para geração automática
- Documentar endpoints críticos manualmente

**Prioridade:** BAIXA - Nice to have

## 🔍 Análise de Riscos

### Riscos Residuais Após P0

#### Alto
- **Vulnerabilidades de dependências** (Dependabot)
  - Impacto: Potencial exploração de vulnerabilidades conhecidas
  - Mitigação: Atualizar dependências vulneráveis

#### Médio
- **Plugins sem self-protection server-side**
  - Impacto: API pode ser chamada diretamente para operações críticas
  - Mitigação: Browser ainda valida + RBAC ativo
  - Resolver: Implementar P0.3b

#### Baixo
- **Falta RBAC visual no dashboard**
  - Impacto: Usuário pode tentar ação sem permissão (backend bloqueia)
  - Mitigação: Backend já valida, apenas confusão de UX
  
- **Paridade API/Dashboard incompleta**
  - Impacto: Algumas features só disponíveis via API ou dashboard
  - Mitigação: Features principais disponíveis em ambos

## 📊 Métricas de Qualidade Atuais

### Segurança
- ✅ Compilação limpa
- ✅ Self-protection Proxmox (infraestrutura principal)
- ✅ Audit logs completos Proxmox
- ✅ RBAC ativo em todos endpoints
- ⚠️ Vulnerabilidades de dependências (2)
- ⚠️ Plugins sem self-protection server-side

### Código
- ✅ Sem rotas duplicadas
- ✅ Arquitetura modular (plugins)
- ✅ Handlers consistentes
- ℹ️ Testes: verificar cobertura

### Operacional
- ✅ Docker Compose pronto
- ✅ Migrations database
- ✅ Health checks
- ✅ Logging estruturado
- ℹ️ Monitoring: verificar métricas exportadas

## 🚀 Plano de Ação Recomendado

### Sprint 1 (Segurança Crítica)
1. **Resolver vulnerabilidades Dependabot** ⚠️ URGENTE
2. Verificar e atualizar dependências Go e NPM
3. Testar build após atualizações

### Sprint 2 (Hardening)
1. Implementar self-protection nos plugins (P0.3b)
2. Adicionar testes para enforcePolicy
3. Documentar headers X-Confirmation e X-TOTP-Code

### Sprint 3 (UX)
1. RBAC visual no dashboard (P0.5)
2. Tooltips de permissões
3. Melhorar mensagens de erro

### Sprint 4 (Completude)
1. Inventário de paridade API/Dashboard (P0.6)
2. Implementar features prioritárias
3. Documentação de API

## 🎯 Critérios de "Pronto para Produção"

### Must Have (Bloqueadores) ✅ COMPLETO
- [x] Backend compila sem erros
- [x] Sem código duplicado
- [x] Self-protection em operações críticas principais (Proxmox)
- [x] Audit logs completos
- [x] RBAC funcional

### Should Have (Altamente Recomendado)
- [ ] Vulnerabilidades de dependências resolvidas ⚠️
- [ ] Self-protection em todos plugins
- [ ] RBAC visual no dashboard
- [ ] Testes de segurança

### Nice to Have (Desejável)
- [ ] Paridade API/Dashboard completa
- [ ] Documentação OpenAPI
- [ ] Testes E2E completos
- [ ] Coverage > 70%

## 📝 Notas de Deploy

### Pré-requisitos
1. ✅ Database migrations aplicadas
2. ✅ Variáveis de ambiente configuradas
3. ⚠️ Verificar vulnerabilidades resolvidas
4. ✅ Secrets montados corretamente

### Checklist de Deploy
- [ ] Backup do banco de dados
- [ ] Rodar migrations: `docker compose run --rm migrate`
- [ ] Build e deploy: `docker compose up -d`
- [ ] Verificar health checks: `curl localhost:8088/healthz`
- [ ] Testar login e RBAC
- [ ] Testar operação crítica com TOTP
- [ ] Verificar audit logs sendo gerados
- [ ] Monitorar logs por 30min

### Rollback Plan
```bash
# Se algo der errado:
cd /home/bill/dev/wc-hub
git checkout <commit-anterior-estavel>
docker compose down
docker compose up -d --build
# Restore database backup se necessário
```

## 🔗 Referências Úteis

- **Repositório:** https://github.com/gabrielbiaggi/wc-hub
- **Dependabot:** https://github.com/gabrielbiaggi/wc-hub/security/dependabot
- **Roadmap P0:** `/home/bill/dev/wc-hub/P0-ROADMAP.md`
- **Progress Summary:** `/home/bill/dev/wc-hub/PROGRESS-SUMMARY.md`
- **Security Engine:** `back/internal/security/domain/policy.go`
- **Audit Repository:** `back/internal/audit/repository/postgres.go`

## ✅ Recomendação Final

**O backend está PRONTO para deploy em produção** com as seguintes observações:

1. **Deploy imediatamente** se as vulnerabilidades Dependabot não forem críticas (verificar severity)
2. **Aguardar correção** se as vulnerabilidades forem HIGH/CRITICAL em componentes expostos
3. **Monitorar ativamente** audit logs após deploy
4. **Planejar Sprint 2** para completar self-protection nos plugins

A infraestrutura principal (Proxmox) está protegida com self-protection server-side e audit completo. Os plugins têm RBAC + validação browser, o que é aceitável como proteção em camadas até Sprint 2.
