<script setup lang="ts">
import { computed } from "vue";
import { useQuery } from "@tanstack/vue-query";
import {
  ArrowDownRight,
  ArrowUpRight,
  Cpu,
  Database,
  Layers3,
  RefreshCw,
  ShieldCheck,
  TriangleAlert,
  Zap,
} from "lucide-vue-next";
import { getModules, getOverview } from "@/lib/api";
import Button from "@/components/ui/Button.vue";
import StatusBadge from "@/components/ui/StatusBadge.vue";
import TelemetryChart from "@/components/charts/TelemetryChart.vue";
import { traduzirMetrica, traduzirTexto } from "@/lib/ptbr";
const query = useQuery({
  queryKey: ["overview"],
  queryFn: getOverview,
  refetchInterval: 30_000,
});
const modules = useQuery({
  queryKey: ["modules"],
  queryFn: getModules,
  staleTime: 60_000,
});
const icons = [Cpu, Layers3, Database, TriangleAlert];
const fallback = {
  generated_at: new Date().toISOString(),
  environment: "connecting",
  self_protected: true,
  metrics: [],
  activity: [],
  series: [] as number[],
};
const snapshot = computed(() => query.data.value ?? fallback);
const seriesStats = computed(() => {
  const values = snapshot.value.series;
  const latest = values.at(-1) ?? 0;
  const peak = values.length ? Math.max(...values) : 0;
  const average = values.length
    ? values.reduce((sum, value) => sum + value, 0) / values.length
    : 0;
  return { latest, peak, average, count: values.length };
});
const systemCards = computed(() =>
  snapshot.value.metrics
    .slice(0, 3)
    .map((metric) => ({
      name: traduzirMetrica(metric.label),
      meta: `${metric.value} ${traduzirMetrica(metric.unit)}`,
      value: traduzirTexto(metric.status),
    })),
);
</script>

