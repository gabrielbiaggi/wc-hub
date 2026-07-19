<script setup lang="ts">
import { computed, ref } from 'vue'
import { useMutation, useQuery, useQueryClient } from '@tanstack/vue-query'
import {
  Activity,
  Cloud,
  ExternalLink,
  Globe2,
  Network,
  Pencil,
  Plus,
  RefreshCw,
  Route,
  ShieldCheck,
  Trash2,
  Waypoints,
} from 'lucide-vue-next'
import Button from '@/components/ui/Button.vue'
import StatusBadge from '@/components/ui/StatusBadge.vue'
import {
  getCloudflareOverview,
  createCloudflareDNSRecord,
  deleteCloudflareDNSRecord,
  updateCloudflareDNSRecord,
  type CloudflareDNSRecord,
  type CloudflareProviderStatus,
  type CloudflareTunnelStatus,
} from '@/lib/api_cloudflare'

type InventoryMode = 'tunnels' | 'dns'

const mode = ref<InventoryMode>('tunnels')
const queryClient = useQueryClient()
const query = useQuery({
  queryKey: ['cloudflare', 'overview'],
  queryFn: getCloudflareOverview,
  refetchInterval: 30_000,
  retry: 1,
})

const overview = computed(() => query.data.value)
const generatedAt = computed(() => {
  const value = overview.value?.generated_at
  return value ? new Date(value).toLocaleString('pt-BR') : 'Aguardando primeira leitura'
})
const healthRatio = computed(() => {
  const total = overview.value?.summary.tunnels ?? 0
  return total ? Math.round(((overview.value?.summary.healthy_tunnels ?? 0) / total) * 100) : 0
})
const principalRecords = computed(() => overview.value?.dns_records ?? [])
const zoneIDs = computed(() => overview.value?.targets.filter((target) => target.kind === 'zone').map((target) => target.id) ?? [])
const selectedZone = ref('')
const dnsType = ref('A')
const dnsName = ref('')
const dnsContent = ref('')
const dnsProxied = ref(true)
const dnsMutation = useMutation({
  mutationFn: async(input:{operation:'create'|'update'|'delete';zoneID:string;record?:CloudflareDNSRecord;content?:string})=>{
    if(input.operation==='delete'&&input.record)return deleteCloudflareDNSRecord(input.zoneID,input.record.id)
    if(input.operation==='update'&&input.record)return updateCloudflareDNSRecord(input.zoneID,input.record.id,{type:input.record.type,name:input.record.name,content:input.content??input.record.content,proxied:input.record.proxied,ttl:input.record.ttl,comment:input.record.comment})
    return createCloudflareDNSRecord(input.zoneID,{type:dnsType.value,name:dnsName.value,content:dnsContent.value,proxied:dnsProxied.value,ttl:1})
  },
  onSuccess:()=>{dnsName.value='';dnsContent.value='';queryClient.invalidateQueries({queryKey:['cloudflare','overview']})},
})
const zoneFor = (record?:CloudflareDNSRecord) => record?.zone_id || selectedZone.value || zoneIDs.value[0] || ''
const createRecord = () => { const zoneID=zoneFor(); if(zoneID&&dnsName.value&&dnsContent.value) dnsMutation.mutate({operation:'create',zoneID}) }
const editRecord = (record:CloudflareDNSRecord) => { const content=window.prompt(`Novo conteúdo para ${record.name}`,record.content); if(content!==null&&content!==record.content)dnsMutation.mutate({operation:'update',zoneID:zoneFor(record),record,content}) }
const removeRecord = (record:CloudflareDNSRecord) => { if(window.confirm(`Excluir definitivamente ${record.type} ${record.name}?`))dnsMutation.mutate({operation:'delete',zoneID:zoneFor(record),record}) }

const providerTone = (status?: CloudflareProviderStatus) =>
  status === 'healthy' ? 'healthy' : status === 'degraded' ? 'warning' : 'critical'
const tunnelTone = (status: CloudflareTunnelStatus) =>
  status === 'healthy' ? 'healthy' : status === 'degraded' || status === 'inactive' ? 'warning' : status === 'down' ? 'critical' : 'info'
const recordTone = (record: CloudflareDNSRecord) => record.proxied ? 'healthy' : 'info'
const shortID = (value: string) => value.length > 18 ? `${value.slice(0, 8)}…${value.slice(-6)}` : value
</script>

