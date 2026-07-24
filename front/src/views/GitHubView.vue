<script setup lang="ts">
import { computed, ref } from "vue";
import { useMutation, useQuery, useQueryClient } from "@tanstack/vue-query";
import {
  AlertCircle,
  CheckCircle2,
  Clock,
  FolderGit2,
  GitBranch,
  GitCommit,
  Play,
  RefreshCw,
  RotateCcw,
  ShieldCheck,
  Star,
  Tag,
  X,
  XCircle,
} from "lucide-vue-next";
import Button from "@/components/ui/Button.vue";
import StatusBadge from "@/components/ui/StatusBadge.vue";
import {
  createGitHubRelease,
  deleteGitHubRelease,
  getGitHubOverview,
  getGitHubWorkflowRunLogs,
  githubRunAction,
  githubWorkflowAction,
  type GitHubProject,
  type GitHubWorkflowRun,
} from "@/lib/api";
import { apiErrorMessage } from "@/lib/api_error";

const selectedRunForLogs = ref<{ repo: string; runId: number } | null>(null);
const runLogsOutput = ref("");
const runLogsLoading = ref(false);

const openRunLogs = async (repo: string, runId: number) => {
  selectedRunForLogs.value = { repo, runId };
  runLogsOutput.value = "Carregando logs de execução...";
  runLogsLoading.value = true;
  try {
    const res = await getGitHubWorkflowRunLogs(repo, runId);
    runLogsOutput.value = res.logs || "Nenhum log encontrado para esta execução.";
  } catch (err: any) {
    runLogsOutput.value = "Erro ao buscar logs: " + (err.message || String(err));
  } finally {
    runLogsLoading.value = false;
  }
};

const releaseRepo = ref("");
const releaseTagName = ref("");
const releaseTitle = ref("");
const releaseBody = ref("");
const createReleaseMut = useMutation({
  mutationFn: () => createGitHubRelease(releaseRepo.value, {
    tag_name: releaseTagName.value,
    name: releaseTitle.value || releaseTagName.value,
    body: releaseBody.value,
    draft: false,
    prerelease: false,
  }),
  onSuccess: () => {
    releaseTagName.value = "";
    releaseTitle.value = "";
    releaseBody.value = "";
    client.invalidateQueries({ queryKey: ["github-overview"] });
  },
});

const deleteReleaseMut = useMutation({
  mutationFn: (input: { repo: string; releaseId: number }) => deleteGitHubRelease(input.repo, input.releaseId),
  onSuccess: () => client.invalidateQueries({ queryKey: ["github-overview"] }),
});

const client = useQueryClient();
const overview = useQuery({
  queryKey: ["github-overview"],
  queryFn: getGitHubOverview,
  refetchInterval: 30000,
});

const workflowDispatch = useMutation({
  mutationFn: (input: {
    repo: string;
    workflowId: number;
    ref: string;
    inputs?: Record<string, string>;
  }) =>
    githubWorkflowAction(
      input.repo,
      input.workflowId,
      "dispatch",
      input.ref,
      input.inputs,
    ),
  onSuccess: () => {
    setTimeout(
      () => client.invalidateQueries({ queryKey: ["github-overview"] }),
      3000,
    );
  },
});

const runAction = useMutation({
  mutationFn: (input: {
    repo: string;
    runId: number;
    action: "cancel" | "rerun";
  }) => githubRunAction(input.repo, input.runId, input.action),
  onSuccess: () =>
    setTimeout(
      () => client.invalidateQueries({ queryKey: ["github-overview"] }),
      2000,
    ),
});

const providerError = computed(() =>
  apiErrorMessage(
    overview.error.value ??
      workflowDispatch.error.value ??
      runAction.error.value,
    "A operação GitHub falhou.",
  ),
);

const formatDate = (timestamp: string) =>
  new Date(timestamp).toLocaleString("pt-BR");

const dispatchWorkflow = (project: GitHubProject, workflowId: number) => {
  const workflow = project.workflows.find((w) => w.id === workflowId);
  if (!workflow) return;

  const ref = window.prompt(
    `Dispatch workflow: ${workflow.name}\n\nBranch/ref (default: ${project.default_branch}):`,
    project.default_branch,
  );

  if (!ref) return;

  if (
    window.confirm(
      `Confirma dispatch do workflow "${workflow.name}" na ref "${ref}"?`,
    )
  ) {
    workflowDispatch.mutate({
      repo: project.full_name,
      workflowId,
      ref,
    });
  }
};

