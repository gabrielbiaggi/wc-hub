<script setup lang="ts">
import { computed } from "vue";
import { useQuery } from "@tanstack/vue-query";
import { Activity, Cpu, HardDrive, MemoryStick, Network, RefreshCw, Thermometer, Zap } from "lucide-vue-next";
import Button from "@/components/ui/Button.vue";
import StatusBadge from "@/components/ui/StatusBadge.vue";
import { getHostTelemetry } from "@/lib/api";

type HostSnapshot = { id: string; name: string; at: string; metrics: Record<string, number> };
const query = useQuery({ queryKey: ["host-telemetry"], queryFn: getHostTelemetry, refetchInterval: 10_000 });
const hosts = computed<HostSnapshot[]>(() => {
  const grouped = new Map<string, HostSnapshot>();
  for (const metric of query.data.value ?? []) {
    const item = grouped.get(metric.host_id) ?? { id: metric.host_id, name: metric.host_name, at: metric.captured_at, metrics: {} };
    item.metrics[metric.metric] = metric.value;
    if (metric.captured_at > item.at) item.at = metric.captured_at;
    grouped.set(metric.host_id, item);
  }
  return [...grouped.values()].sort((a, b) => a.name.localeCompare(b.name, "pt-BR"));
});
const gib = (value = 0) => value / 1073741824;
const number = (value = 0, digits = 1) => value.toLocaleString("pt-BR", { maximumFractionDigits: digits });
const percent = (part = 0, total = 0) => total > 0 ? Math.max(0, Math.min(100, (part / total) * 100)) : 0;
const ageSeconds = (at: string) => Math.max(0, (Date.now() - new Date(at).getTime()) / 1000);
const hostStatus = (host: HostSnapshot) => ageSeconds(host.at) <= 45 ? "healthy" : ageSeconds(host.at) <= 120 ? "warning" : "critical";
const statusLabel = (host: HostSnapshot) => hostStatus(host) === "healthy" ? "transmitindo" : hostStatus(host) === "warning" ? "atrasado" : "offline";
const filesystemUsed = (host: HostSnapshot) => {
  const total = host.metrics.node_filesystem_size_bytes ?? 0;
  return Math.max(0, total - (host.metrics.node_filesystem_avail_bytes ?? 0));
};
const energyWatts = (host: HostSnapshot) => host.metrics.node_rapl_package_joules_total ?? host.metrics.node_hwmon_power_average_watt ?? host.metrics.node_hwmon_power_input_watt;
</script>

<template>
  <div class="mx-auto max-w-[1680px] space-y-5">
    <header class="flex flex-col justify-between gap-4 md:flex-row md:items-end">
      <div>
        <div class="flex items-center gap-2 font-mono text-[10px] uppercase tracking-widest text-signal"><Activity class="h-3.5 w-3.5" /> agentes autenticados</div>
        <h1 class="mt-3 text-3xl font-semibold tracking-tight">Telemetria da infraestrutura</h1>
        <p class="mt-2 text-sm text-muted">Genesys, AI e Hub local. Atualização automática a cada 10 segundos.</p>
      </div>
      <Button variant="outline" :disabled="query.isFetching.value" @click="query.refetch()"><RefreshCw class="h-4 w-4" />Atualizar</Button>
    </header>

    <div v-if="query.isError.value" class="rounded-xl border border-danger/30 bg-danger/5 p-4 text-sm text-danger">Não foi possível obter telemetria. Verifique o agente e a conectividade com o Hub.</div>

    <section class="grid gap-4 xl:grid-cols-3">
      <article v-for="host in hosts" :key="host.id" class="overflow-hidden rounded-xl border border-line bg-panel/65">
        <header class="flex items-start gap-3 border-b border-line p-5">
          <div class="grid h-10 w-10 place-items-center rounded-lg bg-signal/10 text-signal"><Cpu class="h-5 w-5" /></div>
          <div class="min-w-0"><h2 class="truncate text-base font-medium">{{ host.name }}</h2><p class="mt-1 font-mono text-[10px] text-muted">{{ new Date(host.at).toLocaleString("pt-BR") }} · há {{ number(ageSeconds(host.at), 0) }} s</p></div>
          <StatusBadge class="ml-auto" :status="hostStatus(host)" :label="statusLabel(host)" />
        </header>
        <div class="grid grid-cols-2 gap-px bg-line">
          <div class="bg-panel p-4"><Activity class="h-3.5 w-3.5 text-muted" /><p class="mt-3 text-lg text-white">{{ number(host.metrics.node_load1, 2) }}</p><p class="mt-1 text-[10px] text-muted">Carga de 1 min</p></div>
          <div class="bg-panel p-4"><MemoryStick class="h-3.5 w-3.5 text-muted" /><p class="mt-3 text-lg text-white">{{ number(gib(host.metrics.node_memory_MemAvailable_bytes)) }} / {{ number(gib(host.metrics.node_memory_MemTotal_bytes)) }} GiB</p><p class="mt-1 text-[10px] text-muted">Memória disponível / total</p></div>
          <div class="bg-panel p-4"><HardDrive class="h-3.5 w-3.5 text-muted" /><p class="mt-3 text-lg text-white">{{ number(percent(filesystemUsed(host), host.metrics.node_filesystem_size_bytes), 0) }}%</p><p class="mt-1 text-[10px] text-muted">Uso do sistema de arquivos</p></div>
          <div class="bg-panel p-4"><Network class="h-3.5 w-3.5 text-muted" /><p class="mt-3 text-lg text-white">↓ {{ number(gib(host.metrics.node_network_receive_bytes_total)) }} · ↑ {{ number(gib(host.metrics.node_network_transmit_bytes_total)) }} GiB</p><p class="mt-1 text-[10px] text-muted">Tráfego acumulado</p></div>
          <div class="bg-panel p-4"><Thermometer class="h-3.5 w-3.5 text-muted" /><p class="mt-3 text-lg text-white">{{ host.metrics.DCGM_FI_DEV_GPU_TEMP === undefined ? "—" : number(host.metrics.DCGM_FI_DEV_GPU_TEMP, 0) + " °C" }}</p><p class="mt-1 text-[10px] text-muted">Temperatura GPU</p></div>
          <div class="bg-panel p-4"><Zap class="h-3.5 w-3.5 text-muted" /><p class="mt-3 text-lg text-white">{{ energyWatts(host) === undefined ? "sensor indisponível" : number(energyWatts(host), 1) }}</p><p class="mt-1 text-[10px] text-muted">Energia / contador RAPL</p></div>
        </div>
      </article>
    </section>

    <div v-if="!hosts.length && !query.isLoading.value" class="grid min-h-80 place-items-center rounded-xl border border-dashed border-line bg-panel/30 text-center"><div><Activity class="mx-auto h-7 w-7 text-muted" /><p class="mt-3 text-sm text-muted">Nenhum agente enviou métricas ainda.</p></div></div>
  </div>
</template>
