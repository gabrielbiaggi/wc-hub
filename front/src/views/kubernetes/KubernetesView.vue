<script setup lang="ts">
import { computed, ref } from "vue";
import { useMutation, useQuery, useQueryClient } from "@tanstack/vue-query";
import {
  Activity,
  Boxes,
  FileText,
  Minus,
  Plus,
  RefreshCw,
  RotateCcw,
  Server,
  ShieldCheck,
  Terminal,
  TriangleAlert,
} from "lucide-vue-next";
import Button from "@/components/ui/Button.vue";
import StatusBadge from "@/components/ui/StatusBadge.vue";
import {
  execKubernetesPod,
  getKubernetesOverview,
  getKubernetesPodLogs,
  runKubernetesDeploymentAction,
  type KubeDeployment,
  type KubeNode,
  type KubePod,
} from "@/lib/api_kubernetes";
import { apiErrorMessage } from "@/lib/api_error";

const query = useQuery({
  queryKey: ["kubernetes-overview"],
  queryFn: getKubernetesOverview,
  refetchInterval: 20000,
});
const queryClient = useQueryClient();
const action = useMutation({
  mutationFn: (input: {
    namespace: string;
    name: string;
    action: "scale" | "restart";
    replicas?: number;
  }) =>
    runKubernetesDeploymentAction(
      input.namespace,
      input.name,
      input.action,
      input.replicas,
    ),
  onSuccess: () =>
    setTimeout(
      () =>
        queryClient.invalidateQueries({ queryKey: ["kubernetes-overview"] }),
      1000,
    ),
});
const terminal = ref<{
  pod: KubePod;
  container: string;
  command: string;
  output: string;
} | null>(null);
const podExec = useMutation({
  mutationFn: () =>
    execKubernetesPod(
      terminal.value!.pod.metadata.namespace,
      terminal.value!.pod.metadata.name,
      terminal.value!.container,
      ["sh", "-lc", terminal.value!.command],
    ),
  onSuccess: (result) => {
    if (terminal.value) terminal.value.output = result.output;
  },
});
const podLogs = useMutation({
  mutationFn: () =>
    getKubernetesPodLogs(
      terminal.value!.pod.metadata.namespace,
      terminal.value!.pod.metadata.name,
      terminal.value!.container,
    ),
  onSuccess: (result) => {
    if (terminal.value) terminal.value.output = result.output;
  },
});
const data = computed(() => query.data.value);
const clusterError = computed(() =>
  apiErrorMessage(
    query.error.value ??
      action.error.value ??
      podExec.error.value ??
      podLogs.error.value,
    "Não foi possível consultar a API Kubernetes.",
  ),
);
const ready = (node: KubeNode) =>
  node.status.conditions?.find((condition) => condition.type === "Ready")
    ?.status === "True";
const readyNodes = computed(() => data.value?.nodes.filter(ready).length ?? 0);
const readyDeployments = computed(
  () =>
    data.value?.deployments.filter(
      (deployment) =>
        (deployment.status.readyReplicas ?? 0) >=
        (deployment.spec.replicas ?? 0),
    ).length ?? 0,
);
const execute = (
  deployment: KubeDeployment,
  operation: "scale" | "restart",
  replicas?: number,
) => {
  if (
    window.confirm(
      `Confirma a operação em ${deployment.metadata.namespace}/${deployment.metadata.name}${replicas === undefined ? "" : ` para ${replicas} réplica(s)`}?`,
    )
  )
    action.mutate({
      namespace: deployment.metadata.namespace,
      name: deployment.metadata.name,
      action: operation,
      replicas,
    });
};
const openTerminal = (pod: KubePod) => {
  terminal.value = {
    pod,
    container: pod.status.containerStatuses?.[0]?.name || "",
    command: "id && uname -a",
    output: "",
  };
};
const executePod = () => {
  if (
    terminal.value &&
    window.confirm(
      `Executar comando em ${terminal.value.pod.metadata.namespace}/${terminal.value.pod.metadata.name}?`,
    )
  )
    podExec.mutate();
};
</script>

