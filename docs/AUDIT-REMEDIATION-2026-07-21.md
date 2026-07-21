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
- A especificação OpenAPI é válida e cobre as 118 combinações método/rota públicas encontradas no backend. O CI falha quando uma nova rota não for documentada.
- O inventário de rotas é extraído do código atual; não usa mais uma lista manual incompleta.
- O CI usa a versão Go declarada em `go.mod` e executa testes, vet, build, OpenAPI, cobertura de rotas, `govulncheck` e `npm audit`.
- O validador de backup não produz mais falso positivo em modo simulado.

## Limites da evidência

- O workflow remoto criado no GitHub foi recusado antes de executar qualquer step com a anotação `The job was not started because your account is locked due to a billing issue.`. Os mesmos gates foram executados localmente, mas a conta GitHub precisa ser regularizada para fornecer evidência remota e permitir branch protection efetiva.
- Os testes de migrations existentes validam estrutura, mas não substituem um ciclo real clean/upgrade em PostgreSQL. Esse ciclo deve rodar no ambiente de staging.
- O script de backup valida a geração e leitura do archive; um restore destrutivo deve usar um banco descartável separado.
- A cobertura OpenAPI garante presença de operações. Schemas e exemplos detalhados ainda podem ser enriquecidos sem alterar o contrato de cobertura.
- A aprovação final de produção depende de smoke test no staging, credenciais reais, inventário self-target configurado e restore em banco descartável.

## Configuração obrigatória do self-target

Defina em produção todos os identificadores aplicáveis:

- `HUB_CONTAINER_ID` / `HUB_CONTAINER_NAME`
- `HUB_POD_NAME`
- `HUB_PROXMOX_VMID`
- `HUB_TERRAFORM_WORKSPACE`
- `HUB_HOST_ID`

Uma ação destrutiva resolvida para qualquer um desses alvos é bloqueada incondicionalmente, mesmo com confirmação e TOTP válidos.
