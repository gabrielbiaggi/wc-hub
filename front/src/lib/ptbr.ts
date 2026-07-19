const traducoes: Record<string,string> = {
  healthy:'saudável', warning:'atenção', critical:'crítico', info:'informativo',
  running:'em execução', exited:'encerrado', stopped:'parado', online:'online', offline:'offline',
  ready:'pronto', 'not ready':'não pronto', available:'disponível', unavailable:'indisponível',
  connected:'conectado', disconnected:'desconectado', active:'ativo', inactive:'inativo',
  enabled:'habilitado', disabled:'desativado', required:'obrigatório', pending:'pendente',
  succeeded:'concluído', failed:'falhou', completed:'concluído', success:'sucesso', failure:'falha',
  cancelled:'cancelado', queued:'enfileirado', running_job:'em execução',
  allowed:'permitido', denied:'negado', safe:'seguro', dangerous:'perigoso',
  open:'aberto', acknowledged:'reconhecido', resolved:'resolvido', unknown:'desconhecido',
  degraded:'degradado', down:'fora do ar', private:'privado', public:'público', protected:'protegido',
  admin:'administrador', limited:'limitado', stable:'estável', prerelease:'pré-lançamento',
  development:'desenvolvimento', production:'produção', connecting:'conectando',
  'system operational':'sistema operacional', 'daemon reachable':'daemon acessível',
  'proxy unavailable':'proxy indisponível', 'cluster unavailable':'cluster indisponível',
  'cluster operator':'operador do cluster', 'provider unavailable':'provedor indisponível',
  'worker unavailable':'worker indisponível', 'ephemeral worker':'worker efêmero',
  'signed api active':'API assinada ativa', 'oci unavailable':'OCI indisponível',
  'home region':'região principal', 'public ip allowed':'IP público permitido',
  'full token / zone allowlist':'token completo / lista de zonas permitidas',
  overview:'visão geral', inventory:'inventário', telemetry:'telemetria', notifications:'notificações',
  settings:'configurações', integrations:'integrações', audit:'auditoria',
  jobs:'tarefas', cloud:'nuvem', docker:'Docker', kubernetes:'Kubernetes', github:'GitHub',
  cloudflare:'Cloudflare', terraform:'Terraform', proxmox:'Proxmox', storage:'armazenamento',
  'remote-access':'acesso remoto', local:'local', remote:'remoto', mixed:'mista',
  configured:'configurado', unconfigured:'não configurado',
  provisioning:'provisionando', terminating:'encerrando', terminated:'encerrado', starting:'iniciando',
  stopping:'parando', rebooting:'reiniciando', resetting:'reinicializando', needs_attention:'requer atenção',
}

export const traduzirTexto=(valor:string|undefined|null)=>{
  if(!valor)return valor??''
  return traducoes[valor.toLowerCase()]??valor
}

export const traduzirOperacao=(valor:string)=>({validate:'validar',plan:'planejar',apply:'aplicar',rerun:'reexecutar',cancel:'cancelar',restart:'reiniciar',start:'iniciar',stop:'parar',shutdown:'desligar',reboot:'reiniciar',reset:'reinicializar',scale:'escalar'}[valor]??valor)

export const traduzirMetrica=(valor:string)=>({
  'Compute nodes':'Nós de computação','Active workloads':'Cargas de trabalho ativas',
  'Telemetry samples':'Amostras de telemetria','Open alerts':'Alertas abertos',
  discovered:'descobertos','last hour':'última hora',signals:'sinais',online:'online',
}[valor]??traduzirTexto(valor))
