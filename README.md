# WC Hub

Central de operações para infraestrutura, projetos e acesso remoto. O WC Hub é um monorepo com um backend Go responsável por políticas, integrações, execução de jobs e auditoria, e um shell Vue focado em observabilidade e controle.

> Estado: control plane funcional em expansão. Identidade, RBAC, auditoria, Proxmox, Docker, Kubernetes, GitHub, Cloudflare, MergerFS, VNC, PBS, monitoramento/webhooks, NUT/Wake-on-LAN e o cliente de worker Terraform estão conectados ao runtime. WebAuthn e um worker Terraform distribuído ainda não foram entregues. Consulte a [auditoria de cobertura](docs/FEATURE_COVERAGE_AUDIT.md).

## Arquitetura

```text
wc-hub/
├── back/                  API e control plane em Go
│   ├── cmd/api/           composição e entrypoint
│   ├── db/migrations/     SQL puro, versionado
│   ├── internal/          domínios, adapters, workers e transportes
│   └── pkg/               tipos reutilizáveis sem dependência de domínio
├── front/                 Vue 3 + TypeScript + Vite
│   └── src/               app shell, features, stores e design system
├── infra/                 Docker, proxy, Terraform e manifests K3s
├── docs/                  arquitetura, segurança e decisões
├── ops/                   runbooks, scripts operacionais e systemd
└── design-system/         fonte de verdade visual
```

O fluxo de dependência do backend é `transport/adapters -> application -> domain`. Integrações implementam portas definidas por domínio. O frontend não executa infraestrutura diretamente: toda ação passa pela API, por autorização RBAC, pelo motor de políticas e por auditoria imutável.

## Segurança e self-protected

O host identificado por `WC_HUB_LOCAL_TARGET_NAME` nasce com `self_protected=true`. Para esse alvo, o backend nega incondicionalmente ações de desligamento, reboot, destruição, encerramento e comandos destrutivos. Não existe opção de confirmação que transforme essa negação em permissão.

O executor local aceita somente binários presentes em `WC_HUB_LOCAL_COMMAND_ALLOWLIST`; argumentos ainda passam por validação. Ações perigosas em alvos remotos exigem o nome exato do alvo e podem exigir TOTP. A API distingue escopos `local`, `remote` e `cloud` antes de avaliar a política. Veja [docs/SECURITY.md](docs/SECURITY.md).

## Início rápido

```bash
cp .env.example .env
docker compose up --build

Para desenvolvimento com atualização automática do Vue e do Go, use:

```bash
docker compose -f docker-compose.yml -f docker-compose.dev.yml up
```
```

Acesse `http://localhost:8088`. A API fica disponível pelo mesmo origin em `/api/v1`; o backend expõe healthcheck em `/healthz`.

Para desenvolvimento sem containers:

```bash
cd front && npm install && npm run dev
cd back && go run ./cmd/api
```

Go 1.24+, Node 20+ e PostgreSQL 17 são as versões de referência.

## Módulos

Proxmox, Kubernetes/K3s, Docker, GitHub, Cloudflare/Tunnels, Terraform, Telemetry, Remote Access, Desktop VNC, PBS/Backups, Uptime/Webhooks, Energia/NUT/Wake-on-LAN, Storage, Settings e Audit possuem rotas reais. O overview ainda contém indicadores demonstrativos. A situação detalhada e os critérios de conclusão estão na [matriz de cobertura](docs/FEATURE_COVERAGE_AUDIT.md).

## DNS e deploy

O destino inicial é uma VM Ubuntu Server no Proxmox. Publique o serviço em `hub.webcreations.com.br` por Cloudflare Tunnel ou reverse proxy TLS; nunca exponha Docker socket, SSH ou Postgres à internet. A topologia é portátil para Hostinger KVM 4 por Docker Compose.

Para configurar a infraestrutura real, consulte [docs/INFRASTRUCTURE_RUNTIME.md](docs/INFRASTRUCTURE_RUNTIME.md).

## Convenções

- migrations: `NNNNNN_description.up.sql` / `.down.sql`;
- APIs: `/api/v1`, JSON e request ID;
- jobs perigosos: política + confirmação + auditoria antes de enfileirar;
- segredos: nunca em banco em texto puro, logs, frontend ou `.env` versionado.

Commit inicial sugerido:

```text
feat: scaffold wc-hub god dashboard
```