const cancelRun = (project: GitHubProject, run: GitHubWorkflowRun) => {
  if (
    window.confirm(
      `Confirma cancelamento do workflow run #${run.run_number} "${run.display_title}"?`,
    )
  ) {
    runAction.mutate({
      repo: project.full_name,
      runId: run.id,
      action: "cancel",
    });
  }
};

const rerunRun = (project: GitHubProject, run: GitHubWorkflowRun) => {
  if (
    window.confirm(
      `Confirma rerun do workflow run #${run.run_number} "${run.display_title}"?`,
    )
  ) {
    runAction.mutate({
      repo: project.full_name,
      runId: run.id,
      action: "rerun",
    });
  }
};

const getRunStatusIcon = (run: GitHubWorkflowRun) => {
  if (run.status === "completed") {
    if (run.conclusion === "success") return CheckCircle2;
    if (run.conclusion === "failure") return XCircle;
    if (run.conclusion === "cancelled") return X;
    return AlertCircle;
  }
  if (run.status === "in_progress" || run.status === "queued") return Clock;
  return AlertCircle;
};

const getRunStatusColor = (run: GitHubWorkflowRun) => {
  if (run.status === "completed") {
    if (run.conclusion === "success") return "text-green-400";
    if (run.conclusion === "failure") return "text-red-400";
    if (run.conclusion === "cancelled") return "text-slate-400";
    return "text-warning";
  }
  if (run.status === "in_progress") return "text-blue-400";
  if (run.status === "queued") return "text-muted";
  return "text-warning";
};

