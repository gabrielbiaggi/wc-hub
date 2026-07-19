<script setup lang="ts">
import { computed } from 'vue'
import { useMutation, useQuery, useQueryClient } from '@tanstack/vue-query'
import { Activity, Boxes, Minus, Plus, RefreshCw, RotateCcw, Server, ShieldCheck, TriangleAlert } from 'lucide-vue-next'
import Button from '@/components/ui/Button.vue'
import StatusBadge from '@/components/ui/StatusBadge.vue'
import { getKubernetesOverview, runKubernetesDeploymentAction, type KubeDeployment, type KubeNode } from '@/lib/api_kubernetes'

const query = useQuery({ queryKey: ['kubernetes-overview'], queryFn: getKubernetesOverview, refetchInterval: 20000 })
const queryClient = useQueryClient()
const action = useMutation({ mutationFn: (input:{namespace:string;name:string;action:'scale'|'restart';replicas?:number}) => runKubernetesDeploymentAction(input.namespace,input.name,input.action,input.replicas), onSuccess:()=>setTimeout(()=>queryClient.invalidateQueries({queryKey:['kubernetes-overview']}),1000) })
const data = computed(() => query.data.value)
const ready = (node: KubeNode) => node.status.conditions?.find((condition) => condition.type === 'Ready')?.status === 'True'
const readyNodes = computed(() => data.value?.nodes.filter(ready).length ?? 0)
const readyDeployments = computed(() => data.value?.deployments.filter((deployment) => (deployment.status.readyReplicas ?? 0) >= (deployment.spec.replicas ?? 0)).length ?? 0)
const execute = (deployment:KubeDeployment, operation:'scale'|'restart', replicas?:number) => {
  if (window.confirm(`Confirma ${operation} em ${deployment.metadata.namespace}/${deployment.metadata.name}${replicas===undefined?'':` para ${replicas} réplica(s)`}?`)) action.mutate({namespace:deployment.metadata.namespace,name:deployment.metadata.name,action:operation,replicas})
}
</script>

