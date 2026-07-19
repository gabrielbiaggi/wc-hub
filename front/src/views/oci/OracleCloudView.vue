<script setup lang="ts">
import { computed, ref } from 'vue'
import { useMutation, useQuery, useQueryClient } from '@tanstack/vue-query'
import { Boxes, Cloud, Cpu, Globe2, MemoryStick, Network, Play, Power, RefreshCw, RotateCcw, ShieldCheck, Square, Waypoints } from 'lucide-vue-next'
import Button from '@/components/ui/Button.vue'
import StatusBadge from '@/components/ui/StatusBadge.vue'
import { getOCIOverview, runOCIInstanceAction, type OCIInstance, type OCIInstanceAction } from '@/lib/api'

type Tab='instances'|'network'|'identity'
const tab=ref<Tab>('instances')
const client=useQueryClient()
const overview=useQuery({queryKey:['oci-overview'],queryFn:getOCIOverview,refetchInterval:30000,retry:1})
const action=useMutation({mutationFn:(input:{instance:OCIInstance;action:OCIInstanceAction})=>runOCIInstanceAction(input.instance.id,input.action),onSuccess:()=>setTimeout(()=>client.invalidateQueries({queryKey:['oci-overview']}),1800)})
const running=computed(()=>overview.data.value?.instances.filter(item=>item.lifecycle_state==='RUNNING').length??0)
const compartmentName=(id:string)=>overview.data.value?.compartments.find(item=>item.id===id)?.name??'tenancy raiz'
const vcnName=(id:string)=>overview.data.value?.vcns.find(item=>item.id===id)?.display_name??id.split('.').at(-1)?.slice(0,12)??'VCN'
const execute=(instance:OCIInstance,operation:OCIInstanceAction)=>{
  const confirmation=`${operation.toUpperCase()} ${instance.display_name}`
  if(window.prompt(`Ação real na OCI. Digite exatamente: ${confirmation}`)===confirmation)action.mutate({instance,action:operation})
}
</script>

