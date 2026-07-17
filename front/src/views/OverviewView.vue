<script setup lang="ts">
import { useQuery } from '@tanstack/vue-query'
import { ArrowDownRight, ArrowUpRight, Cpu, Database, Layers3, RefreshCw, ShieldCheck, TriangleAlert, Zap } from 'lucide-vue-next'
import { getOverview } from '@/lib/api'
import Button from '@/components/ui/Button.vue'
import StatusBadge from '@/components/ui/StatusBadge.vue'
import TelemetryChart from '@/components/charts/TelemetryChart.vue'
const query = useQuery({ queryKey: ['overview'], queryFn: getOverview, refetchInterval: 30_000 })
const icons = [Cpu, Layers3, Database, TriangleAlert]
const fallback = { generated_at:new Date().toISOString(), environment:'connecting', self_protected:true, metrics:[], activity:[], series:[30,34,32,39,42,47,45,51,49,56,60,58] }
</script>

<template>
  <div class="mx-auto max-w-[1680px] space-y-5">
    <section class="flex flex-col justify-between gap-4 xl:flex-row xl:items-end">
      <div><div class="mb-3 flex flex-wrap items-center gap-2"><StatusBadge status="healthy" label="System operational" /><StatusBadge status="info" :label="(query.data.value ?? fallback).environment" /><span class="inline-flex items-center gap-1.5 font-mono text-[10px] uppercase tracking-wider text-signal"><ShieldCheck class="h-3.5 w-3.5" />self-protected</span></div><h1 class="text-2xl font-semibold tracking-tight text-white md:text-3xl">Operations overview</h1><p class="mt-1 max-w-2xl text-sm text-muted">Sinais vitais, capacidade e eventos críticos em um único plano de controle.</p></div>
      <div class="flex items-center gap-2"><p class="hidden font-mono text-[10px] text-muted sm:block">SYNC {{ new Date((query.data.value ?? fallback).generated_at).toLocaleTimeString('pt-BR') }}</p><Button variant="outline" :disabled="query.isFetching.value" @click="query.refetch()"><RefreshCw :class="['h-3.5 w-3.5', query.isFetching.value && 'animate-spin']" />Atualizar</Button></div>
    </section>

    <div v-if="query.isError.value" class="rounded-xl border border-danger/20 bg-danger/5 px-4 py-3 text-sm text-danger">A API ainda não respondeu. O painel usa estado de conexão até o backend iniciar.</div>

    <section class="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
      <article v-for="(metric,index) in (query.data.value ?? fallback).metrics" :key="metric.label" class="group relative overflow-hidden rounded-xl border border-line bg-panel/70 p-4 shadow-panel transition-colors hover:border-slate-600/80">
        <div class="absolute -right-8 -top-8 h-28 w-28 rounded-full bg-pulse/[.035] blur-2xl transition-colors group-hover:bg-signal/[.06]" /><div class="relative flex items-start justify-between"><div class="grid h-9 w-9 place-items-center rounded-lg border border-line bg-slate-950/60"><component :is="icons[index]" class="h-4 w-4 text-slate-300" /></div><StatusBadge :status="metric.status" /></div><div class="relative mt-5 flex items-end gap-2"><strong class="font-mono text-3xl font-medium tracking-tight text-white">{{metric.value}}</strong><span class="mb-1 text-xs text-muted">{{metric.unit}}</span><span :class="['ml-auto mb-1 flex items-center text-[11px]', metric.delta < 0 ? 'text-signal':'text-pulse']"><ArrowDownRight v-if="metric.delta < 0" class="h-3 w-3"/><ArrowUpRight v-else class="h-3 w-3"/>{{Math.abs(metric.delta)}}%</span></div><p class="relative mt-2 text-xs text-muted">{{metric.label}}</p>
      </article>
      <article v-if="!(query.data.value ?? fallback).metrics.length" v-for="n in 4" :key="n" class="h-40 animate-pulse rounded-xl border border-line bg-panel/60" />
    </section>

    <section class="grid gap-5 xl:grid-cols-[minmax(0,1.7fr)_minmax(340px,.8fr)]">
      <article class="overflow-hidden rounded-xl border border-line bg-panel/65 shadow-panel"><header class="flex items-start justify-between border-b border-line/70 px-5 py-4"><div><div class="flex items-center gap-2"><Zap class="h-4 w-4 text-signal"/><h2 class="text-sm font-medium">Aggregate workload</h2></div><p class="mt-1 text-xs text-muted">Compute pressure · last 24 hours</p></div><div class="text-right"><p class="font-mono text-xl text-white">76.4%</p><p class="text-[10px] text-signal">within envelope</p></div></header><div class="h-[290px] px-3 pb-2 pt-4"><TelemetryChart :values="(query.data.value ?? fallback).series" /></div><footer class="grid grid-cols-3 border-t border-line/70"><div v-for="item in [{k:'CPU PEAK',v:'82%'},{k:'MEMORY',v:'61%'},{k:'I/O WAIT',v:'2.8%'}]" :key="item.k" class="border-r border-line/70 px-5 py-3 last:border-0"><p class="font-mono text-[9px] tracking-widest text-muted">{{item.k}}</p><p class="mt-1 font-mono text-sm text-slate-200">{{item.v}}</p></div></footer></article>
      <article class="rounded-xl border border-line bg-panel/65 shadow-panel"><header class="flex items-center justify-between border-b border-line/70 px-5 py-4"><div><h2 class="text-sm font-medium">Live activity</h2><p class="mt-1 text-xs text-muted">Audit-ready event stream</p></div><span class="h-2 w-2 animate-pulse rounded-full bg-signal shadow-[0_0_10px_#49e29d]" /></header><div class="divide-y divide-line/60 px-5"><div v-for="event in (query.data.value ?? fallback).activity" :key="event.id" class="flex gap-3 py-4"><span :class="['mt-1.5 h-2 w-2 shrink-0 rounded-full',event.severity==='success'?'bg-signal':'bg-pulse']"/><div class="min-w-0"><p class="text-xs leading-relaxed text-slate-300">{{event.message}}</p><div class="mt-1.5 flex gap-2 font-mono text-[9px] uppercase tracking-wider text-muted"><span>{{event.source}}</span><span>·</span><time>{{new Date(event.at).toLocaleTimeString('pt-BR',{hour:'2-digit',minute:'2-digit'})}}</time></div></div></div></div><RouterLink to="/audit" class="m-3 flex cursor-pointer items-center justify-center rounded-lg border border-line py-2.5 text-xs text-muted transition-colors hover:border-slate-600 hover:text-white">View complete audit trail</RouterLink></article>
    </section>

    <section class="grid gap-3 md:grid-cols-3"><article v-for="system in [{name:'Proxmox cluster',meta:'4 nodes · 21 VMs',value:'99.98%'},{name:'K3s fabric',meta:'3 nodes · 84 pods',value:'Healthy'},{name:'Storage fabric',meta:'MergerFS · 18.2 TB',value:'68%'}]" :key="system.name" class="flex items-center gap-4 rounded-xl border border-line bg-panel/50 p-4"><div class="grid h-10 w-10 place-items-center rounded-full border border-signal/15 bg-signal/[.06]"><span class="h-2 w-2 rounded-full bg-signal shadow-[0_0_9px_#49e29d]" /></div><div class="min-w-0 flex-1"><p class="text-sm text-slate-200">{{system.name}}</p><p class="mt-0.5 text-[11px] text-muted">{{system.meta}}</p></div><span class="font-mono text-xs text-slate-300">{{system.value}}</span></article></section>
  </div>
</template>

