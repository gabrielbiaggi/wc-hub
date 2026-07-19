<script setup lang="ts">
import { computed, ref } from 'vue'
import { useMutation, useQuery, useQueryClient } from '@tanstack/vue-query'
import { Box, ExternalLink, GitBranch, Lock, PlayCircle, RefreshCw, RotateCcw, ShieldCheck, Square, Star, Tag } from 'lucide-vue-next'
import Button from '@/components/ui/Button.vue'
import StatusBadge from '@/components/ui/StatusBadge.vue'
import { getGitHubOverview, runGitHubWorkflowAction, type GitHubProject } from '@/lib/api_github'
import { traduzirTexto } from '@/lib/ptbr'

const client = useQueryClient()
const selectedRepository = ref('')
const query = useQuery({ queryKey: ['github-overview'], queryFn: getGitHubOverview, refetchInterval: 60_000 })
const projects = computed(() => query.data.value?.projects ?? [])
const selected = computed<GitHubProject | undefined>(() => projects.value.find((project) => project.repository.full_name === selectedRepository.value) ?? projects.value[0])
const runs = computed(() => projects.value.flatMap((project) => project.workflow_runs.map((run) => ({ ...run, repo: project.repository.full_name }))))
const releases = computed(() => projects.value.flatMap((project) => project.releases.map((release) => ({ ...release, repo: project.repository.full_name }))))
const active = computed(() => runs.value.filter((run) => run.status !== 'completed').length)
const permissionNames = ['admin', 'maintain', 'push', 'triage', 'pull'] as const
const permissionLabel = (permission: typeof permissionNames[number]) => ({ admin:'administrar', maintain:'manter', push:'enviar', triage:'triagem', pull:'baixar' }[permission])
const action = useMutation({
  mutationFn: (input: { repository: string; runID: number; action: 'rerun' | 'cancel' }) => runGitHubWorkflowAction(input.repository, input.runID, input.action),
  onSuccess: () => setTimeout(() => client.invalidateQueries({ queryKey: ['github-overview'] }), 1200),
})
const tone = (status: string, conclusion: string) => status !== 'completed' ? 'info' : conclusion === 'success' ? 'healthy' : conclusion === 'failure' ? 'critical' : 'warning'
const execute = (repository: string, runID: number, operation: 'rerun' | 'cancel') => {
  const verb = operation === 'rerun' ? 'reexecutar' : 'cancelar'
  if (window.confirm(`Confirma ${verb} o workflow #${runID} de ${repository}?`)) action.mutate({ repository, runID, action: operation })
}
</script>

