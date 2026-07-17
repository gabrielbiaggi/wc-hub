# Runbook de backup e restore

## Escopo

O backup mínimo inclui PostgreSQL, `.env`/referências do cofre, configuração do proxy/túnel e arquivos operacionais externos. Nunca coloque dumps ou segredos no repositório. Armazene cópias criptografadas fora do host e aplique retenção definida pela operação.

## Backup do PostgreSQL

Identifique o nome real do serviço com `docker compose ps`. O exemplo pressupõe `postgres` e o banco/usuário `wc_hub`.

```bash
umask 077
mkdir -p /var/backups/wc-hub
docker compose exec -T postgres pg_dump -U wc_hub -d wc_hub --format=custom --no-owner --no-acl > /var/backups/wc-hub/wc-hub-$(date -u +%Y%m%dT%H%M%SZ).dump
sha256sum /var/backups/wc-hub/wc-hub-*.dump
```

Copie o arquivo e seu checksum para armazenamento off-host criptografado. Não considere o backup válido antes de um restore de teste.

## Restore ensaiado

Restaure primeiro em banco isolado, nunca sobre produção.

```bash
createdb wc_hub_restore_test
pg_restore --exit-on-error --clean --if-exists --no-owner --no-acl --dbname=wc_hub_restore_test < <BACKUP.dump>
psql wc_hub_restore_test -c 'SELECT count(*) FROM users;'
psql wc_hub_restore_test -c 'SELECT count(*) FROM audit_logs;'
```

Valide migrations esperadas, integridade referencial, login com credencial de teste e leitura do audit trail. Destrua o ambiente de teste conforme a política local após registrar o resultado.

## Restore de desastre

1. Bloqueie tráfego e mutações.
2. Preserve o banco quebrado e os logs; não sobrescreva a única cópia.
3. Crie uma instância PostgreSQL vazia e compatível.
4. Execute `pg_restore --exit-on-error` no banco novo.
5. Aponte uma instância isolada da aplicação para o banco restaurado.
6. Rode healthcheck e smoke tests antes de alterar o tráfego.
7. Rotacione credenciais se houver suspeita de exposição.

Registre RPO, RTO real, horário do último registro recuperado, checksum usado e aprovador da retomada.
