<script setup lang="ts">
import { computed, ref } from "vue";
import { useMutation, useQuery, useQueryClient } from "@tanstack/vue-query";
import {
  AlertCircle,
  Box,
  ChevronDown,
  ChevronUp,
  FileText,
  Play,
  RefreshCw,
  RotateCcw,
  Server,
  ShieldCheck,
  Terminal,
} from "lucide-vue-next";
import Button from "@/components/ui/Button.vue";
import StatusBadge from "@/components/ui/StatusBadge.vue";
import {
  getKubernetesOverview,
  getKubernetesPodLogs,
  kubernetesDeploymentAction,
  kubernetesPodExec,
  type KubernetesDeployment,
  type KubernetesPod,
} from "@/lib/api";
import { apiErrorMessage } from "@/lib/api_error";

type Tab = "nodes" | "deployments" | "pods" | "events";
const tab = ref<Tab>("deployments");
const client = useQueryClient();
const overview = useQuery({
  queryKey: ["kubernetes-overview"],
  queryFn: getKubernetesOverview,
  refetchInterval: 15000,
});

const deploymentAction = useMutation({
  mutationFn: (input: {
    namespace: string;
    name: string;
    action: "scale" | "restart";
    replicas?: number;
  }) =>
    kubernetesDeploymentAction(
      input.namespace,
      input.name,
      input.action,
      input.replicas,
    ),
  onSuccess: () =>
    setTimeout(
      () => client.invalidateQueries({ queryKey: ["kubernetes-overview"] }),
      2000,
    ),
});

const logsForm = ref({ namespace: "", pod: "", container: "", tail: 100 });
const logsOutput = ref("");
const podLogs = useMutation({
  mutationFn: () =>
    getKubernetesPodLogs(
      logsForm.value.namespace,
      logsForm.value.pod,
      logsForm.value.container || undefined,
      logsForm.value.tail,
    ),
  onSuccess: (data) => {
    logsOutput.value = data.logs;
  },
});

const execForm = ref({
  namespace: "",
  pod: "",
  container: "",
  command: "",
});
const execOutput = ref("");
const podExec = useMutation({
  mutationFn: (input: {
    namespace: string;
    pod: string;
    container: string;
    command: string[];
  }) =>
    kubernetesPodExec(
      input.namespace,
      input.pod,
      input.container,
      input.command,
    ),
  onSuccess: (data) => {
    execOutput.value = data.output;
  },
});

const providerError = computed(() =>
  apiErrorMessage(
    overview.error.value ??
      deploymentAction.error.value ??
      podLogs.error.value ??
      podExec.error.value,
    "A operação Kubernetes falhou.",
  ),
);

const formatDate = (timestamp: string) =>
  new Date(timestamp).toLocaleString("pt-BR");

const scaleDeployment = (deployment: KubernetesDeployment) => {
  const current = deployment.replicas;
  const input = window.prompt(
    `Escalar deployment ${deployment.metadata.name}\n\nRéplicas atual: ${current}\nNovo valor (0-100):`,
    current.toString(),
  );
  if (input === null) return;
  const replicas = parseInt(input, 10);
  if (isNaN(replicas) || replicas < 0 || replicas > 100) {
    window.alert("Valor inválido. Use um número entre 0 e 100.");
    return;
  }
  if (
    window.confirm(
      `Confirma escalar ${deployment.metadata.name} de ${current} para ${replicas} réplicas?`,
    )
  ) {
    deploymentAction.mutate({
      namespace: deployment.metadata.namespace,
      name: deployment.metadata.name,
      action: "scale",
      replicas,
    });
  }
};

const restartDeployment = (deployment: KubernetesDeployment) => {
  if (
    window.confirm(
      `Confirma restart do deployment ${deployment.metadata.name} no namespace ${deployment.metadata.namespace}?`,
    )
  ) {
    deploymentAction.mutate({
      namespace: deployment.metadata.namespace,
      name: deployment.metadata.name,
      action: "restart",
    });
  }
};

const openLogs = (pod: KubernetesPod) => {
  logsForm.value.namespace = pod.metadata.namespace;
  logsForm.value.pod = pod.metadata.name;
  logsForm.value.container =
    pod.containers.length === 1 ? pod.containers[0].name : "";
  logsForm.value.tail = 100;
  logsOutput.value = "";
};

const closeLogs = () => {
  logsForm.value = { namespace: "", pod: "", container: "", tail: 100 };
  logsOutput.value = "";
};

const submitLogs = () => {
  podLogs.mutate();
};

const openExec = (pod: KubernetesPod) => {
  execForm.value.namespace = pod.metadata.namespace;
  execForm.value.pod = pod.metadata.name;
  execForm.value.container =
    pod.containers.length === 1 ? pod.containers[0].name : "";
  execForm.value.command = "";
  execOutput.value = "";
};

