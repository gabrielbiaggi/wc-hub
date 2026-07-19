<script setup lang="ts">
import { computed, ref } from "vue";
import { useMutation, useQuery, useQueryClient } from "@tanstack/vue-query";
import {
  BatteryCharging,
  Cpu,
  Power,
  RefreshCw,
  Send,
  Timer,
} from "lucide-vue-next";
import Button from "@/components/ui/Button.vue";
import StatusBadge from "@/components/ui/StatusBadge.vue";
import { apiErrorMessage } from "@/lib/api_error";
import { getPowerStatus, getWakeTargets, wakeTarget } from "@/lib/api_power";

const client = useQueryClient();
const status = useQuery({
  queryKey: ["power-status"],
  queryFn: getPowerStatus,
  refetchInterval: 30000,
});
const targets = useQuery({
  queryKey: ["power-targets"],
  queryFn: getWakeTargets,
});
const macAddress = ref("");
const selected = computed(() =>
  targets.data.value?.find(
    (item) => item.mac.toLowerCase() === macAddress.value.trim().toLowerCase(),
  ),
);
const adapterError = computed(() => {
  if (status.isError.value)
    return apiErrorMessage(
      status.error.value,
      "Não foi possível consultar o servidor NUT.",
    );
  if (targets.isError.value)
    return apiErrorMessage(
      targets.error.value,
      "Não foi possível carregar os alvos Wake-on-LAN.",
    );
  return "";
});
const wake = useMutation({
  mutationFn: () => wakeTarget(selected.value?.id ?? ""),
  onSuccess: () => client.invalidateQueries({ queryKey: ["power-status"] }),
});
const submitWake = () => {
  if (selected.value) wake.mutate();
};
const percent = computed(() =>
  Math.max(0, Math.min(100, status.data.value?.batteryPercent ?? 0)),
);
const duration = computed(() => {
  const seconds = status.data.value?.runtimeSeconds;
  if (seconds === undefined) return "—";
  return `${Math.floor(seconds / 3600)}h ${Math.floor((seconds % 3600) / 60)}min`;
});
</script>

<template>
  <div class="mx-auto max-w-[1280px] space-y-5">
    <header
      class="flex flex-col justify-between gap-4 md:flex-row md:items-end"
    >
      <div>
        <div class="flex gap-2">
          <StatusBadge
            :status="adapterError ? 'critical' : status.data.value?.online ? 'healthy' : 'warning'"
            :label="
              adapterError ? 'adaptador indisponível' : status.data.value?.online ? 'NUT online' : 'NUT indisponível'
            "
          />
          <span
            class="inline-flex items-center gap-1 font-mono text-[10px] uppercase text-signal"
            ><Power class="h-3.5 w-3.5" />energia protegida</span
          >
        </div>
        <h1 class="mt-4 text-3xl font-semibold">Energia e Wake-on-LAN</h1>
        <p class="mt-2 text-sm text-muted">
          Status do no-break via NUT e partida remota somente para máquinas
          autorizadas.
        </p>
      </div>
      <Button
        variant="outline"
        @click="
          status.refetch();
          targets.refetch();
        "
        ><RefreshCw class="h-4 w-4" />Atualizar</Button
      >
    </header>

    <p
      v-if="adapterError"
      class="rounded-xl border border-danger/30 bg-danger/5 p-4 text-sm text-danger"
    >
      <p class="font-medium">O adaptador de energia não respondeu.</p>
      <p class="mt-1 break-words text-xs">{{ adapterError }}</p>
      <p class="mt-2 text-xs text-muted">
        Defina NUT_SERVER, NUT_UPS_NAME e WOL_ALLOWED_TARGETS no ambiente antes de usar os comandos de energia.
      </p>
    </p>

    <section class="grid gap-5 lg:grid-cols-[1fr_360px]">
      <article class="rounded-xl border border-line bg-panel/65 p-6">
        <div class="flex items-center justify-between">
          <div>
            <p class="text-sm text-muted">Bateria do no-break</p>
            <p class="mt-2 font-mono text-5xl text-slate-100">
              {{ status.data.value?.batteryPercent ?? "—"
              }}<span class="text-xl text-muted">%</span>
            </p>
          </div>
          <BatteryCharging class="h-12 w-12 text-signal" />
        </div>
        <div class="mt-7 h-4 overflow-hidden rounded-full bg-slate-950">
          <div
            class="h-full rounded-full bg-gradient-to-r from-signal to-pulse transition-all"
            :style="{ width: `${percent}%` }"
          ></div>
        </div>
        <div class="mt-6 grid gap-4 sm:grid-cols-3">
          <div>
            <p class="flex items-center gap-1 text-[10px] uppercase text-muted">
              <Cpu class="h-3 w-3" />Carga
            </p>
            <p class="mt-1 font-mono text-lg">
              {{ status.data.value?.loadPercent ?? "—" }}%
            </p>
          </div>
          <div>
            <p class="flex items-center gap-1 text-[10px] uppercase text-muted">
              <Timer class="h-3 w-3" />Autonomia
            </p>
            <p class="mt-1 font-mono text-lg">{{ duration }}</p>
          </div>
          <div>
            <p class="text-[10px] uppercase text-muted">Estado UPS</p>
            <p class="mt-1 font-mono text-lg">
              {{ status.data.value?.upsStatus || "—" }}
            </p>
          </div>
        </div>
        <p v-if="status.data.value?.error" class="mt-6 text-xs text-warning">
          {{ status.data.value.error }}
        </p>
      </article>

      <form
        class="rounded-xl border border-line bg-panel/65 p-5"
        @submit.prevent="submitWake"
      >
        <h2 class="flex items-center gap-2 text-sm">
          <Send class="h-4 w-4 text-pulse" />Wake-on-LAN
        </h2>
        <p class="mt-2 text-xs text-muted">
          Informe uma MAC previamente autorizada. O pacote mágico é auditado.
        </p>
        <label class="mt-5 block text-xs text-muted"
          >MAC address
          <input
            v-model="macAddress"
            list="wol-targets"
            required
            placeholder="aa:bb:cc:dd:ee:ff"
            class="mt-2 w-full rounded-lg border border-line bg-slate-950 p-2 font-mono text-xs text-slate-100"
          />
          <datalist id="wol-targets">
            <option
              v-for="item in targets.data.value"
              :key="item.id"
              :value="item.mac"
            >
              {{ item.id }}
            </option>
          </datalist>
        </label>
        <p v-if="macAddress && !selected" class="mt-2 text-[11px] text-warning">
          A MAC precisa corresponder a um alvo autorizado.
        </p>
        <p v-else-if="selected" class="mt-2 text-[11px] text-muted">
          Alvo: {{ selected.id }}
        </p>
        <Button
          class="mt-5 w-full"
          type="submit"
          :disabled="!selected || wake.isPending.value"
          ><Power class="h-4 w-4" />{{
            wake.isPending.value ? "Enviando..." : "Ligar máquina"
          }}</Button
        >
        <p v-if="wake.isSuccess.value" class="mt-3 text-xs text-signal">
          Magic packet enviado para {{ selected?.id }}.
        </p>
        <p v-if="wake.isError.value" class="mt-3 text-xs text-danger">
          Não foi possível enviar o magic packet.
        </p>
      </form>
    </section>
  </div>
</template>