const getRunStatus = (run: GitHubWorkflowRun): "healthy" | "critical" | "warning" | "info" => {
  if (run.status === "completed") {
    if (run.conclusion === "success") return "healthy";
    if (run.conclusion === "failure") return "critical";
    return "warning";
  }
  if (run.status === "in_progress") return "info";
  return "info";
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
            :status="overview.isError.value ? 'critical' : 'healthy'"
            :label="
              overview.isError.value
                ? 'GitHub indisponível'
                : 'GitHub API conectada'
            "
          />
          <span
            class="inline-flex items-center gap-1.5 font-mono text-[10px] uppercase text-signal"
            ><ShieldCheck class="h-3.5 w-3.5" />Repositórios allowlistados ·
            auditado</span
          >
        </div>
        <h1 class="mt-4 text-3xl font-semibold">GitHub</h1>
        <p class="mt-2 text-sm text-muted">
          Repositórios, workflows, runs e releases via GitHub API.
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
        workflowDispatch.isError.value ||
        runAction.isError.value
      "
      class="rounded-xl border border-danger/20 bg-danger/5 p-4 text-sm text-danger"
    >
      <p class="font-medium">A operação GitHub falhou</p>
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
            label: 'Repositórios',
            value: overview.data.value?.projects?.length ?? 0,
            icon: FolderGit2,
          },
          {
            label: 'Workflows',
            value:
              overview.data.value?.projects?.reduce(
                (sum, p) => sum + (p.workflows?.length ?? 0),
                0,
              ) ?? 0,
            icon: GitBranch,
          },
          {
            label: 'Runs ativos',
            value:
              overview.data.value?.projects?.reduce(
                (sum, p) =>
                  sum +
                  (p.runs?.filter((r) => r.status !== 'completed').length ??
                    0),
                0,
              ) ?? 0,
            icon: Play,
          },
          {
            label: 'Releases',
            value:
              overview.data.value?.projects?.reduce(
                (sum, p) => sum + (p.releases?.length ?? 0),
                0,
              ) ?? 0,
            icon: Tag,
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
    <div class="space-y-5">
      <article
        v-for="project in overview.data.value?.projects"
        :key="project.full_name"
        class="overflow-hidden rounded-xl border border-line bg-panel/65"
      >
        <header class="border-b border-line bg-slate-950/40 p-5">
          <div class="flex items-start justify-between gap-4">
            <div class="min-w-0 flex-1">
              <h2 class="flex items-center gap-2 text-lg font-medium">
                <FolderGit2 class="h-5 w-5 text-signal" />
                {{ project.name }}
              </h2>
              <p class="mt-1 text-xs text-muted">{{ project.full_name }}</p>
              <p v-if="project.description" class="mt-2 text-sm text-slate-300">
                {{ project.description }}
              </p>
            </div>
            <div class="flex flex-wrap items-center gap-3 text-xs">
              <div
                v-if="project.language"
                class="flex items-center gap-1.5 text-muted"
              >
                <div
                  class="h-2 w-2 rounded-full bg-signal"
                />{{ project.language }}
              </div>
              <div class="flex items-center gap-1.5 text-muted">
                <Star class="h-3.5 w-3.5" />{{ project.stargazers_count }}
              </div>
              <StatusBadge
                :status="project.private ? 'warning' : 'info'"
                :label="project.private ? 'Privado' : 'Público'"
              />
            </div>
          </div>
        </header>
        <div class="grid gap-px bg-line/60 lg:grid-cols-2">
          <div class="bg-panel p-5">
            <h3 class="mb-3 flex items-center gap-2 text-sm font-medium">
              <GitBranch class="h-4 w-4 text-signal" />Workflows
            </h3>
            <div class="space-y-2">
              <div
                v-for="workflow in project.workflows"
                :key="workflow.id"
                class="flex items-center justify-between gap-3 rounded-lg border border-line bg-slate-950/40 p-3"
              >
                <div class="min-w-0 flex-1">
                  <p class="text-sm text-slate-200">{{ workflow.name }}</p>
                  <p class="mt-0.5 font-mono text-[9px] text-muted">
                    {{ workflow.path }} · {{ workflow.state }}
                  </p>
                </div>
                <Button
                  variant="outline"
                  size="sm"
                  :disabled="workflowDispatch.isPending.value"
                  @click="dispatchWorkflow(project, workflow.id)"
                  ><Play class="h-3 w-3" />Dispatch</Button
                >
              </div>
              <p
                v-if="!project.workflows?.length"
                class="py-4 text-center text-xs text-muted"
              >
                Nenhum workflow configurado.
              </p>
            </div>
          </div>
          <div class="bg-panel p-5">
            <h3 class="mb-3 flex items-center gap-2 text-sm font-medium">
              <Clock class="h-4 w-4 text-signal" />Workflow Runs
            </h3>
            <div class="space-y-2">
              <div
                v-for="run in project.runs?.slice(0, 5)"
                :key="run.id"
                class="flex items-center justify-between gap-3 rounded-lg border border-line bg-slate-950/40 p-3"
              >
                <div class="min-w-0 flex-1">
                  <div class="flex items-center gap-2">
                    <component
                      :is="getRunStatusIcon(run)"
                      :class="['h-3.5 w-3.5', getRunStatusColor(run)]"
                    />
                    <p class="truncate text-sm text-slate-200">
                      {{ run.display_title }}
                    </p>
                  </div>
                  <p class="mt-1 font-mono text-[9px] text-muted">
                    #{{ run.run_number }} · {{ run.head_branch }} ·
                    {{ formatDate(run.created_at) }}
                  </p>
                </div>
                <div class="flex gap-1">
                  <Button
                    variant="outline"
                    size="sm"
                    title="Ver logs da execução"
                    @click="openRunLogs(project.full_name, run.id)"
                    >Logs</Button
                  >
                  <Button
                    v-if="run.status !== 'completed'"
                    variant="danger"
                    size="sm"
                    :disabled="runAction.isPending.value"
                    @click="cancelRun(project, run)"
                    ><X class="h-3 w-3" /></Button
                  >
                  <Button
                    v-else-if="run.conclusion === 'failure'"
                    variant="outline"
                    size="sm"
                    :disabled="runAction.isPending.value"
                    @click="rerunRun(project, run)"
                    ><RotateCcw class="h-3 w-3" /></Button
                  >
                </div>
              </div>
              <p
                v-if="!project.runs?.length"
                class="py-4 text-center text-xs text-muted"
              >
                Nenhum run recente.
              </p>
            </div>
          </div>
        </div>

        <div class="border-t border-line/60 bg-panel p-5 space-y-4">
          <h3 class="flex items-center gap-2 text-sm font-medium">
            <Tag class="h-4 w-4 text-signal" />Releases & Tags
          </h3>
          <form class="grid gap-3 md:grid-cols-[140px_180px_1fr_auto]" @submit.prevent="releaseRepo = project.full_name; createReleaseMut.mutate()">
            <input v-model="releaseTagName" required placeholder="Tag (ex: v1.2.0)" class="rounded-lg border border-line bg-slate-950 p-2 text-xs font-mono text-slate-200" />
            <input v-model="releaseTitle" placeholder="Título da Release" class="rounded-lg border border-line bg-slate-950 p-2 text-xs text-slate-200" />
            <input v-model="releaseBody" placeholder="Descrição/Changelog" class="rounded-lg border border-line bg-slate-950 p-2 text-xs text-slate-200" />
            <Button type="submit" :disabled="!releaseTagName.trim() || createReleaseMut.isPending.value"><Plus class="h-4 w-4" />Criar Release</Button>
          </form>

          <div class="space-y-2">
            <div v-for="rel in project.releases" :key="rel.id" class="flex items-center justify-between p-3 rounded-lg border border-line bg-slate-950/40">
              <div>
                <p class="text-sm font-medium text-slate-200">{{ rel.name || rel.tag_name }} <span class="font-mono text-xs text-signal">({{ rel.tag_name }})</span></p>
                <p class="text-[10px] text-muted">Publicado em: {{ formatDate(rel.published_at) }}</p>
              </div>
              <Button variant="danger" size="sm" :disabled="deleteReleaseMut.isPending.value" @click="deleteReleaseMut.mutate({ repo: project.full_name, releaseId: rel.id })"><Trash2 class="h-3.5 w-3.5" /></Button>
            </div>
            <p v-if="!project.releases?.length" class="text-xs text-muted">Nenhuma release publicada neste repositório.</p>
          </div>
        </div>
        <div class="grid gap-px bg-line/60 lg:grid-cols-2">
          <div class="bg-panel p-5">
            <h3 class="mb-3 flex items-center gap-2 text-sm font-medium">
              <GitCommit class="h-4 w-4 text-signal" />Commits Recentes
            </h3>
            <div class="space-y-2">
              <div
                v-for="commit in project.commits?.slice(0, 5)"
                :key="commit.sha"
                class="rounded-lg border border-line bg-slate-950/40 p-3"
              >
                <p class="text-sm text-slate-200">{{ commit.message }}</p>
                <p class="mt-1 font-mono text-[9px] text-muted">
                  {{ commit.author }} · {{ commit.sha.substring(0, 7) }} ·
                  {{ formatDate(commit.timestamp) }}
                </p>
              </div>
              <p
                v-if="!project.commits?.length"
                class="py-4 text-center text-xs text-muted"
              >
                Nenhum commit encontrado.
              </p>
            </div>
          </div>
          <div class="bg-panel p-5">
            <h3 class="mb-3 flex items-center gap-2 text-sm font-medium">
              <Tag class="h-4 w-4 text-signal" />Releases
            </h3>
            <div class="space-y-2">
              <div
                v-for="release in project.releases?.slice(0, 5)"
                :key="release.id"
                class="rounded-lg border border-line bg-slate-950/40 p-3"
              >
                <div class="flex items-center gap-2">
                  <p class="text-sm text-slate-200">{{ release.name }}</p>
                  <span
                    v-if="release.prerelease"
                    class="rounded bg-warning/20 px-1.5 py-0.5 font-mono text-[9px] text-warning"
                    >pre</span
                  >
                  <span
                    v-if="release.draft"
                    class="rounded bg-muted/20 px-1.5 py-0.5 font-mono text-[9px] text-muted"
                    >draft</span
                  >
                </div>
                <p class="mt-1 font-mono text-[9px] text-muted">
                  {{ release.tag_name }} ·
                  {{ formatDate(release.published_at || release.created_at) }}
                </p>
              </div>
              <p
                v-if="!project.releases?.length"
                class="py-4 text-center text-xs text-muted"
              >
                Nenhuma release publicada.
              </p>
            </div>
          </div>
        </div>
        <div
          v-if="project.warnings?.length"
          class="border-t border-line bg-warning/5 p-4 text-xs text-warning"
        >
          {{ project.warnings.join(" · ") }}
        </div>
      </article>
      <p
        v-if="!overview.data.value?.projects?.length"
        class="rounded-xl border border-line bg-panel/65 p-10 text-center text-sm text-muted"
      >
        Nenhum repositório configurado.
      </p>
    </div>

    <div
      v-if="selectedRunForLogs"
      class="fixed inset-0 z-50 flex items-center justify-center bg-black/80 p-4"
      @click.self="selectedRunForLogs = null"
    >
      <article class="w-full max-w-4xl overflow-hidden rounded-xl border border-line bg-panel shadow-2xl">
        <header class="flex items-center justify-between border-b border-line p-4">
          <div>
            <h3 class="text-sm font-medium text-slate-100">Logs de Execução (Action Run #{{ selectedRunForLogs.runId }})</h3>
            <p class="text-xs text-muted">{{ selectedRunForLogs.repo }}</p>
          </div>
          <Button variant="outline" size="sm" @click="selectedRunForLogs = null">Fechar</Button>
        </header>
        <div class="p-5">
          <div class="max-h-[70vh] overflow-y-auto rounded-lg border border-line bg-slate-950 p-4 font-mono text-xs text-slate-200 whitespace-pre-wrap break-words">
            {{ runLogsOutput }}
          </div>
        </div>
      </article>
    </div>
  </div>
</template>
