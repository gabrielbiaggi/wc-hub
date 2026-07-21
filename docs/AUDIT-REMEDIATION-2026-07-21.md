# Remediação da auditoria — 2026-07-21

Este documento substitui afirmações absolutas de prontidão presentes nos relatórios anteriores. Ele registra somente verificações reproduzíveis no checkout.

## Corrigido e verificado

- `go test ./...` e `go vet ./...` passam com Go 1.25.
- `npm ci`, type-check e build de produção do frontend passam.
- `govulncheck ./...` não encontrou vulnerabilidades alcançáveis após a atualização de `x/crypto`, `x/net` e `moby/spdystream`.
- O resolver reconhece identificadores canônicos de Docker, Kubernetes e Terraform sem permitir bypass por prefixos adicionados pelos handlers.
- Proxmox propaga o status self-protected para power, delete, snapshot delete e rollback.
- Docker, Kubernetes, Terraform e Proxmox possuem fluxo visual de confirmação exata e TOTP, com os headers verificados no backend.
- As telas operacionais principais desabilitam mutações sem a permissão RBAC correspondente.
- O indicador de autoproteção da sidebar usa o estado retornado pelo backend e mostra estado degradado quando indisponível.
- A especificação OpenAPI é válida e cobre as 118 combinações método/rota públicas encontradas no backend. O gate interno pré-push falha quando uma nova rota não for documentada.
- O inventário de rotas é extraído do código atual; não usa mais uma lista manual incompleta.
- A validação interna pré-push usa a versão Go declarada em `go.mod` e executa testes, vet, build, OpenAPI, cobertura de rotas, `govulncheck` e `npm audit`.
- O validador de backup não produz mais falso positivo em modo simulado.

## Limites da evidência

- Por decisão operacional, o GitHub é somente o repositório remoto e não executa CI/CD. Todos os gates devem passar na infraestrutura interna antes do `git push`.
- A cobertura OpenAPI garante presença de operações. Schemas e exemplos detalhados ainda podem ser enriquecidos sem alterar o contrato de cobertura.

## Evidência final de staging local

Executado em 2026-07-21, em uma stack isolada `wc-hub-audit`, sem reutilizar o volume PostgreSQL da instância de desenvolvimento:

- migrations clean: versões `1 -> 9`, schema final com 29 tabelas e `dirty=false`;
- reversibilidade: `9 -> 0 -> 9` concluída sem erro;
- upgrade: banco separado migrado até a versão 8 e atualizado `8 -> 9`;
- backup real: `pg_dump --format=custom` do banco ativo e leitura validada por `pg_restore --list`;
- restore real: archive restaurado em `wc_hub_audit_restore`, com 29/29 tabelas e 1168/1168 audit logs;
- smoke production Compose: PostgreSQL, migrations, backend, frontend e Terraform worker iniciaram corretamente; serviços long-running ficaram healthy;
- HTTP: `/`, `/healthz` e bootstrap status retornaram 200; endpoint protegido sem sessão retornou 401;
- autenticação: bootstrap 201, overview autenticado 200, mutation sem CSRF 403 e logout com CSRF 204;
- self-protection: health e overview retornaram `self_protected=true`; container e host aliases foram injetados; o resolver reconhece o ID Docker completo a partir do hostname;
- runtime: logs finais do backend e Terraform worker sem eventos `ERROR`, panic ou fatal.

Os bancos, containers, redes, volumes e credenciais efêmeras usados pela auditoria foram removidos ao final. O host local é físico (`systemd-detect-virt=none`), portanto não há `HUB_PROXMOX_VMID` aplicável neste ambiente. Em uma futura VM Proxmox, esse valor continua obrigatório no deployment.

## Configuração obrigatória do self-target

Defina em produção todos os identificadores aplicáveis:

- `HUB_CONTAINER_ID` / `HUB_CONTAINER_NAME`
- `HUB_POD_NAME`
- `HUB_PROXMOX_VMID`
- `HUB_TERRAFORM_WORKSPACE`
- `HUB_HOST_ID`

Uma ação destrutiva resolvida para qualquer um desses alvos é bloqueada incondicionalmente, mesmo com confirmação e TOTP válidos.
