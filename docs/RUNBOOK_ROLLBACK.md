# Runbook de rollback

## Princípio

Rollback reimplanta uma revisão conhecida e preserva evidências. Não apague volumes, logs ou registros de auditoria. Se a mudança alterou o schema de forma incompatível, trate a restauração de banco como incidente separado e obtenha aprovação explícita.

## Coleta inicial

```bash
date -u
git rev-parse HEAD
docker compose ps
docker compose logs --since=30m --tail=500 > wc-hub-rollback.log
```

Anote o último SHA saudável, o SHA com falha e o backup anterior ao deploy.

## Reversão da aplicação

Use uma tag ou SHA conhecido; não force `main` nem reescreva histórico.

```bash
git fetch --tags origin
git switch --detach <SHA_SAUDAVEL>
docker compose build
docker compose up -d --remove-orphans
curl -fsS http://127.0.0.1:8088/healthz
```

Se as imagens são versionadas e publicadas por registry, prefira fixar a tag imutável do release anterior e reaplicar Compose.

## Banco de dados

Não execute migrations `down` automaticamente. Primeiro confirme que a versão anterior da aplicação é compatível com o schema atual. Se não for, pare mutações, coloque o serviço atrás de uma página de manutenção e siga `RUNBOOK_BACKUP_RESTORE.md` usando um banco novo ou restore ensaiado.

## Verificação e encerramento

- Confirme login, sessão/CSRF, leitura de inventário e audit trail.
- Confirme que nenhum segredo apareceu em logs.
- Compare contagens de usuários, integrações, hosts e eventos antes/depois.
- Abra incidente com timeline, impacto, causa provável, revisão revertida e artefatos coletados.
- Retorne a um branch rastreado depois da estabilização; o checkout detached é apenas operacional.
