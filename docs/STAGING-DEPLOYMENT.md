# Guia de Deploy em Staging e Rollback - WC Hub

Este documento descreve os procedimentos de implantação do **WC Hub** no ambiente de staging antes da promoção para produção em `hub.webcreations.com.br`.

---

## 1. Pré-Requisitos do Staging

- Docker 24.0+ e Docker Compose v2.20+
- Instância PostgreSQL 15+ com extensão `pg_trgm` (ou container `postgres:15-alpine`)
- Registro DNS para o ambiente de testes (`staging-hub.webcreations.com.br`)
- Chaves de criptografia (`ENCRYPTION_KEY` e `TOTP_ISSUER`) configuradas em `.env.staging`

---

## 2. Processo de Deploy em Staging

1. **Clonar / Atualizar o Repositório:**
   ```bash
   git fetch origin
   git checkout origin/main
   ```

2. **Executar a Suíte de Smoke Test:**
   ```bash
   ./scripts/smoke_test.sh
   ```

3. **Subir os Containers de Staging:**
   ```bash
   docker compose -f docker-compose.staging.yml up -d --build
   ```

4. **Verificar os Endpoints de Health Matrix:**
   ```bash
   curl -f http://localhost:8080/healthz
   curl -f http://localhost:8080/api/v1/auth/bootstrap-status
   ```

---

## 3. Procedimento de Rollback de Emergência

Em caso de degradação ou falha durante o deploy:

1. **Parar a versão com falha:**
   ```bash
   docker compose -f docker-compose.staging.yml down
   ```

2. **Reverter para o commit/tag estável anterior:**
   ```bash
   git checkout <LAST_STABLE_COMMIT_OR_TAG>
   ```

3. **Restaurar o banco de dados (se necessário):**
   ```bash
   ./scripts/backup_restore_test.sh
   ```

4. **Reiniciar a stack:**
   ```bash
   docker compose -f docker-compose.staging.yml up -d
   ```
