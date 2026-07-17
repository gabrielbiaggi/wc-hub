# Runbook de deploy

## Objetivo e pré-condições

Este procedimento publica uma revisão do WC Hub em um host Ubuntu com Docker Compose. Execute com uma conta operacional dedicada, janela de mudança aprovada e backup recente validado.

- Confirme Go 1.24+, Node 20+ e Docker Engine/Compose nas estações que farão builds.
- Mantenha `.env` fora do Git, com permissão `0600` e segredos provenientes do cofre operacional.
- Confirme `WC_HUB_SELF_PROTECTED=true`, o alvo local exato e cookies seguros em produção.
- Registre revisão Git, operador, horário, motivo e plano de rollback no ticket da mudança.

## Preparação

```bash
git fetch --prune origin
git status --short
git rev-parse HEAD
docker compose config --quiet
cd back && go test ./...
cd ../front && npm ci && npm run build
```

Não prossiga com worktree sujo, configuração inválida ou testes falhando. Gere um backup conforme `RUNBOOK_BACKUP_RESTORE.md` antes de migrations.

## Publicação

```bash
git switch main
git pull --ff-only origin main
docker compose build --pull
docker compose run --rm migrate
docker compose up -d --remove-orphans
```

Não use `docker compose down -v`: volumes contêm estado persistente. Migrations devem ser compatíveis com a versão anterior durante a janela de rollback.

## Validação

```bash
docker compose ps
curl -fsS http://127.0.0.1:8088/healthz
docker compose logs --since=10m --tail=200
```

Valide também pelo browser: login, Overview, Access Control, Integrations, Notifications e Audit. Confirme ausência de erros de console, respostas 5xx e negações RBAC inesperadas. Registre o SHA publicado e encerre a mudança somente após um período de observação.

## Critérios de abortar

Inicie rollback se healthcheck falhar repetidamente, migrations não concluírem, autenticação quebrar, o frontend não carregar ou houver perda de acesso a dados. Preserve logs e não tente corrigir produção manualmente durante a reversão.
