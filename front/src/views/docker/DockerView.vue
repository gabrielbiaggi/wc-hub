<script setup lang="ts">
import { computed, ref } from 'vue'
import { useMutation, useQuery, useQueryClient } from '@tanstack/vue-query'
import { Box, Boxes, Container, Cpu, Database, HardDrive, Network, Play, RefreshCw, RotateCcw, ShieldCheck, Square, Terminal } from 'lucide-vue-next'
import Button from '@/components/ui/Button.vue'
import StatusBadge from '@/components/ui/StatusBadge.vue'
import { execDockerContainer, getDockerInventory, runDockerContainerAction, type DockerContainer, type DockerContainerStats } from '@/lib/api_docker'

const tab = ref<'containers' | 'images'>('containers')
const queryClient = useQueryClient()
const query = useQuery({ queryKey: ['docker-inventory'], queryFn: getDockerInventory, refetchInterval: 15_000 })
const action = useMutation({ mutationFn: (input: { id: string; action: 'start' | 'stop' | 'restart' }) => runDockerContainerAction(input.id, input.action), onSuccess: () => setTimeout(() => queryClient.invalidateQueries({ queryKey: ['docker-inventory'] }), 800) })
const terminal=ref<{container:DockerContainer;command:string;output:string}|null>(null)
const exec=useMutation({mutationFn:()=>execDockerContainer(terminal.value!.container.id,['sh','-lc',terminal.value!.command]),onSuccess:(result)=>{if(terminal.value)terminal.value.output=result.output}})
const containers = computed(() => query.data.value?.containers ?? [])
const images = computed(() => query.data.value?.images ?? [])
const running = computed(() => containers.value.filter((item) => item.state === 'running').length)
const totalImageBytes = computed(() => images.value.reduce((total, item) => total + item.size, 0))
const statsByContainer = computed(() => new Map((query.data.value?.stats ?? []).map((item) => [item.container_id, item])))
const formatBytes = (value = 0) => {
  if (!value) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  const index = Math.min(Math.floor(Math.log(value) / Math.log(1024)), units.length - 1)
  return `${(value / 1024 ** index).toFixed(index > 1 ? 1 : 0)} ${units[index]}`
}
const shortID = (value: string) => value.replace(/^sha256:/, '').slice(0, 12)
const nameOf = (names: string[]) => names[0]?.replace(/^\//, '') || 'sem nome'
const statsOf = (id: string): DockerContainerStats | undefined => statsByContainer.value.get(id)
const execute = (id: string, operation: 'start' | 'stop' | 'restart') => {
  if (window.confirm(`Confirma a operação no container ${shortID(id)}?`)) action.mutate({ id, action: operation })
}
const openTerminal=(container:DockerContainer)=>{terminal.value={container,command:'id && uname -a',output:''}}
const runCommand=()=>{if(terminal.value&&window.confirm(`Executar no container ${nameOf(terminal.value.container.names)}?`))exec.mutate()}
</script>

<template>
  <div class="mx-auto max-w-[1680px] space-y-5">
    <header class="flex flex-col justify-between gap-4 md:flex-row md:items-end">
      <div>
        <div class="flex flex-wrap gap-2">
          <StatusBadge :status="query.data.value?.health.reachable ? 'healthy' : 'warning'" :label="query.data.value?.health.reachable ? 'daemon acessível' : 'proxy indisponível'" />
          <span class="inline-flex items-center gap-1.5 font-mono text-[10px] uppercase tracking-wider text-signal"><ShieldCheck class="h-3.5 w-3.5" />proxy de operações autenticado</span>
        </div>
        <h1 class="mt-4 text-3xl font-semibold tracking-tight">Ambiente Docker</h1>
        <p class="mt-2 text-sm text-muted">Containers, imagens e sinais vitais coletados pela API restrita, sem montar o socket no plano de controle.</p>
      </div>
      <Button variant="outline" :disabled="query.isFetching.value" @click="query.refetch()"><RefreshCw :class="['h-4 w-4', query.isFetching.value && 'animate-spin']" />Atualizar</Button>
    </header>

    <div v-if="query.isError.value" class="rounded-xl border border-danger/20 bg-danger/5 p-4 text-sm text-danger">O proxy Docker não respondeu. Verifique o endpoint restrito, a identidade mTLS e a lista de permissões.</div>
    <div v-if="action.isError.value" class="rounded-xl border border-danger/20 bg-danger/5 p-4 text-sm text-danger">O daemon rejeitou a operação solicitada.</div>
    <div v-if="query.data.value?.warnings.length" class="rounded-xl border border-warning/20 bg-warning/5 p-4 text-xs text-warning">{{ query.data.value.warnings.join(' · ') }}</div>

    <section class="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
      <article v-for="item in [
        { label: 'Containers', value: containers.length, detail: `${running} em execução`, icon: Container },
        { label: 'Imagens', value: images.length, detail: formatBytes(totalImageBytes), icon: Boxes },
        { label: 'Engine', value: query.data.value?.health.version ?? '—', detail: `API ${query.data.value?.health.api_version ?? 'desconhecida'}`, icon: Database },
        { label: 'Plataforma', value: query.data.value?.health.os_type ?? '—', detail: query.data.value?.health.arch ?? 'desconhecida', icon: Cpu },
      ]" :key="item.label" class="rounded-xl border border-line bg-panel/65 p-5">
        <component :is="item.icon" class="h-4 w-4 text-muted" />
        <p class="mt-5 font-mono text-2xl text-white">{{ item.value }}</p>
        <div class="mt-1 flex items-center justify-between gap-2"><p class="text-xs text-muted">{{ item.label }}</p><p class="font-mono text-[9px] uppercase text-slate-500">{{ item.detail }}</p></div>
      </article>
    </section>

    <section class="overflow-hidden rounded-xl border border-line bg-panel/65">
      <header class="flex flex-col gap-3 border-b border-line p-4 sm:flex-row sm:items-center sm:justify-between">
        <div><h2 class="text-sm font-medium">Inventário do ambiente</h2><p class="mt-1 text-[11px] text-muted">Captura {{ query.data.value ? new Date(query.data.value.captured_at).toLocaleString('pt-BR') : 'aguardando conexão' }}</p></div>
        <nav class="flex rounded-lg border border-line bg-slate-950/40 p-1" aria-label="Inventário Docker">
          <button v-for="item in [{ id: 'containers', label: 'Containers' }, { id: 'images', label: 'Imagens' }]" :key="item.id" :class="['cursor-pointer rounded-md px-3 py-1.5 text-xs transition-colors', tab === item.id ? 'bg-signal/10 text-signal' : 'text-muted hover:text-white']" @click="tab = item.id as typeof tab">{{ item.label }}</button>
        </nav>
      </header>

      <div v-if="query.isLoading.value" class="grid min-h-72 place-items-center"><div class="h-8 w-8 animate-spin rounded-full border-2 border-line border-t-signal" /></div>
      <div v-else-if="tab === 'containers'" class="divide-y divide-line/60">
        <article v-for="container in containers" :key="container.id" class="grid gap-4 px-5 py-4 transition-colors hover:bg-white/[.02] xl:grid-cols-[1.2fr_.7fr_1fr_220px] xl:items-center">
          <div class="flex min-w-0 items-center gap-3"><div class="grid h-10 w-10 shrink-0 place-items-center rounded-xl border border-line bg-slate-950/50"><Container class="h-4 w-4 text-slate-300" /></div><div class="min-w-0"><p class="truncate text-sm text-slate-200">{{ nameOf(container.names) }}</p><p class="mt-1 truncate font-mono text-[10px] text-muted">{{ shortID(container.id) }} · {{ container.image }}</p></div></div>
          <div><StatusBadge :status="container.state === 'running' ? 'healthy' : container.state === 'exited' ? 'critical' : 'warning'" :label="container.state" /><p class="mt-2 truncate text-[10px] text-muted">{{ container.status }}</p></div>
          <div v-if="statsOf(container.id)" class="grid grid-cols-2 gap-x-4 gap-y-2 font-mono text-[10px]"><span class="flex items-center gap-1.5 text-muted"><Cpu class="h-3 w-3" />CPU</span><span class="text-right text-slate-300">{{ statsOf(container.id)?.cpu_percent.toFixed(1) }}%</span><span class="flex items-center gap-1.5 text-muted"><HardDrive class="h-3 w-3" />MEM</span><span class="text-right text-slate-300">{{ formatBytes(statsOf(container.id)?.memory_usage) }}</span><span class="flex items-center gap-1.5 text-muted"><Network class="h-3 w-3" />REDE</span><span class="text-right text-slate-300">{{ formatBytes((statsOf(container.id)?.network_rx ?? 0) + (statsOf(container.id)?.network_tx ?? 0)) }}</span></div><p v-else class="text-xs text-muted">Estatísticas disponíveis quando estiver em execução.</p>
          <div class="flex flex-wrap justify-end gap-2"><Button v-if="container.state === 'running'" variant="outline" @click="openTerminal(container)"><Terminal class="h-3.5 w-3.5"/>Terminal</Button><Button v-if="container.state !== 'running'" variant="outline" :disabled="action.isPending.value" @click="execute(container.id,'start')"><Play class="h-3.5 w-3.5"/>Iniciar</Button><Button v-if="container.state === 'running'" variant="outline" :disabled="action.isPending.value" @click="execute(container.id,'restart')"><RotateCcw class="h-3.5 w-3.5"/>Reiniciar</Button><Button v-if="container.state === 'running'" variant="danger" :disabled="action.isPending.value" @click="execute(container.id,'stop')"><Square class="h-3.5 w-3.5"/>Parar</Button></div>
        </article>
        <div v-if="!containers.length" class="grid min-h-64 place-items-center text-center"><div><Container class="mx-auto h-7 w-7 text-muted" /><p class="mt-3 text-sm text-muted">Nenhum container encontrado.</p></div></div>
      </div>

      <div v-else class="divide-y divide-line/60">
        <article v-for="image in images" :key="image.id" class="grid gap-4 px-5 py-4 transition-colors hover:bg-white/[.02] md:grid-cols-[1fr_140px_160px] md:items-center"><div class="flex min-w-0 items-center gap-3"><div class="grid h-10 w-10 shrink-0 place-items-center rounded-xl border border-line bg-slate-950/50"><Box class="h-4 w-4 text-pulse" /></div><div class="min-w-0"><p class="truncate text-sm text-slate-200">{{ image.repo_tags[0] ?? '&lt;none&gt;:&lt;none&gt;' }}</p><p class="mt-1 font-mono text-[10px] text-muted">{{ shortID(image.id) }}</p></div></div><p class="font-mono text-xs text-slate-300 md:text-right">{{ formatBytes(image.size) }}</p><p class="text-xs text-muted md:text-right">{{ new Date(image.created * 1000).toLocaleDateString('pt-BR') }}</p></article>
        <div v-if="!images.length" class="grid min-h-64 place-items-center text-center"><div><Box class="mx-auto h-7 w-7 text-muted" /><p class="mt-3 text-sm text-muted">Nenhuma imagem encontrada.</p></div></div>
      </div>
    </section>
    <div v-if="terminal" class="fixed inset-0 z-50 grid place-items-center bg-slate-950/85 p-4"><section class="w-full max-w-4xl overflow-hidden rounded-xl border border-line bg-panel"><header class="flex items-center justify-between border-b border-line p-4"><div><h2 class="text-sm text-white">Terminal Docker · {{nameOf(terminal.container.names)}}</h2><p class="mt-1 text-[10px] text-warning">Executa um comando sem TTY dentro do container. Saída limitada a 2 MB.</p></div><Button variant="ghost" @click="terminal=null">Fechar</Button></header><form class="flex gap-2 border-b border-line p-4" @submit.prevent="runCommand"><input v-model="terminal.command" required class="flex-1 rounded-lg border border-line bg-slate-950 p-2 font-mono text-xs" placeholder="comando shell"/><Button type="submit" :disabled="exec.isPending.value"><Terminal class="h-4 w-4"/>Executar</Button></form><pre class="min-h-80 max-h-[55vh] overflow-auto bg-slate-950 p-4 font-mono text-xs leading-5 text-slate-200">{{terminal.output||'A saída aparecerá aqui.'}}</pre></section></div>
  </div>
</template>