<template>
  <div class="mx-auto max-w-[1680px] space-y-5">
    <header class="flex flex-col justify-between gap-5 lg:flex-row lg:items-end">
      <div>
        <div class="flex flex-wrap items-center gap-2">
          <StatusBadge
            :status="providerTone(overview?.status)"
            :label="overview?.status ?? 'connecting'"
          />
          <span class="inline-flex items-center gap-1.5 font-mono text-[10px] uppercase tracking-wider text-signal">
            <ShieldCheck class="h-3.5 w-3.5" aria-hidden="true" />
            full API token · zone allowlist
          </span>
        </div>
        <h1 class="mt-4 text-3xl font-semibold tracking-tight text-slate-50">Cloudflare edge fabric</h1>
        <p class="mt-2 max-w-3xl text-sm leading-6 text-muted">
          Saúde operacional dos túneis Zero Trust e inventário DNS limitado às accounts e zones autorizadas.
        </p>
      </div>
      <div class="flex flex-col items-start gap-2 sm:flex-row sm:items-center">
        <p class="font-mono text-[9px] uppercase tracking-wider text-muted">Snapshot: {{ generatedAt }}</p>
        <Button
          variant="outline"
          :disabled="query.isFetching.value"
          aria-label="Atualizar inventário Cloudflare"
          @click="query.refetch()"
        >
          <RefreshCw :class="['h-4 w-4', query.isFetching.value && 'animate-spin']" aria-hidden="true" />
          Atualizar
        </Button>
      </div>
    </header>

    <div
      v-if="query.isLoading.value"
      class="grid min-h-72 place-items-center rounded-xl border border-line bg-panel/50"
      role="status"
      aria-live="polite"
    >
      <div class="text-center">
        <div class="mx-auto h-8 w-8 animate-spin rounded-full border-2 border-line border-t-signal" />
        <p class="mt-4 text-sm text-muted">Consultando o edge fabric autorizado…</p>
      </div>
    </div>

    <section
      v-else-if="query.isError.value"
      class="rounded-xl border border-danger/25 bg-danger/[.05] p-6"
      role="alert"
    >
      <div class="flex items-start gap-4">
        <div class="grid h-10 w-10 shrink-0 place-items-center rounded-xl border border-danger/25 bg-danger/[.08]">
          <Activity class="h-5 w-5 text-danger" aria-hidden="true" />
        </div>
        <div class="min-w-0 flex-1">
          <h2 class="text-sm font-medium text-slate-100">Inventário Cloudflare indisponível</h2>
          <p class="mt-2 text-xs leading-5 text-muted">
            Verifique o token com escopo de leitura, a envelope key e as allowlists configuradas no backend.
          </p>
          <Button class="mt-4" variant="outline" @click="query.refetch()">Tentar novamente</Button>
        </div>
      </div>
    </section>

    <template v-else-if="overview">
      <div v-if="dnsMutation.isError.value" class="rounded-xl border border-danger/20 bg-danger/5 p-4 text-sm text-danger">A Cloudflare rejeitou a alteração DNS. Verifique tipo, conteúdo e permissões do token.</div>
      <section class="grid gap-3 sm:grid-cols-2 xl:grid-cols-4" aria-label="Resumo Cloudflare">
        <article class="rounded-xl border border-line bg-panel/65 p-5 shadow-panel">
          <Waypoints class="h-4 w-4 text-muted" aria-hidden="true" />
          <div class="mt-5 flex items-end justify-between gap-3">
            <p class="font-mono text-2xl text-white">{{ overview.summary.tunnels }}</p>
            <span class="font-mono text-[10px] text-signal">{{ healthRatio }}% healthy</span>
          </div>
          <p class="mt-1 text-xs text-muted">Zero Trust tunnels</p>
        </article>
        <article class="rounded-xl border border-line bg-panel/65 p-5 shadow-panel">
          <Globe2 class="h-4 w-4 text-muted" aria-hidden="true" />
          <p class="mt-5 font-mono text-2xl text-white">{{ overview.summary.dns_records }}</p>
          <p class="mt-1 text-xs text-muted">DNS records</p>
        </article>
        <article class="rounded-xl border border-line bg-panel/65 p-5 shadow-panel">
          <Cloud class="h-4 w-4 text-muted" aria-hidden="true" />
          <p class="mt-5 font-mono text-2xl text-white">{{ overview.summary.proxied_records }}</p>
          <p class="mt-1 text-xs text-muted">Cloudflare proxied</p>
        </article>
        <article class="rounded-xl border border-line bg-panel/65 p-5 shadow-panel">
          <Network class="h-4 w-4 text-muted" aria-hidden="true" />
          <p class="mt-5 font-mono text-2xl text-white">
            {{ overview.summary.accounts }}<span class="px-1 text-sm text-muted">/</span>{{ overview.summary.zones }}
          </p>
          <p class="mt-1 text-xs text-muted">Allowlisted accounts / zones</p>
        </article>
      </section>

      <section class="grid gap-5 xl:grid-cols-[minmax(0,1fr)_340px]">
        <article class="min-w-0 overflow-hidden rounded-xl border border-line bg-panel/65">
          <header class="flex flex-col gap-4 border-b border-line px-5 py-4 sm:flex-row sm:items-center sm:justify-between">
            <div>
              <h2 class="text-sm font-medium text-slate-100">Edge inventory</h2>
              <p class="mt-1 text-xs text-muted">Estado retornado diretamente pela API Cloudflare</p>
            </div>
            <div class="flex rounded-lg border border-line bg-slate-950/50 p-1" role="tablist" aria-label="Tipo de inventário">
              <button
                v-for="item in [{ id: 'tunnels', label: 'Tunnels' }, { id: 'dns', label: 'DNS' }]"
                :key="item.id"
                type="button"
                role="tab"
                :aria-selected="mode === item.id"
                :class="[
                  'cursor-pointer rounded-md px-3 py-1.5 text-xs transition-colors duration-200 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-signal',
                  mode === item.id ? 'bg-slate-800 text-slate-100' : 'text-muted hover:text-slate-200',
                ]"
                @click="mode = item.id as InventoryMode"
              >
                {{ item.label }}
              </button>
            </div>
          </header>

          <div v-if="mode === 'tunnels'" class="divide-y divide-line/60" role="tabpanel">
            <article
              v-for="tunnel in overview.tunnels"
              :key="tunnel.id"
              class="grid gap-4 px-5 py-4 transition-colors duration-200 hover:bg-white/[.018] md:grid-cols-[minmax(0,1fr)_150px_140px] md:items-center"
            >
              <div class="flex min-w-0 items-start gap-3">
                <div class="grid h-9 w-9 shrink-0 place-items-center rounded-lg border border-line bg-slate-950/60">
                  <Route class="h-4 w-4 text-pulse" aria-hidden="true" />
                </div>
                <div class="min-w-0">
                  <p class="truncate text-sm font-medium text-slate-200">{{ tunnel.name }}</p>
                  <p class="mt-1 font-mono text-[9px] uppercase tracking-wider text-muted" :title="tunnel.id">
                    {{ shortID(tunnel.id) }} · {{ tunnel.connections.length }} connector{{ tunnel.connections.length === 1 ? '' : 's' }}
                  </p>
                </div>
              </div>
              <StatusBadge :status="tunnelTone(tunnel.status)" :label="tunnel.status" />
              <p class="font-mono text-[10px] text-muted md:text-right">
                {{ tunnel.connections[0]?.colocation || 'NO ACTIVE COLO' }}
              </p>
            </article>
            <div v-if="!overview.tunnels.length" class="grid min-h-56 place-items-center px-5 text-center">
              <div>
                <Waypoints class="mx-auto h-7 w-7 text-muted" aria-hidden="true" />
                <p class="mt-3 text-sm text-muted">Nenhum túnel encontrado nas accounts autorizadas.</p>
              </div>
            </div>
          </div>

          <div v-else role="tabpanel">
            <form class="grid gap-3 border-b border-line bg-slate-950/25 p-4 md:grid-cols-[180px_90px_1fr_1fr_110px]" @submit.prevent="createRecord">
              <select v-model="selectedZone" class="rounded-lg border border-line bg-slate-950 px-3 text-xs text-slate-200"><option value="">Zone padrão</option><option v-for="zone in zoneIDs" :key="zone" :value="zone">{{shortID(zone)}}</option></select>
              <select v-model="dnsType" class="rounded-lg border border-line bg-slate-950 px-3 text-xs text-slate-200"><option v-for="type in ['A','AAAA','CNAME','TXT','MX','CAA']" :key="type">{{type}}</option></select>
              <input v-model.trim="dnsName" required maxlength="255" placeholder="Nome completo" class="rounded-lg border border-line bg-slate-950 px-3 py-2 text-xs text-slate-200"/>
              <input v-model.trim="dnsContent" required maxlength="4096" placeholder="Conteúdo / destino" class="rounded-lg border border-line bg-slate-950 px-3 py-2 text-xs text-slate-200"/>
              <Button type="submit" :disabled="dnsMutation.isPending.value || !zoneFor()"><Plus class="h-3.5 w-3.5"/>Criar</Button>
            </form>
            <div class="overflow-x-auto">
            <table class="w-full min-w-[760px] text-left">
              <thead class="border-b border-line bg-slate-950/30 font-mono text-[9px] uppercase tracking-widest text-muted">
                <tr>
                  <th class="px-5 py-3 font-medium">Record</th>
                  <th class="px-4 py-3 font-medium">Target</th>
                  <th class="px-4 py-3 font-medium">Proxy</th>
                  <th class="px-5 py-3 text-right font-medium">TTL</th>
                  <th class="px-5 py-3 text-right font-medium">Actions</th>
                </tr>
              </thead>
              <tbody class="divide-y divide-line/60">
                <tr v-for="record in principalRecords" :key="record.id" class="transition-colors duration-200 hover:bg-white/[.018]">
                  <td class="px-5 py-4">
                    <div class="flex min-w-0 items-center gap-3">
                      <span class="inline-flex w-14 shrink-0 justify-center rounded-md border border-line bg-slate-950/50 px-2 py-1 font-mono text-[10px] text-pulse">
                        {{ record.type }}
                      </span>
                      <span class="truncate text-xs text-slate-200" :title="record.name">{{ record.name }}</span>
                    </div>
                  </td>
                  <td class="max-w-sm truncate px-4 py-4 font-mono text-[10px] text-muted" :title="record.content">{{ record.content }}</td>
                  <td class="px-4 py-4"><StatusBadge :status="recordTone(record)" :label="record.proxied ? 'proxied' : 'DNS only'" /></td>
                  <td class="px-5 py-4 text-right font-mono text-[10px] text-muted">{{ record.ttl === 1 ? 'AUTO' : `${record.ttl}s` }}</td>
                  <td class="px-5 py-4"><div class="flex justify-end gap-1"><button class="grid h-8 w-8 place-items-center rounded-lg text-muted hover:bg-white/5 hover:text-white" title="Editar conteúdo" @click="editRecord(record)"><Pencil class="h-3.5 w-3.5"/></button><button class="grid h-8 w-8 place-items-center rounded-lg text-danger hover:bg-danger/10" title="Excluir registro" @click="removeRecord(record)"><Trash2 class="h-3.5 w-3.5"/></button></div></td>
                </tr>
              </tbody>
            </table>
            </div>
            <div v-if="!principalRecords.length" class="grid min-h-56 place-items-center px-5 text-center">
              <div>
                <Globe2 class="mx-auto h-7 w-7 text-muted" aria-hidden="true" />
                <p class="mt-3 text-sm text-muted">Nenhum apontamento DNS principal foi encontrado.</p>
              </div>
            </div>
          </div>
        </article>

        <aside class="space-y-4">
          <article class="rounded-xl border border-line bg-panel/55 p-5">
            <div class="flex items-center gap-3">
              <Activity class="h-4 w-4 text-signal" aria-hidden="true" />
              <h2 class="text-sm font-medium text-slate-100">Provider health</h2>
            </div>
            <div class="mt-4 space-y-3">
              <div
                v-for="target in overview.targets"
                :key="`${target.kind}-${target.id}`"
                class="rounded-lg border border-line bg-slate-950/35 p-3"
              >
                <div class="flex items-center justify-between gap-3">
                  <p class="font-mono text-[9px] uppercase tracking-wider text-muted">{{ target.kind }}</p>
                  <StatusBadge :status="target.status === 'healthy' ? 'healthy' : 'critical'" :label="target.status" />
                </div>
                <p class="mt-3 truncate font-mono text-[10px] text-slate-300" :title="target.id">{{ shortID(target.id) }}</p>
                <p class="mt-1 text-[11px] leading-5 text-muted">{{ target.message }} {{ target.item_count }} items.</p>
              </div>
            </div>
          </article>

          <article class="rounded-xl border border-pulse/20 bg-pulse/[.035] p-5">
            <div class="flex items-center gap-3">
              <ExternalLink class="h-4 w-4 text-pulse" aria-hidden="true" />
              <h2 class="text-sm font-medium text-slate-100">Security boundary</h2>
            </div>
            <p class="mt-3 text-xs leading-5 text-muted">
              O token administrativo é decriptado somente durante cada chamada. Toda account ou zone fora da allowlist continua recusada antes da API externa, inclusive nas mutações.
            </p>
          </article>
        </aside>
      </section>
    </template>
  </div>
</template>
