<script setup lang="ts">
import { computed, ref } from 'vue'
import { useMutation, useQuery, useQueryClient } from '@tanstack/vue-query'
import { Box, Code2, ExternalLink, FileCode2, GitBranch, Lock, PlayCircle, RefreshCw, RotateCcw, ShieldCheck, Square, Tag } from 'lucide-vue-next'
import Button from '@/components/ui/Button.vue'
import StatusBadge from '@/components/ui/StatusBadge.vue'
import { getGitHubCommit, getGitHubCommits, getGitHubOverview, getGitHubWorkflowFile, getGitHubWorkflows, runGitHubWorkflow, runGitHubWorkflowAction, updateGitHubWorkflowFile, type GitHubProject, type GitHubWorkflow } from '@/lib/api_github'
import { traduzirTexto } from '@/lib/ptbr'

const client = useQueryClient()
const selectedRepository = ref('')
const selectedCommitSHA = ref('')
const workflowEditor = ref<{ workflow: GitHubWorkflow; path: string; sha: string; content: string; branch: string; message: string } | null>(null)
const dispatchRef = ref('main')
const query = useQuery({ queryKey: ['github-overview'], queryFn: getGitHubOverview, refetchInterval: 60_000 })
const projects = computed(() => query.data.value?.projects ?? [])
const selected = computed<GitHubProject | undefined>(() => projects.value.find((project) => project.repository.full_name === selectedRepository.value) ?? projects.value[0])
const selectedName = computed(() => selected.value?.repository.full_name ?? '')
const commitsQuery = useQuery({ queryKey: ['github-commits', selectedName], queryFn: () => getGitHubCommits(selectedName.value), enabled: computed(() => !!selectedName.value) })
const workflowsQuery = useQuery({ queryKey: ['github-workflows', selectedName], queryFn: () => getGitHubWorkflows(selectedName.value), enabled: computed(() => !!selectedName.value) })
const commitQuery = useQuery({ queryKey: ['github-commit', selectedName, selectedCommitSHA], queryFn: () => getGitHubCommit(selectedName.value, selectedCommitSHA.value), enabled: computed(() => !!selectedName.value && !!selectedCommitSHA.value) })
const runs = computed(() => projects.value.flatMap((project) => project.workflow_runs.map((run) => ({ ...run, repo: project.repository.full_name }))))
const releases = computed(() => projects.value.flatMap((project) => project.releases.map((release) => ({ ...release, repo: project.repository.full_name }))))
const active = computed(() => runs.value.filter((run) => run.status !== 'completed').length)
const permissionNames = ['admin', 'maintain', 'push', 'triage', 'pull'] as const
const permissionLabel = (permission: typeof permissionNames[number]) => ({ admin:'administrar', maintain:'manter', push:'enviar', triage:'triagem', pull:'baixar' }[permission])
const action = useMutation({
  mutationFn: (input: { repository: string; runID: number; action: 'rerun' | 'cancel' }) => runGitHubWorkflowAction(input.repository, input.runID, input.action),
  onSuccess: () => setTimeout(() => client.invalidateQueries({ queryKey: ['github-overview'] }), 1200),
})
const workflowAction = useMutation({
  mutationFn: (input: { workflow: GitHubWorkflow; operation: 'dispatch' | 'enable' | 'disable' }) => runGitHubWorkflow(selectedName.value, input.workflow.id, input.operation, { ref: dispatchRef.value }),
  onSuccess: () => setTimeout(() => workflowsQuery.refetch(), 800),
})
const workflowSave = useMutation({
  mutationFn: () => updateGitHubWorkflowFile(selectedName.value, workflowEditor.value!),
  onSuccess: () => { workflowEditor.value = null; setTimeout(() => workflowsQuery.refetch(), 800) },
})
const tone = (status: string, conclusion: string) => status !== 'completed' ? 'info' : conclusion === 'success' ? 'healthy' : conclusion === 'failure' ? 'critical' : 'warning'
const execute = (repository: string, runID: number, operation: 'rerun' | 'cancel') => {
  const verb = operation === 'rerun' ? 'reexecutar' : 'cancelar'
  if (window.confirm(`Confirma ${verb} o workflow #${runID} de ${repository}?`)) action.mutate({ repository, runID, action: operation })
}
const decodeBase64 = (content: string) => new TextDecoder().decode(Uint8Array.from(atob(content.replace(/\s/g, '')), (char) => char.charCodeAt(0)))
const editWorkflow = async (workflow: GitHubWorkflow) => {
  const file = await getGitHubWorkflowFile(selectedName.value, workflow.path, selected.value?.repository.default_branch)
  workflowEditor.value = { workflow, path: file.path, sha: file.sha, content: decodeBase64(file.content), branch: selected.value?.repository.default_branch || 'main', message: `chore(actions): atualiza ${workflow.name}` }
}
const operateWorkflow = (workflow: GitHubWorkflow, operation: 'dispatch' | 'enable' | 'disable') => {
  const label = operation === 'dispatch' ? 'executar' : operation === 'enable' ? 'ativar' : 'desativar'
  if (window.confirm(`Confirma ${label} o workflow ${workflow.name}?`)) workflowAction.mutate({ workflow, operation })
}
const saveWorkflow = () => { if (window.confirm('Salvar o YAML criará um novo commit. Confirma?')) workflowSave.mutate() }
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

        <section class="grid gap-5 2xl:grid-cols-2">
          <article class="overflow-hidden rounded-xl border border-line bg-panel/65">
            <header class="border-b border-line p-4"><h2 class="text-sm font-medium">Commits e diferenças</h2><p class="mt-1 text-[10px] text-muted">Selecione um commit para ver arquivos, adições, remoções e o patch completo.</p></header>
            <div class="grid min-h-[380px] lg:grid-cols-[250px_1fr]">
              <div class="max-h-[560px] divide-y divide-line/60 overflow-auto border-r border-line">
                <button v-for="commit in commitsQuery.data.value ?? []" :key="commit.sha" class="w-full p-3 text-left hover:bg-white/[.03]" @click="selectedCommitSHA=commit.sha"><p class="line-clamp-2 text-xs text-slate-200">{{commit.commit.message}}</p><p class="mt-2 font-mono text-[9px] text-muted">{{commit.sha.slice(0,7)}} · {{commit.commit.author.name}}</p></button>
              </div>
              <div class="max-h-[560px] overflow-auto p-4">
                <p v-if="!selectedCommitSHA" class="py-20 text-center text-xs text-muted">Escolha um commit para inspecionar o diff.</p>
                <div v-else-if="commitQuery.data.value" class="space-y-4"><div><p class="text-sm text-white">{{commitQuery.data.value.commit.message}}</p><p class="mt-2 font-mono text-[9px] text-muted">+{{commitQuery.data.value.stats?.additions ?? 0}} / -{{commitQuery.data.value.stats?.deletions ?? 0}}</p></div><div v-for="file in commitQuery.data.value.files ?? []" :key="file.filename" class="rounded-lg border border-line"><header class="flex items-center justify-between border-b border-line p-3"><span class="break-all font-mono text-[10px] text-pulse">{{file.filename}}</span><span class="font-mono text-[9px] text-muted">+{{file.additions}} -{{file.deletions}}</span></header><pre class="overflow-auto p-3 text-[10px] leading-5 text-slate-300">{{file.patch || 'Patch não disponibilizado pela API para este arquivo.'}}</pre></div></div>
              </div>
            </div>
          </article>

          <article class="overflow-hidden rounded-xl border border-line bg-panel/65">
            <header class="border-b border-line p-4"><h2 class="text-sm font-medium">Definições do GitHub Actions</h2><p class="mt-1 text-[10px] text-muted">Execute, ative, desative ou edite o YAML versionado. Toda edição gera um commit.</p></header>
            <div class="divide-y divide-line/60"><div v-for="workflow in workflowsQuery.data.value ?? []" :key="workflow.id" class="p-4"><div class="flex flex-wrap items-center justify-between gap-3"><div><div class="flex items-center gap-2"><FileCode2 class="h-4 w-4 text-signal"/><p class="text-sm text-slate-200">{{workflow.name}}</p></div><p class="mt-1 font-mono text-[9px] text-muted">{{workflow.path}} · {{traduzirTexto(workflow.state)}}</p></div><div class="flex flex-wrap gap-2"><Button size="sm" variant="outline" @click="editWorkflow(workflow)"><Code2 class="h-3.5 w-3.5"/>Editar YAML</Button><Button size="sm" variant="outline" @click="operateWorkflow(workflow, workflow.state === 'active' ? 'disable' : 'enable')">{{workflow.state === 'active' ? 'Desativar' : 'Ativar'}}</Button><Button size="sm" @click="operateWorkflow(workflow,'dispatch')"><PlayCircle class="h-3.5 w-3.5"/>Executar</Button></div></div></div><p v-if="!(workflowsQuery.data.value?.length)" class="p-10 text-center text-xs text-muted">Nenhuma definição encontrada.</p></div>
          </article>
        </section>

        <article class="overflow-hidden rounded-xl border border-line bg-panel/55"><header class="border-b border-line p-4"><h2 class="text-sm font-medium">Releases</h2></header><div class="grid gap-px bg-line/60 sm:grid-cols-2"><a v-for="release in selected.releases" :key="release.id" :href="release.html_url" target="_blank" rel="noreferrer" class="bg-panel p-4 hover:bg-slate-900"><div class="flex items-center gap-2"><Tag class="h-3.5 w-3.5 text-signal"/><p class="text-sm text-slate-200">{{release.name || release.tag_name}}</p></div><p class="mt-2 font-mono text-[9px] text-muted">{{release.tag_name}} · {{release.prerelease ? 'pré-lançamento' : 'estável'}}</p></a><p v-if="!selected.releases.length" class="col-span-2 p-8 text-center text-xs text-muted">Nenhuma release publicada.</p></div></article>
      </div>
    </section>

    <div v-if="workflowEditor" class="fixed inset-0 z-50 grid place-items-center bg-slate-950/85 p-4"><section class="flex max-h-[92vh] w-full max-w-5xl flex-col overflow-hidden rounded-xl border border-line bg-panel shadow-2xl"><header class="flex items-center justify-between border-b border-line p-4"><div><h2 class="text-sm font-medium">Editar {{workflowEditor.workflow.name}}</h2><p class="mt-1 font-mono text-[9px] text-muted">{{workflowEditor.path}}</p></div><Button variant="ghost" @click="workflowEditor=null">Fechar</Button></header><div class="grid gap-3 border-b border-line p-4 md:grid-cols-2"><label class="text-xs text-muted">Ramificação<input v-model="workflowEditor.branch" class="mt-2 w-full rounded-lg border border-line bg-slate-950 p-2 text-slate-200"/></label><label class="text-xs text-muted">Mensagem do commit<input v-model="workflowEditor.message" class="mt-2 w-full rounded-lg border border-line bg-slate-950 p-2 text-slate-200"/></label></div><textarea v-model="workflowEditor.content" spellcheck="false" class="min-h-[480px] flex-1 resize-none bg-slate-950 p-4 font-mono text-xs leading-5 text-slate-200 outline-none"/><footer class="flex justify-end gap-2 border-t border-line p-4"><Button variant="outline" @click="workflowEditor=null">Cancelar</Button><Button :disabled="workflowSave.isPending.value" @click="saveWorkflow">Criar commit com alteração</Button></footer></section></div>
  </div>
</template>
