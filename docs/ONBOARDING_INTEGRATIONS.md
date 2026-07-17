# Onboarding de integrações

## Fluxo padrão

1. Defina dono, finalidade, ambiente e recursos permitidos.
2. Crie uma identidade dedicada no provider; nunca reutilize conta pessoal.
3. Conceda o menor escopo possível e estabeleça expiração/rotação.
4. Registre apenas os metadados no Integrations Center.
5. Injete o segredo pelo mecanismo seguro do ambiente, fora do browser e do Git.
6. Valide saúde e uma operação somente leitura.
7. Revise o audit trail e documente como revogar a credencial.

Valores de tokens, chaves privadas, kubeconfigs, senhas e TOTP não devem aparecer em screenshots, tickets, logs ou respostas da API. A UI mostra apenas estado e campos mascarados.

## Checklist por provider

| Provider | Identidade recomendada | Escopo inicial | Validação |
|---|---|---|---|
| Proxmox | API token dedicado | audit/read nos pools necessários | listar nodes/VMs |
| Docker | agente mTLS ou socket proxy restrito | operações tipadas, sem shell | listar containers |
| Kubernetes | ServiceAccount dedicado | namespace/read mínimo | listar nodes/workloads permitidos |
| GitHub | GitHub App ou PAT fine-grained | repositórios explícitos, read inicial | listar repos/workflows |
| Cloudflare | API token scoped | conta/zone allowlist | consultar tunnel/DNS |
| Terraform | identidade efêmera do worker | plan antes de apply | validate/plan sem mutação |
| SSH | chave dedicada + known_hosts | alvos remotos allowlisted | handshake sem shell local |

## Rotação

Crie a nova credencial antes de revogar a antiga, injete-a no ambiente, reinicie apenas o componente consumidor e valide saúde. Depois revogue a anterior no provider e registre os identificadores — nunca o valor — no ticket e na auditoria.

## Offboarding e incidente

Desabilite a integração no control plane, revogue a credencial na origem, encerre sessões relacionadas e preserve logs. Em suspeita de vazamento, trate como incidente: rotacione imediatamente, revise audit trail por período e escopo, procure uso fora do allowlist e documente impacto.