<template>
  <div class="mx-auto max-w-[1680px] space-y-5">
    <header class="flex flex-col justify-between gap-4 md:flex-row md:items-end">
      <div><div class="flex flex-wrap gap-2"><StatusBadge :status="overview.isError.value?'critical':'healthy'" :label="overview.isError.value?'OCI indisponível':'API assinada ativa'"/><span class="inline-flex items-center gap-1.5 font-mono text-[10px] uppercase text-signal"><ShieldCheck class="h-3.5 w-3.5"/>chave de assinatura da API · controle total auditado</span></div><h1 class="mt-4 text-3xl font-semibold">Oracle Cloud Infrastructure</h1><p class="mt-2 text-sm text-muted">Computação, regiões, compartimentos e topologia de rede carregados diretamente da tenancy OCI.</p></div>
      <Button variant="outline" :disabled="overview.isFetching.value" @click="overview.refetch()"><RefreshCw :class="['h-4 w-4',overview.isFetching.value&&'animate-spin']"/>Atualizar</Button>
    </header>
    <div v-if="overview.isError.value||action.isError.value" class="rounded-xl border border-danger/20 bg-danger/5 p-4 text-sm text-danger">Não foi possível concluir a operação OCI. Verifique a política IAM, assinatura da API e o estado atual da instância.</div>
    <section class="grid gap-3 sm:grid-cols-2 xl:grid-cols-5">
      <article v-for="item in [{label:'Regiões',value:overview.data.value?.regions.length??0,icon:Globe2},{label:'Compartimentos',value:overview.data.value?.compartments.length??0,icon:Boxes},{label:'Instâncias',value:overview.data.value?.instances.length??0,icon:Cpu},{label:'Em execução',value:running,icon:Play},{label:'VCNs / Sub-redes',value:`${overview.data.value?.vcns.length??0} / ${overview.data.value?.subnets.length??0}`,icon:Network}]" :key="item.label" class="rounded-xl border border-line bg-panel/65 p-5"><component :is="item.icon" class="h-4 w-4 text-muted"/><p class="mt-5 font-mono text-2xl text-white">{{item.value}}</p><p class="mt-1 text-xs text-muted">{{item.label}}</p></article>
    </section>
    <section class="overflow-hidden rounded-xl border border-line bg-panel/65">
      <header class="flex flex-col gap-3 border-b border-line p-4 md:flex-row md:items-center md:justify-between"><div><h2 class="text-sm font-medium">Inventário da tenancy</h2><p class="mt-1 font-mono text-[9px] text-muted">Principal {{overview.data.value?.home_region||'—'}} · captura {{overview.data.value?new Date(overview.data.value.captured_at).toLocaleString('pt-BR'):'—'}}</p></div><nav class="flex rounded-lg border border-line p-1"><button v-for="item in [{id:'instances',label:'Computação'},{id:'network',label:'Rede'},{id:'identity',label:'Regiões e ADs'}]" :key="item.id" :class="['rounded px-3 py-1.5 text-xs',tab===item.id?'bg-signal/10 text-signal':'text-muted']" @click="tab=item.id as Tab">{{item.label}}</button></nav></header>

      <div v-if="tab==='instances'" class="divide-y divide-line/60">
        <div v-for="instance in overview.data.value?.instances" :key="instance.id" class="grid gap-4 p-4 xl:grid-cols-[1fr_260px_320px] xl:items-center">
          <div><div class="flex flex-wrap items-center gap-2"><p class="text-sm text-slate-200">{{instance.display_name||'instância sem nome'}}</p><StatusBadge :status="instance.lifecycle_state==='RUNNING'?'healthy':instance.lifecycle_state==='STOPPED'?'warning':'info'" :label="instance.lifecycle_state"/></div><p class="mt-2 font-mono text-[9px] text-muted">{{instance.shape}} · {{instance.availability_domain}} · {{instance.fault_domain||'sem domínio de falha'}}</p><p class="mt-1 truncate font-mono text-[9px] text-muted" :title="instance.id">{{instance.id}}</p></div>
          <div class="grid grid-cols-2 gap-3"><div class="rounded-lg border border-line/60 p-3"><Cpu class="h-3.5 w-3.5 text-muted"/><p class="mt-2 font-mono text-sm">{{instance.ocpus||'—'}}</p><p class="text-[9px] text-muted">OCPUs</p></div><div class="rounded-lg border border-line/60 p-3"><MemoryStick class="h-3.5 w-3.5 text-muted"/><p class="mt-2 font-mono text-sm">{{instance.memory_gb||'—'}} GB</p><p class="text-[9px] text-muted">Memória</p></div><p class="col-span-2 truncate text-[10px] text-muted">{{compartmentName(instance.compartment_id)}}</p></div>
          <div class="flex flex-wrap justify-end gap-2"><Button v-if="instance.lifecycle_state!=='RUNNING'" variant="outline" :disabled="action.isPending.value" @click="execute(instance,'start')"><Play class="h-3.5 w-3.5"/>Iniciar</Button><template v-else><Button variant="outline" :disabled="action.isPending.value" @click="execute(instance,'reboot')"><RotateCcw class="h-3.5 w-3.5"/>Reiniciar</Button><Button variant="outline" :disabled="action.isPending.value" @click="execute(instance,'shutdown')"><Power class="h-3.5 w-3.5"/>Desligar</Button><Button variant="danger" :disabled="action.isPending.value" @click="execute(instance,'stop')"><Square class="h-3.5 w-3.5"/>Parar</Button></template></div>
        </div><p v-if="!overview.data.value?.instances.length" class="p-10 text-center text-sm text-muted">Nenhuma instância ativa retornada.</p>
      </div>

      <div v-else-if="tab==='network'" class="grid gap-px bg-line/60 lg:grid-cols-2">
        <article v-for="vcn in overview.data.value?.vcns" :key="vcn.id" class="bg-panel p-5"><div class="flex items-center justify-between gap-3"><div><div class="flex items-center gap-2"><Network class="h-4 w-4 text-signal"/><p class="text-sm text-slate-200">{{vcn.display_name||vcn.dns_label||'VCN sem nome'}}</p></div><p class="mt-2 font-mono text-[9px] text-muted">{{vcn.cidr_blocks.join(', ')}} · {{compartmentName(vcn.compartment_id)}}</p></div><StatusBadge :status="vcn.lifecycle_state==='AVAILABLE'?'healthy':'warning'" :label="vcn.lifecycle_state"/></div><div class="mt-4 space-y-2"><div v-for="subnet in overview.data.value?.subnets.filter(item=>item.vcn_id===vcn.id)" :key="subnet.id" class="flex items-center justify-between rounded-lg border border-line/60 p-3"><div class="flex items-center gap-2"><Waypoints class="h-3.5 w-3.5 text-muted"/><div><p class="text-xs text-slate-300">{{subnet.display_name||'sub-rede sem nome'}}</p><p class="mt-1 font-mono text-[9px] text-muted">{{subnet.cidr_block}} · {{subnet.availability_domain||'regional'}}</p></div></div><StatusBadge :status="subnet.lifecycle_state==='AVAILABLE'?'healthy':'warning'" :label="subnet.prohibit_public_ip_on_vnic?'privada':'IP público permitido'"/></div></div></article><p v-if="!overview.data.value?.vcns.length" class="bg-panel p-10 text-center text-sm text-muted lg:col-span-2">Nenhuma VCN retornada.</p>
      </div>

      <div v-else class="grid gap-px bg-line/60 md:grid-cols-2 xl:grid-cols-3"><article v-for="region in overview.data.value?.regions" :key="region.name" class="bg-panel p-5"><div class="flex items-center justify-between"><div class="flex items-center gap-2"><Cloud class="h-4 w-4 text-muted"/><p class="text-sm text-slate-200">{{region.name}}</p></div><StatusBadge :status="region.status==='READY'?'healthy':'warning'" :label="region.home?'região principal':region.status"/></div></article><article v-for="domain in overview.data.value?.availability_domains" :key="domain.name" class="bg-panel p-5"><div class="flex items-center gap-2"><Globe2 class="h-4 w-4 text-signal"/><div><p class="text-sm text-slate-200">{{domain.name}}</p><p class="mt-1 text-[10px] text-muted">Domínio de disponibilidade</p></div></div></article></div>
    </section>
  </div>
</template>
