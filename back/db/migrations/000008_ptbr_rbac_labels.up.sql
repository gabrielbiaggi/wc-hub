UPDATE roles SET
  name = CASE slug
    WHEN 'god-admin' THEN 'Administrador soberano'
    WHEN 'operator' THEN 'Operador'
    WHEN 'auditor' THEN 'Auditor'
    ELSE name
  END,
  description = CASE slug
    WHEN 'god-admin' THEN 'Operador com acesso total à plataforma; ações críticas ainda exigem aprovação da política.'
    WHEN 'operator' THEN 'Gerencia operações de infraestrutura não críticas.'
    WHEN 'auditor' THEN 'Acesso somente leitura à telemetria e às trilhas de auditoria.'
    ELSE description
  END
WHERE slug IN ('god-admin', 'operator', 'auditor');

UPDATE permissions SET description = CASE slug
  WHEN 'overview.read' THEN 'Consultar a visão operacional global.'
  WHEN 'telemetry.read' THEN 'Consultar a telemetria da infraestrutura.'
  WHEN 'hosts.execute.safe' THEN 'Executar comandos permitidos em hosts gerenciados.'
  WHEN 'hosts.execute.critical' THEN 'Solicitar comandos críticos em hosts remotos.'
  WHEN 'terraform.plan' THEN 'Criar planos do Terraform.'
  WHEN 'terraform.apply' THEN 'Aplicar planos do Terraform aprovados.'
  WHEN 'terminal.open' THEN 'Abrir sessões de terminal remoto auditadas.'
  WHEN 'storage.read' THEN 'Navegar nos volumes de armazenamento configurados.'
  WHEN 'storage.write' THEN 'Modificar arquivos em volumes graváveis.'
  WHEN 'audit.read' THEN 'Consultar logs de auditoria.'
  WHEN 'proxmox.read' THEN 'Consultar inventário e estado do Proxmox.'
  WHEN 'proxmox.sync' THEN 'Sincronizar o inventário do Proxmox.'
  WHEN 'jobs.read' THEN 'Consultar a fila de tarefas e o estado do agendador.'
  WHEN 'jobs.manage' THEN 'Enfileirar e cancelar tarefas operacionais.'
  WHEN 'terminal.connect' THEN 'Criar tickets auditados de terminal SSH.'
  WHEN 'agents.manage' THEN 'Provisionar e gerenciar agentes de hosts.'
  WHEN 'docker.read' THEN 'Consultar containers, imagens e métricas de execução do Docker.'
  WHEN 'kubernetes.read' THEN 'Consultar nós, cargas de trabalho e eventos do Kubernetes.'
  WHEN 'cloudflare.read' THEN 'Consultar túneis e registros DNS permitidos do Cloudflare.'
  WHEN 'github.read' THEN 'Consultar repositórios, workflows e releases permitidos do GitHub.'
  WHEN 'terraform.read' THEN 'Consultar resultados de validação e planejamento do Terraform.'
  WHEN 'docker.manage' THEN 'Iniciar, parar e reiniciar containers Docker.'
  WHEN 'kubernetes.manage' THEN 'Escalar e reiniciar cargas de trabalho do Kubernetes.'
  WHEN 'cloudflare.manage' THEN 'Criar, atualizar e excluir recursos permitidos do Cloudflare.'
  WHEN 'github.manage' THEN 'Disparar, cancelar e reexecutar workflows em repositórios permitidos.'
  WHEN 'proxmox.manage' THEN 'Controlar máquinas virtuais e containers nos clusters Proxmox configurados.'
  WHEN 'oci.read' THEN 'Consultar regiões, compartimentos, computação e inventário de rede do Oracle Cloud.'
  WHEN 'oci.manage' THEN 'Iniciar, parar, reiniciar e reinicializar instâncias de computação do Oracle Cloud.'
  ELSE description
END
WHERE slug IN (
  'overview.read','telemetry.read','hosts.execute.safe','hosts.execute.critical','terraform.plan',
  'terraform.apply','terminal.open','storage.read','storage.write','audit.read','proxmox.read',
  'proxmox.sync','jobs.read','jobs.manage','terminal.connect','agents.manage','docker.read',
  'kubernetes.read','cloudflare.read','github.read','terraform.read','docker.manage',
  'kubernetes.manage','cloudflare.manage','github.manage','proxmox.manage','oci.read','oci.manage'
);
