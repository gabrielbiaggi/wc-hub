<script setup lang="ts">
import { computed, ref } from "vue";
import { useMutation, useQuery, useQueryClient } from "@tanstack/vue-query";
import {
  Box,
  Container,
  Play,
  Power,
  RefreshCw,
  RotateCcw,
  ShieldCheck,
  Square,
  Terminal,
} from "lucide-vue-next";
import Button from "@/components/ui/Button.vue";
import StatusBadge from "@/components/ui/StatusBadge.vue";
import {
  dockerContainerAction,
  dockerContainerExec,
  getDockerInventory,
  type DockerContainer,
} from "@/lib/api";
import { apiErrorMessage } from "@/lib/api_error";

const client = useQueryClient();
const inventory = useQuery({
  queryKey: ["docker-inventory"],
  queryFn: getDockerInventory,
  refetchInterval: 15000,
});

const containerAction = useMutation({
  mutationFn: (input: { id: string; action: "start" | "stop" | "restart" }) =>
    dockerContainerAction(input.id, input.action),
  onSuccess: () =>
    setTimeout(
      () => client.invalidateQueries({ queryKey: ["docker-inventory"] }),
      1200,
    ),
});

const execForm = ref({ containerId: "", command: "" });
const execOutput = ref("");
const execContainer = useMutation({
  mutationFn: (input: { id: string; command: string[] }) =>
    dockerContainerExec(input.id, input.command),
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

const execute = (
  container: DockerContainer,
  action: "start" | "stop" | "restart",
) => {
  if (
    window.confirm(
      `Confirma ${action} do container ${container.names[0] || container.id.substring(0, 12)}?`,
    )
  )
    containerAction.mutate({ id: container.id, action });
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
  const command = execForm.value.command.trim().split(/\s+/);
  if (command.length === 0) return;
  execContainer.mutate({ id: execForm.value.containerId, command });
};

const selectedContainer = computed(() =>
  inventory.data.value?.containers.find(
    (c) => c.id === execForm.value.containerId,
  ),
);
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
          Containers, imagens, estatísticas e controle via Docker API.
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
          <div class="flex flex-wrap justify-end gap-2">
            <Button
              variant="outline"
              @click="openExec(container)"
              :disabled="container.state !== 'running'"
              ><Terminal class="h-3.5 w-3.5" />Exec</Button
            >
            <Button
              v-if="container.state !== 'running'"
              variant="outline"
              :disabled="containerAction.isPending.value"
              @click="execute(container, 'start')"
              ><Play class="h-3.5 w-3.5" />Iniciar</Button
            >
            <template v-else>
              <Button
                variant="outline"
                :disabled="containerAction.isPending.value"
                @click="execute(container, 'restart')"
                ><RotateCcw class="h-3.5 w-3.5" />Reiniciar</Button
              >
              <Button
                variant="danger"
                :disabled="containerAction.isPending.value"
                @click="execute(container, 'stop')"
                ><Square class="h-3.5 w-3.5" />Parar</Button
              >
            </template>
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
  </div>
</template>
