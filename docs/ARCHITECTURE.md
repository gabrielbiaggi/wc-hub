# Architecture

## Control plane

WC Hub is a control plane, not a collection of browser-side API clients. The Go service owns credentials, inventory normalization, policy decisions, jobs, scheduling, WebSocket streams, terminal brokering, file access and audit records.

```text
Vue shell -> HTTP/WS transport -> application services -> domain policies
                                                    -> ports -> adapters
                                                    -> job queue -> workers
```

Integration adapters normalize Proxmox, Docker, Kubernetes, GitHub, Cloudflare and cloud resources into the central inventory. Collectors write time-series snapshots; PostgreSQL keeps operational state and audit metadata. A dedicated metrics backend can be introduced when retention or cardinality exceeds PostgreSQL's envelope.

## Backend boundaries

- `internal/<domain>/domain`: entities, policies and ports;
- `internal/<domain>/application`: use cases and orchestration;
- `internal/adapters`: external APIs and runtime-specific implementations;
- `internal/platform`: configuration, persistence, crypto and transport;
- `internal/jobs`: durable jobs, workers and scheduler;
- `pkg`: small packages safe to consume across domains.

Every mutating use case follows `authenticate -> authorize -> classify target -> evaluate policy -> confirm -> audit intent -> enqueue/execute -> audit result`.

## Frontend boundaries

Vue Router owns module navigation, Pinia owns ephemeral UI state, TanStack Query owns server state and caching, and TanStack Table owns large inventory projections. ECharts renders telemetry, xterm.js renders terminal streams, and splitpanes provides remote workspace layouts. Components under `components/ui` follow the shadcn-vue composition model and are locally owned.

## Deployment

The initial Compose topology exposes only nginx. API and PostgreSQL remain on the private `control-plane` network. Production should terminate TLS through Cloudflare Tunnel or a host reverse proxy and restrict the origin to Cloudflare/private networks.