<template>
  <div class="mx-auto max-w-[1680px] space-y-5">
    <header class="flex flex-col justify-between gap-4 md:flex-row md:items-end">
      <div>
        <div class="flex flex-wrap gap-2"><StatusBadge :status="query.isError.value ? 'critical' : 'healthy'" :label="query.isError.value ? 'provedor indisponível' : '12 repositórios autorizados'"/><span class="inline-flex items-center gap-1.5 font-mono text-[10px] uppercase text-signal"><ShieldCheck class="h-3.5 w-3.5"/>token administrador · lista explícita</span></div>
        <h1 class="mt-4 text-3xl font-semibold">Entrega pelo GitHub</h1>
        <p class="mt-2 text-sm text-muted">Permissões efetivas, CI/CD, releases e controle operacional de Actions.</p>
      </div>
      <Button variant="outline" :disabled="query.isFetching.value" @click="query.refetch()"><RefreshCw :class="['h-4 w-4', query.isFetching.value && 'animate-spin']"/>Atualizar</Button>
    </header>

    <div v-if="query.isError.value" class="rounded-xl border border-danger/20 bg-danger/5 p-4 text-sm text-danger">A API GitHub não respondeu. O timeout foi ampliado e a coleta agora é paralela; tente atualizar.</div>
    <div v-if="query.data.value?.warnings.length" class="rounded-xl border border-warning/20 bg-warning/5 p-4 text-xs text-warning">{{ query.data.value.warnings.join(' · ') }}</div>
    <div v-if="action.isError.value" class="rounded-xl border border-danger/20 bg-danger/5 p-4 text-sm text-danger">A operação foi rejeitada pelo GitHub. Confirme o estado atual do workflow.</div>

    <section class="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
      <article v-for="item in [{l:'Repositórios',v:projects.length,i:Box},{l:'Workflows ativos',v:active,i:PlayCircle},{l:'Releases recentes',v:releases.length,i:Tag},{l:'Acesso administrativo',v:projects.filter(p=>p.repository.permissions?.admin).length,i:ShieldCheck}]" :key="item.l" class="rounded-xl border border-line bg-panel/65 p-5"><component :is="item.i" class="h-4 w-4 text-muted"/><p class="mt-5 font-mono text-2xl">{{item.v}}</p><p class="text-xs text-muted">{{item.l}}</p></article>
    </section>

    <section class="grid gap-5 xl:grid-cols-[360px_1fr]">
      <aside class="overflow-hidden rounded-xl border border-line bg-panel/55">
        <header class="border-b border-line p-4"><h2 class="text-sm font-medium">Repositórios autorizados</h2><p class="mt-1 text-[10px] text-muted">Selecione para inspecionar todos os detalhes.</p></header>
        <div class="max-h-[620px] divide-y divide-line/60 overflow-auto">
          <button v-for="project in projects" :key="project.repository.id" :class="['w-full p-4 text-left transition-colors hover:bg-white/[.03]', selected?.repository.id === project.repository.id && 'bg-signal/[.06]']" @click="selectedRepository = project.repository.full_name">
            <div class="flex items-center gap-2"><Lock v-if="project.repository.private" class="h-3.5 w-3.5 text-warning"/><Box v-else class="h-3.5 w-3.5 text-pulse"/><p class="truncate text-sm text-slate-200">{{project.repository.full_name}}</p></div>
            <div class="mt-2 flex gap-3 font-mono text-[9px] uppercase text-muted"><span>{{project.repository.language || 'mista'}}</span><span>{{project.workflow_runs.length}} execuções</span><span>{{project.releases.length}} releases</span></div>
          </button>
        </div>
      </aside>

      <div v-if="selected" class="space-y-5">
        <article class="rounded-xl border border-line bg-panel/65 p-5">
          <div class="flex flex-col justify-between gap-4 md:flex-row"><div><div class="flex items-center gap-2"><h2 class="text-lg font-medium text-white">{{selected.repository.full_name}}</h2><StatusBadge :status="selected.repository.permissions?.admin ? 'healthy' : 'warning'" :label="selected.repository.permissions?.admin ? 'administrador' : 'limitado'"/></div><p class="mt-2 max-w-3xl text-xs leading-5 text-muted">{{selected.repository.description || 'Sem descrição.'}}</p></div><a :href="selected.repository.html_url" target="_blank" rel="noreferrer" class="inline-flex h-9 items-center gap-2 rounded-lg border border-line px-3 text-xs text-muted hover:text-white">Abrir GitHub<ExternalLink class="h-3.5 w-3.5"/></a></div>
          <div class="mt-5 grid gap-3 sm:grid-cols-2 lg:grid-cols-5"><div v-for="detail in [{k:'Visibilidade',v:traduzirTexto(selected.repository.visibility)},{k:'Ramificação padrão',v:selected.repository.default_branch},{k:'Problemas abertos',v:selected.repository.open_issues_count},{k:'Estrelas',v:selected.repository.stargazers_count},{k:'Tamanho',v:`${selected.repository.size} KB`}]" :key="detail.k" class="rounded-lg border border-line/70 bg-slate-950/30 p-3"><p class="font-mono text-[9px] uppercase text-muted">{{detail.k}}</p><p class="mt-2 truncate text-sm text-slate-200">{{detail.v}}</p></div></div>
          <div class="mt-4 flex flex-wrap gap-2 font-mono text-[9px] uppercase"><span v-for="permission in permissionNames" :key="permission" :class="['rounded border px-2 py-1', selected.repository.permissions?.[permission] ? 'border-signal/20 text-signal' : 'border-line text-muted']">{{permissionLabel(permission)}}</span></div>
        </article>

        <article class="overflow-hidden rounded-xl border border-line bg-panel/65"><header class="border-b border-line p-4"><h2 class="text-sm font-medium">Execuções de workflow</h2></header><div class="divide-y divide-line/60"><div v-for="run in selected.workflow_runs" :key="run.id" class="grid gap-3 p-4 lg:grid-cols-[1fr_130px_190px] lg:items-center"><div class="flex gap-3"><GitBranch class="mt-0.5 h-4 w-4 text-pulse"/><div><a :href="run.html_url" target="_blank" rel="noreferrer" class="text-sm text-slate-200 hover:text-white">{{run.display_title || run.name}}</a><p class="mt-1 font-mono text-[9px] text-muted">{{run.head_branch}} · {{run.event}} · #{{run.run_number}} · {{run.head_sha.slice(0,7)}}</p></div></div><StatusBadge :status="tone(run.status,run.conclusion)" :label="run.conclusion || run.status"/><div class="flex justify-end gap-2"><Button v-if="run.status === 'completed'" size="sm" variant="outline" :disabled="action.isPending.value" @click="execute(selected.repository.full_name,run.id,'rerun')"><RotateCcw class="h-3.5 w-3.5"/>Reexecutar</Button><Button v-else size="sm" variant="danger" :disabled="action.isPending.value" @click="execute(selected.repository.full_name,run.id,'cancel')"><Square class="h-3.5 w-3.5"/>Cancelar</Button></div></div><p v-if="!selected.workflow_runs.length" class="p-10 text-center text-sm text-muted">Nenhum workflow encontrado.</p></div></article>

        <article class="overflow-hidden rounded-xl border border-line bg-panel/55"><header class="border-b border-line p-4"><h2 class="text-sm font-medium">Releases</h2></header><div class="grid gap-px bg-line/60 sm:grid-cols-2"><a v-for="release in selected.releases" :key="release.id" :href="release.html_url" target="_blank" rel="noreferrer" class="bg-panel p-4 hover:bg-slate-900"><div class="flex items-center gap-2"><Tag class="h-3.5 w-3.5 text-signal"/><p class="text-sm text-slate-200">{{release.name || release.tag_name}}</p></div><p class="mt-2 font-mono text-[9px] text-muted">{{release.tag_name}} · {{release.prerelease ? 'pré-lançamento' : 'estável'}}</p></a><p v-if="!selected.releases.length" class="col-span-2 p-8 text-center text-xs text-muted">Nenhuma release publicada.</p></div></article>
      </div>
    </section>
  </div>
</template>
