<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, ref } from "vue";
import { useQuery } from "@tanstack/vue-query";
import RFB from "@novnc/novnc/lib/rfb.js";
import { Monitor, PlugZap, RefreshCw, ShieldCheck } from "lucide-vue-next";
import Button from "@/components/ui/Button.vue";
import StatusBadge from "@/components/ui/StatusBadge.vue";
import { apiErrorMessage } from "@/lib/api_error";
import { getVncTargets } from "@/lib/api_vnc";

const targets = useQuery({ queryKey: ["vnc-targets"], queryFn: getVncTargets });
const target = ref("");
const canvas = ref<HTMLElement>();
const state = ref<"disconnected" | "connecting" | "connected" | "failed">("disconnected");
let rfb: RFB | undefined;

const selected = computed(() =>
  targets.data.value?.find((item) => item.id === target.value),
);
const targetsError = computed(() =>
  targets.isError.value
    ? apiErrorMessage(
        targets.error.value,
        "Não foi possível carregar os alvos VNC autorizados.",
      )
    : "",
);
const wsURL = () => {
  const protocol = location.protocol === "https:" ? "wss" : "ws";
  if (!selected.value) return "";
  return `${protocol}://${location.host}${selected.value.ws_path}`;
};
const disconnect = () => {
  rfb?.disconnect();
  rfb = undefined;
  state.value = "disconnected";
};
const connect = async () => {
  if (!canvas.value || !target.value) return;
  disconnect();
  state.value = "connecting";
  await nextTick();
  rfb = new RFB(canvas.value, wsURL(), {
    shared: false,
  });
  rfb.scaleViewport = true;
  rfb.resizeSession = false;
  rfb.addEventListener("connect", () => {
    state.value = "connected";
  });
  rfb.addEventListener("disconnect", (event: Event) => {
    const clean = (event as CustomEvent<{ clean?: boolean }>).detail?.clean;
    state.value = clean ? "disconnected" : "failed";
  });
};
onBeforeUnmount(disconnect);
</script>

<template>
  <div class="mx-auto max-w-[1680px] space-y-5">
    <header class="flex flex-col justify-between gap-4 md:flex-row md:items-end">
      <div>
        <div class="flex gap-2">
          <StatusBadge
            :status="targetsError ? 'critical' : state === 'connected' ? 'healthy' : state === 'failed' ? 'critical' : 'info'"
            :label="targetsError ? 'gateway indisponível' : state"
          />
          <span class="inline-flex items-center gap-1.5 font-mono text-[10px] uppercase text-signal">
            <ShieldCheck class="h-3.5 w-3.5" />ponte noVNC segura
          </span>
        </div>
        <h1 class="mt-4 text-3xl font-semibold">Desktop remoto</h1>
        <p class="mt-2 text-sm text-muted">
          Console gráfico via noVNC. Para VMs Proxmox, o Hub cria o ticket efêmero no servidor: token e ticket nunca chegam ao navegador.
        </p>
      </div>
      <Button variant="outline" @click="targets.refetch"><RefreshCw class="h-4 w-4" />Atualizar alvos</Button>
    </header>

    <div v-if="targetsError" class="rounded-xl border border-danger/30 bg-danger/5 p-4 text-sm text-danger">
      <p class="font-medium">Não foi possível consultar os alvos do gateway VNC.</p>
      <p class="mt-1 break-words text-xs">{{ targetsError }}</p>
      <p class="mt-2 text-xs text-muted">
        Configure o endpoint e a allowlist VNC no serviço backend; enquanto isso o console permanece bloqueado, sem simular uma lista vazia.
      </p>
    </div>

    <section class="rounded-xl border border-line bg-panel/65 p-5">
      <div class="grid gap-3 md:grid-cols-[1fr_auto_auto]">
        <label class="text-xs text-muted">
          VM / alvo VNC
          <select v-model="target" :disabled="Boolean(targetsError)" class="mt-2 w-full rounded-lg border border-line bg-slate-950 p-2 disabled:cursor-not-allowed disabled:opacity-50">
            <option value="" disabled>Selecione um alvo</option>
            <option v-for="item in targets.data.value" :key="item.id" :value="item.id">{{ item.name || item.id }} · {{ item.address }}</option>
          </select>
        </label>
        <Button class="self-end" :disabled="!selected || state === 'connecting' || Boolean(targetsError)" @click="connect"><PlugZap class="h-4 w-4" />Conectar</Button>
        <Button class="self-end" variant="outline" :disabled="state === 'disconnected'" @click="disconnect">Desconectar</Button>
      </div>
    </section>

    <section class="overflow-hidden rounded-xl border border-line bg-black">
      <header class="flex items-center gap-2 border-b border-line bg-panel p-4">
        <Monitor class="h-4 w-4 text-pulse" />
        <span class="text-sm text-slate-200">{{ selected?.id || "Nenhum alvo selecionado" }}</span>
      </header>
      <div ref="canvas" class="min-h-[620px] bg-slate-950"></div>
    </section>
  </div>
</template>
