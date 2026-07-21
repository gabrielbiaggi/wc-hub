<script setup lang="ts">
import { ref } from "vue";
import { useMutation, useQuery, useQueryClient } from "@tanstack/vue-query";
import { Activity, Link, Pencil, Plus, RefreshCw, Trash2 } from "lucide-vue-next";
import Button from "@/components/ui/Button.vue";
import StatusBadge from "@/components/ui/StatusBadge.vue";
import {
  createMonitorTarget,
  deleteMonitorTarget,
  getMonitorTargets,
  getMonitorWebhook,
  setMonitorWebhook,
  updateMonitorTarget,
  type MonitorTarget,
} from "@/lib/api_monitor";
const client = useQueryClient();
const targets = useQuery({
  queryKey: ["monitor-targets"],
  queryFn: getMonitorTargets,
  refetchInterval: 15000,
});
const webhook = useQuery({
  queryKey: ["monitor-webhook"],
  queryFn: getMonitorWebhook,
});
const form = ref({
  name: "",
  target: "",
  kind: "http" as "http" | "tcp",
  intervalSeconds: 60,
  enabled: true,
});
const hook = ref("");
const create = useMutation({
  mutationFn: () => createMonitorTarget(form.value),
  onSuccess: () => {
    form.value = {
      name: "",
      target: "",
      kind: "http",
      intervalSeconds: 60,
      enabled: true,
    };
    client.invalidateQueries({ queryKey: ["monitor-targets"] });
  },
});
const remove = useMutation({
  mutationFn: deleteMonitorTarget,
  onSuccess: () => client.invalidateQueries({ queryKey: ["monitor-targets"] }),
});
const update = useMutation({
  mutationFn: (input: { id:string; target: Omit<MonitorTarget, "id" | "lastStatus" | "lastLatencyMS" | "lastError" | "lastCheckedAt"> }) => updateMonitorTarget(input.id, input.target),
  onSuccess: () => client.invalidateQueries({ queryKey: ["monitor-targets"] }),
});
const saveHook = useMutation({
  mutationFn: () => setMonitorWebhook(hook.value),
  onSuccess: () => {
    hook.value = "";
    webhook.refetch();
  },
});
const submitCreate = () => create.mutate();
const submitWebhook = () => saveHook.mutate();
const removeTarget = (item: MonitorTarget) => {
  if (window.confirm(`Excluir monitor ${item.name}?`)) remove.mutate(item.id);
};
const editTarget = (item: MonitorTarget) => {
  const name = window.prompt("Nome do monitor", item.name); if (name === null) return;
  const target = window.prompt("Alvo (URL ou host:porta)", item.target); if (target === null) return;
  const interval = window.prompt("Intervalo em segundos", String(item.intervalSeconds)); if (interval === null) return;
  const intervalSeconds = Number(interval);
  if (!name.trim() || !target.trim() || !Number.isInteger(intervalSeconds) || intervalSeconds < 15 || intervalSeconds > 3600) { window.alert("Dados inválidos. Intervalo entre 15 e 3600 segundos."); return; }
  update.mutate({ id:item.id, target:{ name:name.trim(), target:target.trim(), kind:item.kind, intervalSeconds, enabled:item.enabled } });
};
</script>
<template>
  <div class="mx-auto max-w-[1680px] space-y-5">
    <header class="flex justify-between">
      <div>
        <div class="flex gap-2">
          <StatusBadge
            :status="targets.isError.value ? 'critical' : 'healthy'"
            :label="
              targets.isError.value ? 'vigia indisponível' : 'vigia ativo'
            "
          /><span
            class="inline-flex items-center gap-1 font-mono text-[10px] uppercase text-signal"
            ><Activity class="h-3.5 w-3.5" />HTTP e TCP</span
          >
        </div>
        <h1 class="mt-4 text-3xl font-semibold">Uptime e alertas</h1>
        <p class="mt-2 text-sm text-muted">
          Sondas periódicas com alerta de transição para Discord, Slack ou
          webhook HTTPS.
        </p>
      </div>
      <Button variant="outline" @click="targets.refetch"
        ><RefreshCw class="h-4 w-4" />Atualizar</Button
      >
    </header>
    <section class="grid gap-5 xl:grid-cols-[1fr_380px]">
      <div class="overflow-hidden rounded-xl border border-line bg-panel/65">
        <header class="border-b border-line p-4">
          <h2 class="text-sm">Serviços monitorados</h2>
        </header>
        <div class="divide-y divide-line">
          <article
            v-for="item in targets.data.value"
            :key="item.id"
            class="grid gap-3 p-4 md:grid-cols-[1fr_120px_120px_100px]"
          >
            <div>
              <p class="text-sm text-slate-200">{{ item.name }}</p>
              <p class="mt-1 font-mono text-[10px] text-muted">
                {{ item.kind.toUpperCase() }} · {{ item.target }}
              </p>
              <p v-if="item.lastError" class="mt-1 text-[10px] text-danger">
                {{ item.lastError }}
              </p>
            </div>
            <StatusBadge
              :status="
                item.lastStatus === 'up'
                  ? 'healthy'
                  : item.lastStatus === 'down'
                    ? 'critical'
                    : 'info'
              "
              :label="item.lastStatus"
            />
            <p class="text-xs text-muted">
              {{ item.lastLatencyMS ? `${item.lastLatencyMS} ms` : "—" }}
            </p>
            <div class="flex gap-2"><Button size="sm" variant="outline" @click="editTarget(item)"
              ><Pencil class="h-3.5 w-3.5"
            /></Button><Button size="sm" variant="danger" @click="removeTarget(item)"
              ><Trash2 class="h-3.5 w-3.5"
            /></Button></div>
          </article>
          <p
            v-if="!targets.data.value?.length"
            class="p-10 text-center text-sm text-muted"
          >
            Nenhum serviço cadastrado.
          </p>
        </div>
      </div>
      <aside class="space-y-5">
        <form
          class="space-y-3 rounded-xl border border-line bg-panel/65 p-5"
          @submit.prevent="submitCreate"
        >
          <h2 class="flex gap-2 text-sm">
            <Plus class="h-4 w-4 text-signal" />Novo alvo
          </h2>
          <input
            v-model="form.name"
            required
            placeholder="Nome"
            class="w-full rounded-lg border border-line bg-slate-950 p-2 text-xs"
          /><select
            v-model="form.kind"
            class="w-full rounded-lg border border-line bg-slate-950 p-2 text-xs"
          >
            <option value="http">HTTP GET</option>
            <option value="tcp">TCP</option></select
          ><input
            v-model="form.target"
            required
            :placeholder="
              form.kind === 'http' ? 'https://servico.exemplo' : 'host:porta'
            "
            class="w-full rounded-lg border border-line bg-slate-950 p-2 text-xs"
          /><label class="text-xs text-muted"
            >Intervalo (segundos)<input
              v-model.number="form.intervalSeconds"
              type="number"
              min="15"
              max="3600"
              class="mt-1 w-full rounded-lg border border-line bg-slate-950 p-2" /></label
          ><Button type="submit">Adicionar monitor</Button>
        </form>
        <form
          class="space-y-3 rounded-xl border border-line bg-panel/65 p-5"
          @submit.prevent="submitWebhook"
        >
          <h2 class="flex gap-2 text-sm">
            <Link class="h-4 w-4 text-pulse" />Webhook externo
          </h2>
          <p class="text-xs text-muted">
            {{
              webhook.data.value?.configured
                ? "Webhook configurado."
                : "Sem webhook configurado."
            }}
            O endereço não é exibido após salvo.
          </p>
          <input
            v-model="hook"
            type="url"
            placeholder="https://discord.com/api/webhooks/..."
            class="w-full rounded-lg border border-line bg-slate-950 p-2 text-xs"
          /><Button type="submit">Salvar webhook</Button>
        </form>
      </aside>
    </section>
  </div>
</template>
