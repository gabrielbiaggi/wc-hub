<script setup lang="ts">
import { computed, ref } from "vue";
import { useMutation, useQuery, useQueryClient } from "@tanstack/vue-query";
import {
  Box,
  Container,
  Copy,
  Play,
  Power,
  RefreshCw,
  RotateCcw,
  ShieldCheck,
  Square,
  Terminal,
  Trash2,
  Upload,
  Zap,
} from "lucide-vue-next";
import Button from "@/components/ui/Button.vue";
import StatusBadge from "@/components/ui/StatusBadge.vue";
import ActionGuardModal from "@/components/ActionGuardModal.vue";
import { usePermissions } from "@/composables/usePermissions";
import {
  dockerContainerAction,
  dockerContainerExec,
  getDockerInventory,
  type DockerContainer,
  buildActionHeaders,
} from "@/lib/api";
import { apiErrorMessage } from "@/lib/api_error";

const client = useQueryClient();
const { hasPermission } = usePermissions();
const canManage = computed(() => hasPermission("docker.manage"));
type GuardedDockerAction = { kind: "action"; id: string; action: "stop" | "restart" | "kill" | "remove" } | { kind: "exec"; id: string; command: string[] };
const guardedAction = ref<GuardedDockerAction | null>(null);
const guardedTarget = computed(() => guardedAction.value ? `docker/container/${guardedAction.value.id}` : "");
const inventory = useQuery({
  queryKey: ["docker-inventory"],
  queryFn: getDockerInventory,
  refetchInterval: 15000,
});

const containerAction = useMutation({
  mutationFn: (input: { id: string; action: "start" | "stop" | "restart" | "kill" | "remove"; headers?: Record<string,string> }) =>
    dockerContainerAction(input.id, input.action, input.headers),
  onSuccess: () =>
    setTimeout(
      () => client.invalidateQueries({ queryKey: ["docker-inventory"] }),
      1200,
    ),
});

const execForm = ref({ containerId: "", command: "" });
const execOutput = ref("");
const execContainer = useMutation({
  mutationFn: (input: { id: string; command: string[]; headers?: Record<string,string> }) =>
    dockerContainerExec(input.id, input.command, input.headers),
  onSuccess: (data) => {
    execOutput.value = data.output;
  },
});

const providerError = computed(() =>
  apiErrorMessage(
    inventory.error.value ??
      containerAction.error.value ??
      execContainer.error.value,
    "A operação Docker falhou.",
  ),
);

const formatBytes = (value = 0) => {
  if (!value) return "0 B";
  const units = ["B", "KB", "MB", "GB", "TB"],
    index = Math.min(Math.floor(Math.log(value) / Math.log(1024)), 4);
  return `${(value / 1024 ** index).toFixed(index > 2 ? 1 : 0)} ${units[index]}`;
};

const formatDate = (timestamp: number) =>
  new Date(timestamp * 1000).toLocaleString("pt-BR");
const composeProject = (container: DockerContainer) =>
  container.labels["com.docker.compose.project"] || "serviço avulso";
const composeService = (container: DockerContainer) =>
  container.labels["com.docker.compose.service"] || "sem serviço Compose";

const execute = (
  container: DockerContainer,
  action: "start" | "stop" | "restart" | "kill" | "remove",
) => {
  if (!canManage.value) return;
  if (action === "start") { containerAction.mutate({ id: container.id, action }); return; }
  guardedAction.value = { kind: "action", id: container.id, action };
};

const openExec = (container: DockerContainer) => {
  execForm.value.containerId = container.id;
  execForm.value.command = "";
  execOutput.value = "";
};

const closeExec = () => {
  execForm.value.containerId = "";
  execForm.value.command = "";
  execOutput.value = "";
};

const submitExec = () => {
  if (!canManage.value) return;
  const command = execForm.value.command.trim().split(/\s+/);
  if (command.length === 0 || !command[0]) return;
  guardedAction.value = { kind: "exec", id: execForm.value.containerId, command };
};

