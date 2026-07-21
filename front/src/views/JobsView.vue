<script setup lang="ts">
import { ref } from "vue";
import { useMutation, useQuery, useQueryClient } from "@tanstack/vue-query";
import { Clock3, Play, RefreshCw, Workflow } from "lucide-vue-next";
import Button from "@/components/ui/Button.vue";
import StatusBadge from "@/components/ui/StatusBadge.vue";
import { enqueueJob, getJobs } from "@/lib/api";

const client = useQueryClient();
const selectedKind = ref<"proxmox.sync" | "telemetry.maintenance">(
  "proxmox.sync",
);
const query = useQuery({
  queryKey: ["jobs"],
  queryFn: getJobs,
  refetchInterval: 2500,
});
const enqueue = useMutation({
  mutationFn: () => enqueueJob(selectedKind.value),
  onSuccess: () => client.invalidateQueries({ queryKey: ["jobs"] }),
});
const tone = (status: string) =>
  status === "succeeded"
    ? "healthy"
    : status === "failed"
      ? "critical"
      : status === "running"
        ? "info"
        : "warning";
const run = () => {
  if (
    window.confirm(
      `Enfileirar ${selectedKind.value}? A operação é auditada e executada pelo worker.`,
    )
  )
    enqueue.mutate();
};
</script>

<template>
  <div class="mx-auto max-w-[1680px] space-y-5">
    <header
      class="flex flex-col gap-4 md:flex-row md:items-end md:justify-between"
    >
      <div>
        <div
          class="flex items-center gap-2 font-mono text-[10px] uppercase tracking-widest text-signal"
        >
          <Workflow class="h-3.5 w-3.5" />Fila durável no PostgreSQL
        </div>
        <h1 class="mt-3 text-3xl font-semibold tracking-tight">
          Tarefas e workers
        </h1>
        <p class="mt-2 text-sm text-muted">
          Reserva concorrente SKIP LOCKED, nova tentativa exponencial e execução
          fora da requisição.
        </p>
      </div>
      <div class="flex flex-wrap gap-2">
        <select
          v-model="selectedKind"
          class="rounded-lg border border-line bg-panel px-3 py-2 text-xs text-slate-100"
        >
          <option value="proxmox.sync">Sincronizar Proxmox</option>
          <option value="telemetry.maintenance">
            Manutenção de telemetria
          </option></select
        ><Button :disabled="enqueue.isPending.value" @click="run"
          ><Play class="h-4 w-4" />Enfileirar</Button
        ><Button variant="outline" @click="query.refetch()"
          ><RefreshCw class="h-4 w-4" />Atualizar</Button
        >
      </div>
    </header>
    <p
      v-if="enqueue.isError.value"
      class="rounded-xl border border-danger/20 bg-danger/5 p-4 text-sm text-danger"
    >
      Não foi possível enfileirar a tarefa.
    </p>
    <section class="overflow-hidden rounded-xl border border-line bg-panel/65">
      <div class="divide-y divide-line/60">
        <article
          v-for="job in query.data.value"
          :key="job.id"
          class="grid gap-3 px-5 py-4 md:grid-cols-[1fr_130px_120px_170px]"
        >
          <div class="flex gap-3">
            <div
              class="grid h-9 w-9 shrink-0 place-items-center rounded-lg border border-line bg-slate-950/50"
            >
              <Clock3 class="h-4 w-4 text-muted" />
            </div>
            <div>
              <p class="text-sm text-slate-200">{{ job.kind }}</p>
              <p class="mt-1 font-mono text-[9px] text-muted">{{ job.id }}</p>
              <p v-if="job.last_error" class="mt-2 text-[11px] text-danger">
                {{ job.last_error }}
              </p>
            </div>
          </div>
          <div class="self-center">
            <StatusBadge :status="tone(job.status)" :label="job.status" />
          </div>
          <p class="self-center font-mono text-[10px] text-muted">
            TENTATIVA {{ job.attempts }}/{{ job.max_attempts }}
          </p>
          <time class="self-center text-xs text-muted">{{
            new Date(job.created_at).toLocaleString("pt-BR")
          }}</time>
        </article>
        <div
          v-if="!query.data.value?.length"
          class="p-14 text-center text-sm text-muted"
        >
          Fila vazia.
        </div>
      </div>
    </section>
  </div>
</template>
