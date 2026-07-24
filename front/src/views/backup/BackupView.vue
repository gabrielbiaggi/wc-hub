<script setup lang="ts">
import { computed } from "vue";
import { useQuery } from "@tanstack/vue-query";
import {
  Archive,
  Database,
  HardDrive,
  RefreshCw,
  ShieldCheck,
} from "lucide-vue-next";
import Button from "@/components/ui/Button.vue";
import StatusBadge from "@/components/ui/StatusBadge.vue";
import { apiErrorMessage } from "@/lib/api_error";
import { getBackupOverview } from "@/lib/api_backup";
import { restoreProxmoxBackup } from "@/lib/api";
import { useMutation } from "@tanstack/vue-query";
import { ref } from "vue";

const query = useQuery({
  queryKey: ["backup-overview"],
  queryFn: getBackupOverview,
  refetchInterval: 60000,
});
const stores = computed(() => query.data.value?.datastores ?? []);
const errorMessage = computed(() =>
  query.isError.value
    ? apiErrorMessage(
        query.error.value,
        "Não foi possível consultar o Proxmox Backup Server.",
      )
    : "",
);

const showRestoreModal = ref(false);
const restoreNode = ref("");
const restoreStorage = ref("pbs");
const restoreArchive = ref("");
const restoreVMID = ref<number | undefined>(undefined);
const restoreForce = ref(false);

const restoreMutation = useMutation({
  mutationFn: () =>
    restoreProxmoxBackup(
      restoreNode.value,
      restoreStorage.value,
      restoreArchive.value,
      restoreVMID.value!,
      restoreForce.value,
    ),
  onSuccess: () => {
    alert("Processo de restauração de backup iniciado no Proxmox VE!");
    showRestoreModal.value = false;
  },
});