const confirmGuardedAction = (payload: { confirmation:string; totpCode:string }) => {
  const action = guardedAction.value;
  if (!action) return;
  const headers = buildActionHeaders(payload.confirmation, payload.totpCode);
  if (action.kind === "exec") execContainer.mutate({ id: action.id, command: action.command, headers });
  else containerAction.mutate({ id: action.id, action: action.action, headers });
  guardedAction.value = null;
};

const selectedContainer = computed(() =>
  inventory.data.value?.containers.find(
    (c) => c.id === execForm.value.containerId,
  ),
);

// Auto-Update SSE Stream
const updateStreamModalOpen = ref(false);
const updateStreamLogs = ref<{ text: string; type: string }[]>([]);
const updateStreamActive = ref(false);
const currentUpdatingContainer = ref<DockerContainer | null>(null);
const activeEventSource = ref<EventSource | null>(null);

const startUpdateStream = (container: DockerContainer) => {
  currentUpdatingContainer.value = container;
  updateStreamLogs.value = [];
  updateStreamModalOpen.value = true;
  updateStreamActive.value = true;

  const url = `/api/v1/docker/containers/${container.id}/update-stream`;
  const eventSource = new EventSource(url, { withCredentials: true });
  activeEventSource.value = eventSource;

  eventSource.addEventListener("log", (e) => {
    try {
      const data = JSON.parse(e.data);
      updateStreamLogs.value.push(data);
    } catch {
      updateStreamLogs.value.push({ text: e.data, type: "info" });
    }
  });

  eventSource.addEventListener("complete", () => {
    updateStreamActive.value = false;
    eventSource.close();
    activeEventSource.value = null;
    client.invalidateQueries({ queryKey: ["docker-inventory"] });
  });

  eventSource.onerror = () => {
    updateStreamActive.value = false;
    eventSource.close();
    activeEventSource.value = null;
  };
};

const closeUpdateStream = () => {
  if (activeEventSource.value) {
    activeEventSource.value.close();
    activeEventSource.value = null;
  }
  updateStreamActive.value = false;
  updateStreamModalOpen.value = false;
  currentUpdatingContainer.value = null;
};

// Clone Stack
const cloneModalOpen = ref(false);
const cloneForm = ref({ containerId: "", suffix: "staging" });
const cloneStatus = ref("");

const openClone = (container: DockerContainer) => {
  cloneForm.value.containerId = container.id;
  cloneForm.value.suffix = "staging";
  cloneStatus.value = "";
  cloneModalOpen.value = true;
};

const submitClone = async () => {
  if (!cloneForm.value.containerId) return;
  try {
    cloneStatus.value = "Clonando stack...";
    const { cloneDockerStack } = await import("@/lib/api_docker");
    const result = await cloneDockerStack(cloneForm.value.containerId, cloneForm.value.suffix);
    cloneStatus.value = `Sucesso! Stack clonada criada: ${result.new_stack_name}`;
    setTimeout(() => {
      cloneModalOpen.value = false;
      client.invalidateQueries({ queryKey: ["docker-inventory"] });
    }, 1500);
  } catch (err: any) {
    cloneStatus.value = "Erro: " + (err.response?.data?.error?.message || err.message);
  }
};
</script>