<template>
  <div class="mx-auto max-w-[1680px] space-y-5">
    <section
      class="flex flex-col justify-between gap-4 xl:flex-row xl:items-end"
    >
      <div>
        <div class="mb-3 flex flex-wrap items-center gap-2">
          <StatusBadge
            status="healthy"
            label="Sistema operacional"
          /><StatusBadge
            status="info"
            :label="(query.data.value ?? fallback).environment"
          /><StatusBadge
            v-if="modules.data.value"
            status="info"
            :label="`${modules.data.value.length} módulos API`"
          /><span
            class="inline-flex items-center gap-1.5 font-mono text-[10px] uppercase tracking-wider text-signal"
            ><ShieldCheck class="h-3.5 w-3.5" />autoprotegido</span
          >
        </div>
        <h1
          class="text-2xl font-semibold tracking-tight text-white md:text-3xl"
        >
          Visão geral das operações
        </h1>
        <p class="mt-1 max-w-2xl text-sm text-muted">
          Sinais vitais, capacidade e eventos críticos em um único plano de
          controle.
        </p>
      </div>
      <div class="flex items-center gap-2">
        <p class="hidden font-mono text-[10px] text-muted sm:block">
          SINCRONIZADO
          {{
            new Date(
              (query.data.value ?? fallback).generated_at,
            ).toLocaleTimeString("pt-BR")
          }}
        </p>
        <Button
          variant="outline"
          :disabled="query.isFetching.value"
          @click="query.refetch()"
          ><RefreshCw
            :class="['h-3.5 w-3.5', query.isFetching.value && 'animate-spin']"
          />Atualizar</Button
        >
      </div>
    </section>

    <div
      v-if="query.isError.value"
      class="rounded-xl border border-danger/20 bg-danger/5 px-4 py-3 text-sm text-danger"
    >
      A API ainda não respondeu. O painel usa estado de conexão até o backend
      iniciar.
    </div>

    <section class="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
      <article
        v-for="(metric, index) in (query.data.value ?? fallback).metrics"
        :key="metric.label"
        class="group relative overflow-hidden rounded-xl border border-line bg-panel/70 p-4 shadow-panel transition-colors hover:border-slate-600/80"
      >
        <div
          class="absolute -right-8 -top-8 h-28 w-28 rounded-full bg-pulse/[.035] blur-2xl transition-colors group-hover:bg-signal/[.06]"
        />
        <div class="relative flex items-start justify-between">
          <div
            class="grid h-9 w-9 place-items-center rounded-lg border border-line bg-slate-950/60"
          >
            <component :is="icons[index]" class="h-4 w-4 text-slate-300" />
          </div>
          <StatusBadge :status="metric.status" />
        </div>
        <div class="relative mt-5 flex items-end gap-2">
          <strong
            class="font-mono text-3xl font-medium tracking-tight text-white"
            >{{ metric.value }}</strong
          ><span class="mb-1 text-xs text-muted">{{
            traduzirMetrica(metric.unit)
          }}</span
          ><span
            :class="[
              'ml-auto mb-1 flex items-center text-[11px]',
              metric.delta < 0 ? 'text-signal' : 'text-pulse',
            ]"
            ><ArrowDownRight
              v-if="metric.delta < 0"
              class="h-3 w-3"
            /><ArrowUpRight v-else class="h-3 w-3" />{{
              Math.abs(metric.delta)
            }}%</span
          >
        </div>
        <p class="relative mt-2 text-xs text-muted">
          {{ traduzirMetrica(metric.label) }}
        </p>
      </article>
      <article
        v-if="!(query.data.value ?? fallback).metrics.length"
        v-for="n in 4"
        :key="n"
        class="h-40 animate-pulse rounded-xl border border-line bg-panel/60"
      />
    </section>

    <section
      class="grid gap-5 xl:grid-cols-[minmax(0,1.7fr)_minmax(340px,.8fr)]"
    >
      <article
        class="overflow-hidden rounded-xl border border-line bg-panel/65 shadow-panel"
      >
        <header
          class="flex items-start justify-between border-b border-line/70 px-5 py-4"
        >
          <div>
            <div class="flex items-center gap-2">
              <Zap class="h-4 w-4 text-signal" />
              <h2 class="text-sm font-medium">Telemetria agregada</h2>
            </div>
            <p class="mt-1 text-xs text-muted">
              Amostras persistidas · últimas 24 horas
            </p>
          </div>
          <div class="text-right">
            <p class="font-mono text-xl text-white">
              {{ seriesStats.latest.toFixed(2) }}
            </p>
            <p class="text-[10px] text-signal">último valor medido</p>
          </div>
        </header>
        <div class="h-[290px] px-3 pb-2 pt-4">
          <TelemetryChart :values="snapshot.series" />
        </div>
        <footer class="grid grid-cols-3 border-t border-line/70">
          <div
            v-for="item in [
              { k: 'PICO', v: seriesStats.peak.toFixed(2) },
              { k: 'MÉDIA', v: seriesStats.average.toFixed(2) },
              { k: 'PONTOS', v: String(seriesStats.count) },
            ]"
            :key="item.k"
            class="border-r border-line/70 px-5 py-3 last:border-0"
          >
            <p class="font-mono text-[9px] tracking-widest text-muted">
              {{ item.k }}
            </p>
            <p class="mt-1 font-mono text-sm text-slate-200">{{ item.v }}</p>
          </div>
        </footer>
      </article>
      <article class="rounded-xl border border-line bg-panel/65 shadow-panel">
        <header
          class="flex items-center justify-between border-b border-line/70 px-5 py-4"
        >
          <div>
            <h2 class="text-sm font-medium">Atividade ao vivo</h2>
            <p class="mt-1 text-xs text-muted">
              Fluxo de eventos pronto para auditoria
            </p>
          </div>
          <span
            class="h-2 w-2 animate-pulse rounded-full bg-signal shadow-[0_0_10px_#49e29d]"
          />
        </header>
        <div class="divide-y divide-line/60 px-5">
          <div
            v-for="event in (query.data.value ?? fallback).activity"
            :key="event.id"
            class="flex gap-3 py-4"
          >
            <span
              :class="[
                'mt-1.5 h-2 w-2 shrink-0 rounded-full',
                event.severity === 'success' ? 'bg-signal' : 'bg-pulse',
              ]"
            />
            <div class="min-w-0">
              <p class="text-xs leading-relaxed text-slate-300">
                {{ event.message }}
              </p>
              <div
                class="mt-1.5 flex gap-2 font-mono text-[9px] uppercase tracking-wider text-muted"
              >
                <span>{{ event.source }}</span
                ><span>·</span
                ><time>{{
                  new Date(event.at).toLocaleTimeString("pt-BR", {
                    hour: "2-digit",
                    minute: "2-digit",
                  })
                }}</time>
              </div>
            </div>
          </div>
        </div>
        <RouterLink
          to="/audit"
          class="m-3 flex cursor-pointer items-center justify-center rounded-lg border border-line py-2.5 text-xs text-muted transition-colors hover:border-slate-600 hover:text-white"
          >Ver trilha de auditoria completa</RouterLink
        >
      </article>
    </section>

    <section class="grid gap-3 md:grid-cols-3">
      <article
        v-for="system in systemCards"
        :key="system.name"
        class="flex items-center gap-4 rounded-xl border border-line bg-panel/50 p-4"
      >
        <div
          class="grid h-10 w-10 place-items-center rounded-full border border-signal/15 bg-signal/[.06]"
        >
          <span
            class="h-2 w-2 rounded-full bg-signal shadow-[0_0_9px_#49e29d]"
          />
        </div>
        <div class="min-w-0 flex-1">
          <p class="text-sm text-slate-200">{{ system.name }}</p>
          <p class="mt-0.5 text-[11px] text-muted">{{ system.meta }}</p>
        </div>
        <span class="font-mono text-xs uppercase text-slate-300">{{
          system.value
        }}</span>
      </article>
    </section>
  </div>
</template>