const bytes = (n = 0) => {
  const u = ["B", "KB", "MB", "GB", "TB"];
  const i = n ? Math.min(Math.floor(Math.log(n) / Math.log(1024)), 4) : 0;
  return `${(n / 1024 ** i).toFixed(i > 2 ? 1 : 0)} ${u[i]}`;
};
</script>
<template>
  <div class="mx-auto max-w-[1680px] space-y-5">
    <header class="flex justify-between">
      <div>
        <div class="flex gap-2">
          <StatusBadge
            :status="query.isError.value ? 'critical' : 'healthy'"
            :label="
              query.isError.value ? 'PBS indisponível' : 'backup read-only'
            "
          /><span
            class="inline-flex items-center gap-1 font-mono text-[10px] uppercase text-signal"
            ><ShieldCheck class="h-3.5 w-3.5" />API PBS</span
          >
        </div>
        <h1 class="mt-4 text-3xl font-semibold">Backups e recuperação</h1>
        <p class="mt-2 text-sm text-muted">
          Datastores, espaço, snapshots, deduplicação e tarefas recentes do
          Proxmox Backup Server.
        </p>
      </div>
      <div class="flex items-center gap-2">
        <Button variant="outline" @click="showRestoreModal = true"><Archive class="h-4 w-4" />Restaurar PBS para Proxmox</Button>
        <Button variant="outline" @click="query.refetch"><RefreshCw class="h-4 w-4" />Atualizar</Button>
      </div>
    </header>

    <div v-if="showRestoreModal" class="fixed inset-0 z-50 flex items-center justify-center bg-black/80 p-4" @click.self="showRestoreModal = false">
      <article class="w-full max-w-lg overflow-hidden rounded-xl border border-line bg-panel p-6 shadow-2xl space-y-4">
        <h3 class="text-lg font-medium text-slate-100">Restaurar Snapshot PBS no Proxmox VE</h3>
        <p class="text-xs text-muted">Forneça as informações do snapshot e do nó de destino para disparar o restore.</p>

        <form class="space-y-3" @submit.prevent="restoreMutation.mutate()">
          <div>
            <label class="text-xs text-muted block mb-1">Nó Proxmox VE Destino</label>
            <input v-model="restoreNode" required placeholder="ex: pve-node-1" class="w-full rounded-lg border border-line bg-slate-950 p-2 text-xs text-slate-200" />
          </div>
          <div>
            <label class="text-xs text-muted block mb-1">Storage PBS de Origem</label>
            <input v-model="restoreStorage" required placeholder="ex: pbs-backup" class="w-full rounded-lg border border-line bg-slate-950 p-2 text-xs text-slate-200" />
          </div>
          <div>
            <label class="text-xs text-muted block mb-1">Arquivo Snapshot / Volid PBS</label>
            <input v-model="restoreArchive" required placeholder="ex: pbs-backup:backup/vm/100/2026-07-24T12:00:00Z" class="w-full rounded-lg border border-line bg-slate-950 p-2 text-xs font-mono text-slate-200" />
          </div>
          <div>
            <label class="text-xs text-muted block mb-1">Novo VMID / Target VMID</label>
            <input v-model.number="restoreVMID" type="number" required placeholder="ex: 105" class="w-full rounded-lg border border-line bg-slate-950 p-2 text-xs font-mono text-slate-200" />
          </div>
          <div class="flex items-center gap-2 pt-2">
            <input v-model="restoreForce" type="checkbox" id="forceRestore" class="rounded border-line" />
            <label for="forceRestore" class="text-xs text-slate-300">Sobrescrever se o VMID já existir (Force)</label>
          </div>

          <div class="flex justify-end gap-2 pt-4 border-t border-line">
            <Button variant="outline" type="button" @click="showRestoreModal = false">Cancelar</Button>
            <Button type="submit" :disabled="!restoreNode || !restoreStorage || !restoreArchive || !restoreVMID || restoreMutation.isPending.value">Disparar Restauração</Button>
          </div>
        </form>
      </article>
    </div>
    <div
      v-if="errorMessage"
      class="rounded-xl border border-danger/30 bg-danger/5 p-4 text-sm text-danger"
    >
      <p class="font-medium">A integração com o Proxmox Backup Server não está disponível.</p>
      <p class="mt-1 break-words text-xs">{{ errorMessage }}</p>
      <p class="mt-2 text-xs text-muted">
        Defina a configuração PBS no backend e atualize a tela. Os indicadores abaixo não representam dados de backup enquanto a conexão falhar.
      </p>
    </div>
    <div
      v-if="query.data.value?.warnings.length"
      class="rounded-xl border border-warning/20 bg-warning/5 p-4 text-xs text-warning"
    >
      {{ query.data.value.warnings.join(" · ") }}
    </div>
    <section class="grid gap-3 md:grid-cols-3">
      <article
        v-for="item in [
          { l: 'Datastores', v: stores.length, i: Database },
          {
            l: 'Snapshots',
            v: stores.reduce((n, s) => n + s.snapshots, 0),
            i: Archive,
          },
          {
            l: 'Espaço livre',
            v: bytes(stores.reduce((n, s) => n + s.available, 0)),
            i: HardDrive,
          },
        ]"
        :key="item.l"
        class="rounded-xl border border-line bg-panel/65 p-5"
      >
        <component :is="item.i" class="h-4 w-4 text-muted" />
        <p class="mt-4 font-mono text-2xl">{{ item.v }}</p>
        <p class="text-xs text-muted">{{ item.l }}</p>
      </article>
    </section>
    <section class="overflow-hidden rounded-xl border border-line bg-panel/65">
      <header class="border-b border-line p-4">
        <h2 class="text-sm">Datastores</h2>
      </header>
      <div class="divide-y divide-line">
        <article
          v-for="store in stores"
          :key="store.name"
          class="grid gap-3 p-4 md:grid-cols-[1fr_160px_160px_120px]"
        >
          <div>
            <p class="text-sm text-slate-200">{{ store.name }}</p>
            <p class="mt-1 font-mono text-[9px] text-muted">
              {{ store.path }} · {{ store.snapshots }} snapshots
            </p>
          </div>
          <p class="text-xs text-muted">
            {{ bytes(store.used) }} / {{ bytes(store.total) }}
          </p>
          <p class="text-xs text-muted">
            Deduplicação {{ store.deduplication.toFixed(2) }}×
          </p>
          <StatusBadge
            :status="store.status === 'healthy' ? 'healthy' : 'warning'"
            :label="store.status"
          />
        </article>
      </div>
    </section>
    <section class="overflow-hidden rounded-xl border border-line bg-panel/65">
      <header class="border-b border-line p-4">
        <h2 class="text-sm">Tarefas recentes</h2>
      </header>
      <div class="divide-y divide-line">
        <article
          v-for="task in query.data.value?.tasks"
          :key="task.upid"
          class="grid gap-2 p-3 md:grid-cols-[1fr_160px_150px]"
        >
          <p class="font-mono text-[10px] text-slate-300">
            {{ task.worker_type }}
          </p>
          <p class="text-xs text-muted">{{ task.user }}</p>
          <p class="text-xs text-muted">{{ task.status || "em andamento" }}</p>
        </article>
      </div>
    </section>
  </div>
</template>