const closeExec = () => {
  execForm.value = { namespace: "", pod: "", container: "", command: "" };
  execOutput.value = "";
};

const submitExec = () => {
  const command = execForm.value.command.trim().split(/\s+/);
  if (command.length === 0 || !execForm.value.container) return;
  podExec.mutate({
    namespace: execForm.value.namespace,
    pod: execForm.value.pod,
    container: execForm.value.container,
    command,
  });
};

const selectedPod = computed(() =>
  overview.data.value?.pods.find(
    (p) =>
      p.metadata.namespace === logsForm.value.namespace &&
      p.metadata.name === logsForm.value.pod,
  ),
);

const selectedExecPod = computed(() =>
  overview.data.value?.pods.find(
    (p) =>
      p.metadata.namespace === execForm.value.namespace &&
      p.metadata.name === execForm.value.pod,
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
            :status="overview.isError.value ? 'critical' : 'healthy'"
            :label="
              overview.isError.value
                ? 'cluster indisponível'
                : 'cluster Kubernetes'
            "
          />
          <span
            class="inline-flex items-center gap-1.5 font-mono text-[10px] uppercase text-signal"
            ><ShieldCheck class="h-3.5 w-3.5" />API K8s · operações
            auditadas</span
          >
        </div>
        <h1 class="mt-4 text-3xl font-semibold">Kubernetes Cluster</h1>
        <p class="mt-2 text-sm text-muted">
          Nodes, deployments, pods e eventos via API Kubernetes.
        </p>
      </div>
      <Button
        :disabled="overview.isLoading.value || overview.isRefetching.value"
        @click="overview.refetch()"
        ><RefreshCw
          :class="['h-4 w-4', overview.isRefetching.value && 'animate-spin']"
        />Atualizar</Button
      >
    </header>
    <div
      v-if="
        overview.isError.value ||
        deploymentAction.isError.value ||
        podLogs.isError.value ||
        podExec.isError.value
      "
      class="rounded-xl border border-danger/20 bg-danger/5 p-4 text-sm text-danger"
    >
      <p class="font-medium">A operação Kubernetes falhou</p>
      <p class="mt-1 break-words font-mono text-xs">{{ providerError }}</p>
    </div>
    <div
      v-if="overview.data.value?.warnings?.length"
      class="rounded-xl border border-warning/20 bg-warning/5 p-4 text-xs text-warning"
    >
      {{ overview.data.value.warnings.join(" · ") }}
    </div>
    <section class="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
      <article
        v-for="item in [
          {
            label: 'Nodes',
            value: overview.data.value?.nodes?.length ?? 0,
            icon: Server,
          },
          {
            label: 'Deployments',
            value: overview.data.value?.deployments?.length ?? 0,
            icon: Box,
          },
          {
            label: 'Pods',
            value: overview.data.value?.pods?.length ?? 0,
            icon: Play,
          },
          {
            label: 'Eventos',
            value: overview.data.value?.events?.length ?? 0,
            icon: AlertCircle,
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
    <article class="overflow-hidden rounded-xl border border-line bg-panel/65">
      <header
        class="flex items-center justify-between border-b border-line p-4"
      >
        <div>
          <h2 class="text-sm font-medium">Recursos</h2>
          <p class="mt-1 text-[10px] text-muted">
            Captura
            {{
              overview.data.value
                ? formatDate(overview.data.value.captured_at)
                : "—"
            }}
            · Fonte: {{ overview.data.value?.source || "—" }}
          </p>
        </div>
        <nav class="flex flex-wrap rounded-lg border border-line p-1">
          <button
            v-for="item in [
              { id: 'deployments', label: 'Deployments' },
              { id: 'pods', label: 'Pods' },
              { id: 'nodes', label: 'Nodes' },
              { id: 'events', label: 'Eventos' },
            ]"
            :key="item.id"
            :class="[
              'rounded px-3 py-1.5 text-xs',
              tab === item.id ? 'bg-signal/10 text-signal' : 'text-muted',
            ]"
            @click="tab = item.id as Tab"
          >
            {{ item.label }}
          </button>
        </nav>
      </header>
      <div v-if="tab === 'deployments'" class="divide-y divide-line/60">
        <div
          v-for="deployment in overview.data.value?.deployments"
          :key="deployment.metadata.uid"
          class="grid gap-4 p-4 lg:grid-cols-[1fr_200px_280px] lg:items-center"
        >
          <div>
            <p class="text-sm text-slate-200">
              {{ deployment.metadata.name }}
            </p>
            <p class="mt-1 font-mono text-[9px] text-muted">
              Namespace: {{ deployment.metadata.namespace }} · Criado
              {{ formatDate(deployment.metadata.created_at) }}
            </p>
          </div>
          <div>
            <StatusBadge
              :status="
                deployment.ready_replicas === deployment.replicas &&
                deployment.replicas > 0
                  ? 'healthy'
                  : 'warning'
              "
              :label="`${deployment.ready_replicas}/${deployment.replicas} ready`"
            />
            <p class="mt-2 font-mono text-[9px] text-muted">
              Available: {{ deployment.available_replicas }} · Updated:
              {{ deployment.updated_replicas }}
            </p>
          </div>
          <div class="flex flex-wrap justify-end gap-2">
            <Button
              variant="outline"
              :disabled="deploymentAction.isPending.value"
              @click="scaleDeployment(deployment)"
              ><ChevronUp class="h-3.5 w-3.5" />Scale</Button
            >
            <Button
              variant="outline"
              :disabled="deploymentAction.isPending.value"
              @click="restartDeployment(deployment)"
              ><RotateCcw class="h-3.5 w-3.5" />Restart</Button
            >
          </div>
        </div>
        <p
          v-if="!overview.data.value?.deployments?.length"
          class="p-10 text-center text-sm text-muted"
        >
          Nenhum deployment encontrado.
        </p>
      </div>
      <div v-else-if="tab === 'pods'" class="divide-y divide-line/60">
        <div
          v-for="pod in overview.data.value?.pods"
          :key="pod.metadata.uid"
          class="grid gap-4 p-4 lg:grid-cols-[1fr_200px_280px] lg:items-center"
        >
          <div>
            <p class="text-sm text-slate-200">{{ pod.metadata.name }}</p>
            <p class="mt-1 font-mono text-[9px] text-muted">
              Namespace: {{ pod.metadata.namespace }} · Node:
              {{ pod.node_name }} · IP: {{ pod.pod_ip || "—" }}
            </p>
            <p class="mt-1 font-mono text-[9px] text-signal">
              Containers: {{ pod.containers.map((c) => c.name).join(", ") }}
            </p>
          </div>
          <div>
            <StatusBadge
              :status="
                pod.phase === 'Running' && pod.status === 'Running'
                  ? 'healthy'
                  : pod.phase === 'Pending'
                    ? 'info'
                    : 'warning'
              "
              :label="pod.phase"
            />
            <p class="mt-2 font-mono text-[9px] text-muted">{{ pod.status }}</p>
          </div>
          <div class="flex flex-wrap justify-end gap-2">
            <Button variant="outline" @click="openLogs(pod)"
              ><FileText class="h-3.5 w-3.5" />Logs</Button
            >
            <Button
              variant="outline"
              :disabled="pod.phase !== 'Running'"
              @click="openExec(pod)"
              ><Terminal class="h-3.5 w-3.5" />Exec</Button
            >
          </div>
        </div>
        <p
          v-if="!overview.data.value?.pods?.length"
          class="p-10 text-center text-sm text-muted"
        >
          Nenhum pod encontrado.
        </p>
      </div>
      <div v-else-if="tab === 'nodes'" class="divide-y divide-line/60">
        <div
          v-for="node in overview.data.value?.nodes"
          :key="node.metadata.uid"
          class="grid gap-3 p-4 md:grid-cols-[1fr_180px_280px]"
        >
          <div>
            <p class="text-sm text-slate-200">{{ node.metadata.name }}</p>
            <p class="mt-1 font-mono text-[9px] text-muted">
              Roles: {{ node.roles.join(", ") || "—" }} · Kubelet:
              {{ node.kubelet_version }}
            </p>
            <p class="mt-1 font-mono text-[9px] text-muted">
              {{ node.os_image }}
            </p>
          </div>
          <StatusBadge
            :status="node.status === 'Ready' ? 'healthy' : 'critical'"
            :label="node.status"
          />
          <div class="grid gap-2 text-xs">
            <div class="flex justify-between">
              <span class="text-muted">CPU</span>
              <span class="font-mono text-white"
                >{{ node.allocatable.cpu }} / {{ node.capacity.cpu }}</span
              >
            </div>
            <div class="flex justify-between">
              <span class="text-muted">Memory</span>
              <span class="font-mono text-white"
                >{{ node.allocatable.memory }} /
                {{ node.capacity.memory }}</span
              >
            </div>
            <div class="flex justify-between">
              <span class="text-muted">Pods</span>
              <span class="font-mono text-white"
                >{{ node.allocatable.pods }} / {{ node.capacity.pods }}</span
              >
            </div>
          </div>
        </div>
        <p
          v-if="!overview.data.value?.nodes?.length"
          class="p-10 text-center text-sm text-muted"
        >
          Nenhum node encontrado.
        </p>
      </div>
      <div v-else class="divide-y divide-line/60">
        <div
          v-for="event in overview.data.value?.events"
          :key="event.metadata.uid"
          class="grid gap-3 p-4 md:grid-cols-[1fr_120px]"
        >
          <div>
            <div class="flex items-center gap-2">
              <span
                :class="[
                  'rounded px-2 py-0.5 font-mono text-xs',
                  event.type === 'Normal'
                    ? 'bg-green-500/20 text-green-400'
                    : 'bg-warning/20 text-warning',
                ]"
              >
                {{ event.type }}
              </span>
              <span class="font-mono text-xs text-signal">{{
                event.reason
              }}</span>
            </div>
            <p class="mt-2 text-sm text-slate-200">{{ event.message }}</p>
            <p class="mt-1 font-mono text-[9px] text-muted">
              {{ event.involved_object.kind }}/{{
                event.involved_object.name
              }}
              · Namespace: {{ event.involved_object.namespace || "—" }} · Count:
              {{ event.count }}
            </p>
            <p class="mt-1 font-mono text-[9px] text-muted">
              First: {{ formatDate(event.first_timestamp) }} · Last:
              {{ formatDate(event.last_timestamp) }}
            </p>
          </div>
        </div>
        <p
          v-if="!overview.data.value?.events?.length"
          class="p-10 text-center text-sm text-muted"
        >
          Nenhum evento encontrado.
        </p>
      </div>
    </article>
    <div
      v-if="logsForm.namespace"
      class="fixed inset-0 z-50 flex items-center justify-center bg-black/80 p-4"
      @click.self="closeLogs"
    >
      <article
        class="w-full max-w-4xl overflow-hidden rounded-xl border border-line bg-panel shadow-2xl"
      >
        <header
          class="flex items-center justify-between border-b border-line bg-slate-950/40 p-5"
        >
          <div>
            <h2 class="text-lg font-medium">Pod Logs</h2>
            <p class="mt-1 text-xs text-muted">
              {{ logsForm.namespace }}/{{ logsForm.pod }}
            </p>
          </div>
          <Button variant="outline" @click="closeLogs">Fechar</Button>
        </header>
        <div class="p-5">
          <form @submit.prevent="submitLogs" class="grid gap-3 md:grid-cols-3">
            <label
              v-if="
                selectedPod && selectedPod.containers.length > 1
              "
              class="text-xs text-muted"
              >Container<select
                v-model="logsForm.container"
                class="mt-1 w-full rounded-lg border border-line bg-slate-950 p-2"
              >
                <option value="">Todos</option>
                <option
                  v-for="container in selectedPod.containers"
                  :key="container.name"
                  :value="container.name"
                >
                  {{ container.name }}
                </option>
              </select></label
            >
            <label class="text-xs text-muted"
              >Tail (linhas)<input
                v-model.number="logsForm.tail"
                type="number"
                min="1"
                max="10000"
                class="mt-1 w-full rounded-lg border border-line bg-slate-950 p-2"
            /></label>
            <Button
              type="submit"
              :disabled="podLogs.isPending.value"
              class="self-end"
              ><FileText class="h-4 w-4" />Carregar Logs</Button
            >
          </form>
          <div
            v-if="logsOutput"
            class="mt-4 max-h-96 overflow-y-auto rounded-lg border border-line bg-slate-950 p-4"
          >
            <pre
              class="whitespace-pre-wrap break-words font-mono text-xs text-slate-300"
              >{{ logsOutput }}</pre
            >
          </div>
          <div
            v-if="podLogs.isError.value"
            class="mt-4 rounded-lg border border-danger/20 bg-danger/5 p-3 text-xs text-danger"
          >
            Erro: {{ podLogs.error.value?.message }}
          </div>
        </div>
      </article>
    </div>
    <div
      v-if="execForm.namespace"
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
            <h2 class="text-lg font-medium">Pod Exec</h2>
            <p class="mt-1 text-xs text-muted">
              {{ execForm.namespace }}/{{ execForm.pod }}
            </p>
          </div>
          <Button variant="outline" @click="closeExec">Fechar</Button>
        </header>
        <div class="p-5">
          <form @submit.prevent="submitExec" class="space-y-4">
            <label
              v-if="
                selectedExecPod &&
                selectedExecPod.containers.length > 1
              "
              class="text-xs text-muted"
              >Container<select
                v-model="execForm.container"
                required
                class="mt-1 w-full rounded-lg border border-line bg-slate-950 p-2"
              >
                <option value="" disabled>Selecione</option>
                <option
                  v-for="container in selectedExecPod.containers"
                  :key="container.name"
                  :value="container.name"
                >
                  {{ container.name }}
                </option>
              </select></label
            >
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
              :disabled="podExec.isPending.value"
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
            v-if="podExec.isError.value"
            class="mt-4 rounded-lg border border-danger/20 bg-danger/5 p-3 text-xs text-danger"
          >
            Erro: {{ podExec.error.value?.message }}
          </div>
        </div>
      </article>
    </div>
  </div>
</template>