<template>
  <div class="mx-auto max-w-[1680px] space-y-5">
    <header
      class="flex flex-col justify-between gap-4 md:flex-row md:items-end"
    >
      <div>
        <div class="flex flex-wrap gap-2">
          <StatusBadge
            :status="
              inventory.isError.value
                ? 'critical'
                : inventory.data.value?.health.reachable
                  ? 'healthy'
                  : 'critical'
            "
            :label="
              inventory.isError.value
                ? 'provedor indisponível'
                : 'runtime Docker'
            "
          />
          <span
            class="inline-flex items-center gap-1.5 font-mono text-[10px] uppercase text-signal"
            ><ShieldCheck class="h-3.5 w-3.5" />API Docker · operações
            auditadas</span
          >
        </div>
        <h1 class="mt-4 text-3xl font-semibold">Docker Engine</h1>
        <p class="mt-2 text-sm text-muted">
          Origem: <span class="font-mono text-slate-300">{{ inventory.data.value?.source || inventory.data.value?.health.source || "aguardando endpoint" }}</span>. Containers classificados por projeto Compose e serviço.
        </p>
      </div>
      <Button
        :disabled="inventory.isLoading.value || inventory.isRefetching.value"
        @click="inventory.refetch()"
        ><RefreshCw
          :class="[
            'h-4 w-4',
            inventory.isRefetching.value && 'animate-spin',
          ]"
        />Atualizar</Button
      >
    </header>
    <div
      v-if="
        inventory.isError.value ||
        containerAction.isError.value ||
        execContainer.isError.value
      "
      class="rounded-xl border border-danger/20 bg-danger/5 p-4 text-sm text-danger"
    >
      <p class="font-medium">A operação Docker falhou</p>
      <p class="mt-1 break-words font-mono text-xs">{{ providerError }}</p>
    </div>
    <div
      v-if="inventory.data.value?.warnings?.length"
      class="rounded-xl border border-warning/20 bg-warning/5 p-4 text-xs text-warning"
    >
      {{ inventory.data.value.warnings.join(" · ") }}
    </div>
    <section class="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
      <article
        v-for="item in [
          {
            label: 'Containers',
            value: inventory.data.value?.containers?.length ?? 0,
            icon: Container,
          },
          {
            label: 'Em execução',
            value:
              inventory.data.value?.containers?.filter(
                (c) => c.state === 'running',
              ).length ?? 0,
            icon: Play,
          },
          {
            label: 'Imagens',
            value: inventory.data.value?.images?.length ?? 0,
            icon: Box,
          },
          {
            label: 'Stats disponíveis',
            value: inventory.data.value?.stats?.length ?? 0,
            icon: ShieldCheck,
          },
        ]"
        :key="item.label"
        class="rounded-xl border border-line bg-panel/65 p-5"
      >
        <component :is="item.icon" class="h-4 w-4 text-muted" />
        <p class="mt-5 font-mono text-2xl text-white">{{ item.value }}</p>
        <p class="mt-1 text-xs text-muted">{{ item.label }}</p>
      </article>
    </section>
    <section
      v-if="inventory.data.value?.health"
      class="rounded-xl border border-line bg-panel/65 p-5"
    >
      <h2 class="text-sm font-medium">Informações do Engine</h2>
      <div class="mt-4 grid gap-3 text-xs md:grid-cols-4">
        <div>
          <p class="text-muted">Versão</p>
          <p class="mt-1 font-mono text-white">
            {{ inventory.data.value.health.version || "—" }}
          </p>
        </div>
        <div>
          <p class="text-muted">API Version</p>
          <p class="mt-1 font-mono text-white">
            {{ inventory.data.value.health.api_version || "—" }}
          </p>
        </div>
        <div>
          <p class="text-muted">Sistema Operacional</p>
          <p class="mt-1 font-mono text-white">
            {{ inventory.data.value.health.os_type || "—" }}
          </p>
        </div>
        <div>
          <p class="text-muted">Arquitetura</p>
          <p class="mt-1 font-mono text-white">
            {{ inventory.data.value.health.arch || "—" }}
          </p>
        </div>
      </div>
    </section>
    <article class="overflow-hidden rounded-xl border border-line bg-panel/65">
      <header class="border-b border-line p-4">
        <h2 class="text-sm font-medium">Containers</h2>
        <p class="mt-1 text-[10px] text-muted">
          Captura
          {{
            inventory.data.value
              ? new Date(inventory.data.value.captured_at).toLocaleString(
                  "pt-BR",
                )
              : "—"
          }}
          · Origem {{ inventory.data.value?.source || inventory.data.value?.health.source || "endpoint" }}
        </p>
      </header>
      <div class="divide-y divide-line/60">
        <div
          v-for="container in inventory.data.value?.containers"
          :key="container.id"
          class="grid gap-4 p-4 lg:grid-cols-[1fr_200px_340px] lg:items-center"
        >
          <div>
            <p class="text-sm text-slate-200">
              {{ container.names[0]?.replace(/^\//, "") || container.id.substring(0, 12) }}
            </p>
            <p class="mt-1 font-mono text-[9px] text-muted">
              {{ container.image }} · Criado
              {{ formatDate(container.created) }}
            </p>
            <p class="mt-1 font-mono text-[9px] text-signal">
              {{ composeProject(container) }} / {{ composeService(container) }}
            </p>
            <p
              v-if="container.ports.length > 0"
              class="mt-1 font-mono text-[9px] text-signal"
            >
              Portas:
              {{
                container.ports
                  .map(
                    (p) =>
                      `${p.public_port ? p.public_port + ":" : ""}${p.private_port}/${p.type}`,
                  )
                  .join(", ")
              }}
            </p>
          </div>
          <div>
            <StatusBadge
              :status="container.state === 'running' ? 'healthy' : 'warning'"
              :label="container.state"
            />
            <p class="mt-2 font-mono text-[9px] text-muted">
              {{ container.status }}
            </p>
          </div>
            <Button
              variant="outline"
              :disabled="!canManage"
              @click="startUpdateStream(container)"
              ><Upload class="h-3.5 w-3.5" />Auto-Update (SSE)</Button
            >
            <Button
              variant="outline"
              :disabled="!canManage"
              @click="openClone(container)"
              ><Copy class="h-3.5 w-3.5" />Clonar Stack</Button
            >
            <Button
              variant="outline"
              :disabled="!canManage || container.state !== 'running'"
              @click="openExec(container)"
              ><Terminal class="h-3.5 w-3.5" />Exec</Button
            >
            <Button
              v-if="container.state !== 'running'"
              variant="outline"
              :disabled="!canManage || containerAction.isPending.value"
              @click="execute(container, 'start')"
              ><Play class="h-3.5 w-3.5" />Iniciar</Button
            >
            <template v-else>
              <Button
                variant="outline"
                :disabled="!canManage || containerAction.isPending.value"
                @click="execute(container, 'restart')"
                ><RotateCcw class="h-3.5 w-3.5" />Reiniciar</Button
              >
              <Button
                variant="danger"
                :disabled="!canManage || containerAction.isPending.value"
                @click="execute(container, 'stop')"
                ><Square class="h-3.5 w-3.5" />Parar</Button
              >
              <Button
                variant="danger"
                :disabled="!canManage || containerAction.isPending.value"
                @click="execute(container, 'kill')"
                ><Zap class="h-3.5 w-3.5" />Forçar parada</Button
              >
            </template>
            <Button
              variant="danger"
              :disabled="!canManage || containerAction.isPending.value"
              @click="execute(container, 'remove')"
              ><Trash2 class="h-3.5 w-3.5" />Remover</Button
            >
          </div>
        </div>
        <p
          v-if="!inventory.data.value?.containers?.length"
          class="p-10 text-center text-sm text-muted"
        >
          Nenhum container encontrado.
        </p>
      </div>
    </article>
    <article class="overflow-hidden rounded-xl border border-line bg-panel/65">
      <header class="border-b border-line p-4">
        <h2 class="text-sm font-medium">Imagens</h2>
      </header>
      <div class="divide-y divide-line/60">
        <div
          v-for="image in inventory.data.value?.images"
          :key="image.id"
          class="grid gap-3 p-4 md:grid-cols-[1fr_180px_180px]"
        >
          <div>
            <p class="text-sm text-slate-200">
              {{
                image.repo_tags[0] ||
                image.id.replace(/^sha256:/, "").substring(0, 12)
              }}
            </p>
            <p class="mt-1 font-mono text-[9px] text-muted">
              Criado {{ formatDate(image.created) }}
              <template v-if="image.containers > 0">
                · {{ image.containers }} container(s)
              </template>
            </p>
          </div>
          <div>
            <p class="font-mono text-xs text-muted">
              Tamanho: {{ formatBytes(image.size) }}
            </p>
          </div>
          <div>
            <p class="font-mono text-xs text-muted">
              Compartilhado: {{ formatBytes(image.shared_size) }}
            </p>
          </div>
        </div>
        <p
          v-if="!inventory.data.value?.images?.length"
          class="p-10 text-center text-sm text-muted"
        >
          Nenhuma imagem encontrada.
        </p>
      </div>
    </article>
    <article
      v-if="inventory.data.value?.stats?.length"
      class="overflow-hidden rounded-xl border border-line bg-panel/65"
    >
      <header class="border-b border-line p-4">
        <h2 class="text-sm font-medium">Estatísticas de Runtime</h2>
      </header>
      <div class="divide-y divide-line/60">
        <div
          v-for="stat in inventory.data.value.stats"
          :key="stat.container_id"
          class="grid gap-3 p-4 md:grid-cols-[1fr_auto]"
        >
          <div>
            <p class="text-sm text-slate-200">{{ stat.name }}</p>
            <p class="mt-1 font-mono text-[9px] text-muted">
              {{ stat.container_id.substring(0, 12) }}
            </p>
          </div>
          <div class="grid gap-2 text-xs md:grid-cols-4">
            <div>
              <p class="text-muted">CPU</p>
              <p class="font-mono text-white">
                {{ stat.cpu_percent.toFixed(1) }}%
              </p>
            </div>
            <div>
              <p class="text-muted">Memória</p>
              <p class="font-mono text-white">
                {{ formatBytes(stat.memory_usage) }} /
                {{ formatBytes(stat.memory_limit) }}
              </p>
            </div>
            <div>
              <p class="text-muted">Rede RX/TX</p>
              <p class="font-mono text-white">
                {{ formatBytes(stat.network_rx) }} /
                {{ formatBytes(stat.network_tx) }}
              </p>
            </div>
            <div>
              <p class="text-muted">Disco R/W</p>
              <p class="font-mono text-white">
                {{ formatBytes(stat.block_read) }} /
                {{ formatBytes(stat.block_write) }}
              </p>
            </div>
          </div>
        </div>
      </div>
    </article>
    <div
      v-if="execForm.containerId"
      class="fixed inset-0 z-50 flex items-center justify-center bg-black/80 p-4"
      @click.self="closeExec"
    >
      <article
        class="w-full max-w-2xl overflow-hidden rounded-xl border border-line bg-panel shadow-2xl"
      >
        <header
          class="flex items-center justify-between border-b border-line bg-slate-950/40 p-5"
        >
          <div>
            <h2 class="text-lg font-medium">Docker Exec</h2>
            <p class="mt-1 text-xs text-muted">
              {{
                selectedContainer?.names[0]?.replace(/^\//, "") ||
                execForm.containerId.substring(0, 12)
              }}
            </p>
          </div>
          <Button variant="outline" @click="closeExec">Fechar</Button>
        </header>
        <div class="p-5">
          <form @submit.prevent="submitExec" class="space-y-4">
            <label class="text-xs text-muted"
              >Comando (separado por espaços)<input
                v-model="execForm.command"
                type="text"
                required
                placeholder="ls -la /app"
                class="mt-1 w-full rounded-lg border border-line bg-slate-950 p-2 font-mono"
            /></label>
            <Button
              type="submit"
              :disabled="execContainer.isPending.value"
              class="w-full"
              ><Terminal class="h-4 w-4" />Executar</Button
            >
          </form>
          <div
            v-if="execOutput"
            class="mt-4 max-h-96 overflow-y-auto rounded-lg border border-line bg-slate-950 p-4"
          >
            <p class="mb-2 text-xs font-medium text-signal">Output:</p>
            <pre
              class="whitespace-pre-wrap break-words font-mono text-xs text-slate-300"
              >{{ execOutput }}</pre
            >
          </div>
          <div
            v-if="execContainer.isError.value"
            class="mt-4 rounded-lg border border-danger/20 bg-danger/5 p-3 text-xs text-danger"
          >
            Erro: {{ execContainer.error.value?.message }}
          </div>
        </div>
      </article>
    </div>
    <!-- Modal Auto-Update SSE -->
    <div
      v-if="updateStreamModalOpen"
      class="fixed inset-0 z-50 flex items-center justify-center bg-black/80 p-4"
      @click.self="closeUpdateStream"
    >
      <article class="w-full max-w-2xl overflow-hidden rounded-xl border border-line bg-panel shadow-2xl">
        <header class="flex items-center justify-between border-b border-line bg-slate-950/40 p-5">
          <div>
            <h2 class="text-lg font-medium">Auto-Update Remoto (SSE Stream)</h2>
            <p class="mt-1 text-xs text-muted">
              Container: {{ currentUpdatingContainer?.names[0]?.replace(/^\//, "") }}
            </p>
          </div>
          <Button variant="outline" :disabled="updateStreamActive" @click="closeUpdateStream">Fechar</Button>
        </header>
        <div class="p-5">
          <div class="mb-3 flex items-center gap-2">
            <span :class="['h-2.5 w-2.5 rounded-full', updateStreamActive ? 'animate-ping bg-emerald-400' : 'bg-slate-500']"></span>
            <span class="text-xs font-mono text-signal">
              {{ updateStreamActive ? "Transmitindo logs do deploy em tempo real..." : "Processo finalizado" }}
            </span>
          </div>
          <div class="max-h-96 min-h-[160px] overflow-y-auto rounded-lg border border-line bg-slate-950 p-4 font-mono text-xs">
            <div v-for="(log, idx) in updateStreamLogs" :key="idx" :class="[
              log.type === 'error' ? 'text-rose-400 font-bold' :
              log.type === 'warning' ? 'text-amber-300 font-semibold' :
              log.type === 'success' ? 'text-emerald-400 font-bold' : 'text-slate-300'
            ]">
              [{{ new Date().toLocaleTimeString() }}] {{ log.text }}
            </div>
            <div v-if="updateStreamLogs.length === 0" class="text-slate-500">Conectando ao stream do container...</div>
          </div>
        </div>
      </article>
    </div>

    <!-- Modal Clonar Stack -->
    <div
      v-if="cloneModalOpen"
      class="fixed inset-0 z-50 flex items-center justify-center bg-black/80 p-4"
      @click.self="cloneModalOpen = false"
    >
      <article class="w-full max-w-md overflow-hidden rounded-xl border border-line bg-panel shadow-2xl">
        <header class="flex items-center justify-between border-b border-line bg-slate-950/40 p-5">
          <div>
            <h2 class="text-lg font-medium">Clonar Stack Docker</h2>
            <p class="mt-1 text-xs text-muted">Criar cópia da stack em novo ambiente</p>
          </div>
          <Button variant="outline" @click="cloneModalOpen = false">Fechar</Button>
        </header>
        <div class="p-5 space-y-4">
          <label class="block text-xs text-muted">
            Sufixo do Ambiente (ex: staging, test)
            <input v-model="cloneForm.suffix" type="text" class="mt-1 w-full rounded-lg border border-line bg-slate-950 p-2 font-mono" placeholder="staging" />
          </label>
          <Button class="w-full" @click="submitClone"><Copy class="h-4 w-4" />Confirmar Clonagem</Button>
          <p v-if="cloneStatus" class="mt-2 font-mono text-xs text-signal break-words">{{ cloneStatus }}</p>
        </div>
      </article>
    </div>

    <ActionGuardModal :show="!!guardedAction" :target-name="guardedTarget" title="Operação Docker protegida" @cancel="guardedAction = null" @confirm="confirmGuardedAction" />
  </div>
</template>