<template>
  <div class="mx-auto max-w-[1680px] space-y-5">
    <header
      class="flex flex-col justify-between gap-4 md:flex-row md:items-end"
    >
      <div>
        <div class="flex gap-2">
          <StatusBadge
            :status="query.isError.value ? 'critical' : 'healthy'"
            :label="
              query.isError.value
                ? 'cluster indisponível'
                : 'operador do cluster'
            "
          /><span
            class="inline-flex items-center gap-1.5 font-mono text-[10px] uppercase text-signal"
            ><ShieldCheck class="h-3.5 w-3.5" />Kubernetes RBAC</span
          >
        </div>
        <h1 class="mt-4 text-3xl font-semibold">Malha Kubernetes</h1>
        <p class="mt-2 text-sm text-muted">
          Nós, deployments, pods problemáticos e eventos do cluster autorizado.
        </p>
      </div>
      <Button
        variant="outline"
        :disabled="query.isFetching.value"
        @click="query.refetch()"
        ><RefreshCw
          :class="['h-4 w-4', query.isFetching.value && 'animate-spin']"
        />Atualizar</Button
      >
    </header>

    <div
      v-if="
        query.isError.value ||
        action.isError.value ||
        podExec.isError.value ||
        podLogs.isError.value
      "
      class="rounded-xl border border-danger/20 bg-danger/5 p-4 text-sm text-danger"
    >
      <p class="font-medium">Falha no adaptador Kubernetes</p>
      <p class="mt-1 break-words font-mono text-xs">{{ clusterError }}</p>
    </div>
    <section class="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
      <article
        v-for="item in [
          {
            l: 'Nós prontos',
            v: `${readyNodes}/${data?.nodes.length ?? 0}`,
            i: Server,
          },
          {
            l: 'Deployments prontos',
            v: `${readyDeployments}/${data?.deployments.length ?? 0}`,
            i: Boxes,
          },
          {
            l: 'Pods com problemas',
            v: data?.problem_pods.length ?? 0,
            i: TriangleAlert,
          },
          { l: 'Eventos de alerta', v: data?.events.length ?? 0, i: Activity },
        ]"
        :key="item.l"
        class="rounded-xl border border-line bg-panel/65 p-5"
      >
        <component :is="item.i" class="h-4 w-4 text-muted" />
        <p class="mt-5 font-mono text-2xl text-white">{{ item.v }}</p>
        <p class="mt-1 text-xs text-muted">{{ item.l }}</p>
      </article>
    </section>

    <section class="grid gap-5 xl:grid-cols-[1fr_1.2fr]">
      <article
        class="overflow-hidden rounded-xl border border-line bg-panel/65"
      >
        <header class="border-b border-line p-4">
          <h2 class="text-sm font-medium">Nós do cluster</h2>
        </header>
        <div class="divide-y divide-line/60">
          <div
            v-for="node in data?.nodes"
            :key="node.metadata.uid"
            class="flex items-center gap-3 p-4"
          >
            <Server class="h-4 w-4 text-pulse" />
            <div class="min-w-0 flex-1">
              <p class="text-sm text-slate-200">{{ node.metadata.name }}</p>
              <p class="mt-1 text-[10px] text-muted">
                {{ node.status.nodeInfo?.kubeletVersion }} ·
                {{ node.status.nodeInfo?.architecture }}
              </p>
            </div>
            <StatusBadge
              :status="ready(node) ? 'healthy' : 'critical'"
              :label="ready(node) ? 'pronto' : 'não pronto'"
            />
          </div>
          <p
            v-if="!data?.nodes.length"
            class="p-10 text-center text-sm text-muted"
          >
            Nenhum nó retornado.
          </p>
        </div>
      </article>
      <article
        class="overflow-hidden rounded-xl border border-line bg-panel/65"
      >
        <header class="border-b border-line p-4">
          <h2 class="text-sm font-medium">Implantações</h2>
        </header>
        <div class="divide-y divide-line/60">
          <div
            v-for="deployment in data?.deployments"
            :key="deployment.metadata.uid"
            class="grid gap-3 p-4 md:grid-cols-[1fr_90px_250px] md:items-center"
          >
            <div>
              <p class="text-sm text-slate-200">
                {{ deployment.metadata.name }}
              </p>
              <p class="mt-1 font-mono text-[9px] text-muted">
                {{ deployment.metadata.namespace }} · disponíveis
                {{ deployment.status.availableReplicas ?? 0 }} · indisponíveis
                {{ deployment.status.unavailableReplicas ?? 0 }}
              </p>
            </div>
            <StatusBadge
              :status="
                (deployment.status.readyReplicas ?? 0) >=
                (deployment.spec.replicas ?? 0)
                  ? 'healthy'
                  : 'warning'
              "
              :label="`${deployment.status.readyReplicas ?? 0}/${deployment.spec.replicas ?? 0}`"
            />
            <div class="flex justify-end gap-2">
              <Button
                variant="outline"
                :disabled="
                  action.isPending.value || deployment.spec.replicas <= 0
                "
                @click="
                  execute(deployment, 'scale', deployment.spec.replicas - 1)
                "
                ><Minus class="h-3.5 w-3.5" /></Button
              ><Button
                variant="outline"
                :disabled="action.isPending.value"
                @click="
                  execute(deployment, 'scale', deployment.spec.replicas + 1)
                "
                ><Plus class="h-3.5 w-3.5" /></Button
              ><Button
                variant="outline"
                :disabled="action.isPending.value"
                @click="execute(deployment, 'restart')"
                ><RotateCcw class="h-3.5 w-3.5" />Reiniciar</Button
              >
            </div>
          </div>
        </div>
      </article>
    </section>

    <section
      v-if="data?.problem_pods.length"
      class="rounded-xl border border-danger/20 bg-danger/[.03]"
    >
      <header class="border-b border-danger/10 p-4">
        <h2 class="text-sm font-medium">Pods que exigem atenção</h2>
      </header>
      <div class="divide-y divide-line/60">
        <div
          v-for="pod in data.problem_pods"
          :key="pod.metadata.uid"
          class="flex items-center gap-3 p-4"
        >
          <TriangleAlert class="h-4 w-4 text-danger" />
          <div class="flex-1">
            <p class="text-sm text-slate-200">{{ pod.metadata.name }}</p>
            <p class="text-[10px] text-muted">
              {{ pod.metadata.namespace }} ·
              {{ pod.status.reason || pod.status.phase }}
            </p>
          </div>
          <StatusBadge status="critical" :label="pod.status.phase" /><Button
            variant="outline"
            @click="openTerminal(pod)"
            ><Terminal class="h-3.5 w-3.5" />Diagnosticar</Button
          >
        </div>
      </div>
    </section>

    <section class="overflow-hidden rounded-xl border border-line bg-panel/65">
      <header class="border-b border-line p-4">
        <h2 class="text-sm font-medium">Todos os pods e terminais</h2>
        <p class="mt-1 text-[10px] text-muted">
          Abra logs ou execute comandos pontuais nos containers autorizados pelo
          ServiceAccount.
        </p>
      </header>
      <div class="grid gap-px bg-line/60 md:grid-cols-2 xl:grid-cols-3">
        <button
          v-for="pod in data?.pods"
          :key="pod.metadata.uid"
          class="bg-panel p-4 text-left hover:bg-slate-900"
          @click="openTerminal(pod)"
        >
          <div class="flex items-center justify-between">
            <div>
              <p class="text-sm text-slate-200">{{ pod.metadata.name }}</p>
              <p class="mt-1 font-mono text-[9px] text-muted">
                {{ pod.metadata.namespace }} ·
                {{ pod.status.containerStatuses?.length || 0 }} container(s)
              </p>
            </div>
            <StatusBadge
              :status="
                pod.status.phase === 'Running'
                  ? 'healthy'
                  : pod.status.phase === 'Succeeded'
                    ? 'info'
                    : 'warning'
              "
              :label="pod.status.phase"
            />
          </div>
        </button>
      </div>
    </section>

    <section
      v-if="data?.events.length"
      class="overflow-hidden rounded-xl border border-warning/20 bg-warning/[.03]"
    >
      <header class="border-b border-warning/10 p-4">
        <h2 class="text-sm font-medium">Eventos de alerta recentes</h2>
      </header>
      <div class="divide-y divide-line/60">
        <div
          v-for="event in data.events"
          :key="event.metadata.uid"
          class="flex items-start gap-3 p-4"
        >
          <Activity class="mt-0.5 h-4 w-4 shrink-0 text-warning" />
          <div class="min-w-0 flex-1">
            <div class="flex flex-wrap items-center gap-2">
              <p class="text-sm text-slate-200">
                {{ event.reason || "Alerta" }}
              </p>
              <span class="font-mono text-[9px] uppercase text-muted"
                >{{ event.regarding?.kind || event.involvedObject?.kind }}
                {{ event.regarding?.name || event.involvedObject?.name }}</span
              >
            </div>
            <p class="mt-1 text-xs text-muted">{{ event.message }}</p>
          </div>
          <span v-if="event.count" class="font-mono text-[10px] text-warning"
            >×{{ event.count }}</span
          >
        </div>
      </div>
    </section>
    <div
      v-if="terminal"
      class="fixed inset-0 z-50 grid place-items-center bg-slate-950/85 p-4"
    >
      <section
        class="w-full max-w-5xl overflow-hidden rounded-xl border border-line bg-panel"
      >
        <header
          class="flex items-center justify-between border-b border-line p-4"
        >
          <div>
            <h2 class="text-sm text-white">
              Pod · {{ terminal.pod.metadata.namespace }}/{{
                terminal.pod.metadata.name
              }}
            </h2>
            <p class="mt-1 text-[10px] text-warning">
              Logs e exec são limitados; não há TTY persistente nem entrada
              interativa.
            </p>
          </div>
          <Button variant="ghost" @click="terminal = null">Fechar</Button>
        </header>
        <div
          class="grid gap-3 border-b border-line p-4 md:grid-cols-[220px_1fr_auto_auto]"
        >
          <select
            v-model="terminal.container"
            class="rounded-lg border border-line bg-slate-950 p-2 text-xs"
          >
            <option
              v-for="item in terminal.pod.status.containerStatuses"
              :key="item.name"
            >
              {{ item.name }}
            </option></select
          ><input
            v-model="terminal.command"
            class="rounded-lg border border-line bg-slate-950 p-2 font-mono text-xs"
          /><Button
            variant="outline"
            :disabled="podLogs.isPending.value"
            @click="podLogs.mutate()"
            ><FileText class="h-4 w-4" />Logs</Button
          ><Button :disabled="podExec.isPending.value" @click="executePod"
            ><Terminal class="h-4 w-4" />Executar</Button
          >
        </div>
        <pre
          class="min-h-80 max-h-[55vh] overflow-auto bg-slate-950 p-4 font-mono text-xs leading-5 text-slate-200"
          >{{ terminal.output || "A saída aparecerá aqui." }}</pre>
      </section>
    </div>
  </div>
</template>