<template>
  <div class="mx-auto max-w-[1680px] space-y-5">
    <header class="flex flex-col justify-between gap-4 md:flex-row md:items-end">
      <div>
        <div class="flex gap-2"><StatusBadge :status="query.isError.value ? 'critical' : 'healthy'" :label="query.isError.value ? 'cluster unavailable' : 'cluster operator'" /><span class="inline-flex items-center gap-1.5 font-mono text-[10px] uppercase text-signal"><ShieldCheck class="h-3.5 w-3.5" />Kubernetes RBAC</span></div>
        <h1 class="mt-4 text-3xl font-semibold">Kubernetes fabric</h1>
        <p class="mt-2 text-sm text-muted">Nós, deployments, pods problemáticos e eventos do cluster autorizado.</p>
      </div>
      <Button variant="outline" :disabled="query.isFetching.value" @click="query.refetch()"><RefreshCw :class="['h-4 w-4', query.isFetching.value && 'animate-spin']" />Atualizar</Button>
    </header>

    <div v-if="query.isError.value" class="rounded-xl border border-danger/20 bg-danger/5 p-4 text-sm text-danger">Não foi possível consultar a API Kubernetes. Verifique endpoint, CA e ServiceAccount.</div>
    <div v-if="action.isError.value" class="rounded-xl border border-danger/20 bg-danger/5 p-4 text-sm text-danger">O cluster rejeitou a alteração do deployment.</div>
    <section class="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
      <article v-for="item in [{ l: 'Nodes ready', v: `${readyNodes}/${data?.nodes.length ?? 0}`, i: Server }, { l: 'Deployments ready', v: `${readyDeployments}/${data?.deployments.length ?? 0}`, i: Boxes }, { l: 'Problem pods', v: data?.problem_pods.length ?? 0, i: TriangleAlert }, { l: 'Warning events', v: data?.events.length ?? 0, i: Activity }]" :key="item.l" class="rounded-xl border border-line bg-panel/65 p-5">
        <component :is="item.i" class="h-4 w-4 text-muted" /><p class="mt-5 font-mono text-2xl text-white">{{ item.v }}</p><p class="mt-1 text-xs text-muted">{{ item.l }}</p>
      </article>
    </section>

    <section class="grid gap-5 xl:grid-cols-[1fr_1.2fr]">
      <article class="overflow-hidden rounded-xl border border-line bg-panel/65">
        <header class="border-b border-line p-4"><h2 class="text-sm font-medium">Cluster nodes</h2></header>
        <div class="divide-y divide-line/60">
          <div v-for="node in data?.nodes" :key="node.metadata.uid" class="flex items-center gap-3 p-4"><Server class="h-4 w-4 text-pulse" /><div class="min-w-0 flex-1"><p class="text-sm text-slate-200">{{ node.metadata.name }}</p><p class="mt-1 text-[10px] text-muted">{{ node.status.nodeInfo?.kubeletVersion }} · {{ node.status.nodeInfo?.architecture }}</p></div><StatusBadge :status="ready(node) ? 'healthy' : 'critical'" :label="ready(node) ? 'ready' : 'not ready'" /></div>
          <p v-if="!data?.nodes.length" class="p-10 text-center text-sm text-muted">Nenhum nó retornado.</p>
        </div>
      </article>
      <article class="overflow-hidden rounded-xl border border-line bg-panel/65">
        <header class="border-b border-line p-4"><h2 class="text-sm font-medium">Deployments</h2></header>
        <div class="divide-y divide-line/60"><div v-for="deployment in data?.deployments" :key="deployment.metadata.uid" class="grid gap-3 p-4 md:grid-cols-[1fr_90px_250px] md:items-center"><div><p class="text-sm text-slate-200">{{ deployment.metadata.name }}</p><p class="mt-1 font-mono text-[9px] text-muted">{{ deployment.metadata.namespace }} · available {{deployment.status.availableReplicas??0}} · unavailable {{deployment.status.unavailableReplicas??0}}</p></div><StatusBadge :status="(deployment.status.readyReplicas ?? 0) >= (deployment.spec.replicas ?? 0) ? 'healthy' : 'warning'" :label="`${deployment.status.readyReplicas??0}/${deployment.spec.replicas??0}`"/><div class="flex justify-end gap-2"><Button variant="outline" :disabled="action.isPending.value || deployment.spec.replicas<=0" @click="execute(deployment,'scale',deployment.spec.replicas-1)"><Minus class="h-3.5 w-3.5"/></Button><Button variant="outline" :disabled="action.isPending.value" @click="execute(deployment,'scale',deployment.spec.replicas+1)"><Plus class="h-3.5 w-3.5"/></Button><Button variant="outline" :disabled="action.isPending.value" @click="execute(deployment,'restart')"><RotateCcw class="h-3.5 w-3.5"/>Restart</Button></div></div></div>
      </article>
    </section>

    <section v-if="data?.problem_pods.length" class="rounded-xl border border-danger/20 bg-danger/[.03]">
      <header class="border-b border-danger/10 p-4"><h2 class="text-sm font-medium">Pods requiring attention</h2></header>
      <div class="divide-y divide-line/60"><div v-for="pod in data.problem_pods" :key="pod.metadata.uid" class="flex items-center gap-3 p-4"><TriangleAlert class="h-4 w-4 text-danger" /><div class="flex-1"><p class="text-sm text-slate-200">{{ pod.metadata.name }}</p><p class="text-[10px] text-muted">{{ pod.metadata.namespace }} · {{ pod.status.reason || pod.status.phase }}</p></div><StatusBadge status="critical" :label="pod.status.phase" /></div></div>
    </section>

    <section v-if="data?.events.length" class="overflow-hidden rounded-xl border border-warning/20 bg-warning/[.03]">
      <header class="border-b border-warning/10 p-4"><h2 class="text-sm font-medium">Recent warning events</h2></header>
      <div class="divide-y divide-line/60">
        <div v-for="event in data.events" :key="event.metadata.uid" class="flex items-start gap-3 p-4"><Activity class="mt-0.5 h-4 w-4 shrink-0 text-warning" /><div class="min-w-0 flex-1"><div class="flex flex-wrap items-center gap-2"><p class="text-sm text-slate-200">{{ event.reason || 'Warning' }}</p><span class="font-mono text-[9px] uppercase text-muted">{{ event.regarding?.kind || event.involvedObject?.kind }} {{ event.regarding?.name || event.involvedObject?.name }}</span></div><p class="mt-1 text-xs text-muted">{{ event.message }}</p></div><span v-if="event.count" class="font-mono text-[10px] text-warning">×{{ event.count }}</span></div>
      </div>
    </section>
  </div>
</template>
