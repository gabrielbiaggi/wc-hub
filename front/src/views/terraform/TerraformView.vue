<script setup lang="ts">
import { computed, ref } from 'vue'
import { useMutation, useQuery, useQueryClient } from '@tanstack/vue-query'
import { FileCode2, ListTree, Play, Plus, RefreshCw, Rocket, ShieldCheck, Skull, SquareMinus, TriangleAlert } from 'lucide-vue-next'
import Button from '@/components/ui/Button.vue'
import StatusBadge from '@/components/ui/StatusBadge.vue'
import ActionGuardModal from '@/components/ActionGuardModal.vue'
import { buildActionHeaders } from '@/lib/api'
import { getTerraformRuns, startTerraformRun, type TerraformOperation } from '@/lib/api_terraform'
import { usePermissions } from '@/composables/usePermissions'

const queryClient = useQueryClient()
const workspace = ref('')
const selectedID = ref('')
const guardOperation = ref<TerraformOperation | null>(null)
const { hasPermission } = usePermissions()
const canRead = computed(() => hasPermission('terraform.read'))
const canApply = computed(() => hasPermission('terraform.apply'))
const canManage = computed(() => hasPermission('terraform.manage') || canApply.value)
const query = useQuery({ queryKey: ['terraform-runs'], queryFn: getTerraformRuns, refetchInterval: 5000 })
const runs = computed(() => query.data.value?.items ?? [])
const selected = computed(() => runs.value.find((item) => item.id === selectedID.value) ?? runs.value[0])
const mutation = useMutation({
  mutationFn: (input: { operation: TerraformOperation; headers?: Record<string,string> }) => startTerraformRun(input.operation, workspace.value, input.headers),
  onSuccess: (run) => {
    selectedID.value = run.id
    void queryClient.invalidateQueries({ queryKey: ['terraform-runs'] })
  },
})

const run = (operation: TerraformOperation) => {
  if (!workspace.value) return
  if (!canManage.value) return
  if (operation === 'apply' || operation === 'destroy') { guardOperation.value = operation; return }
  mutation.mutate({ operation })
}
const guardedTarget = computed(() => `terraform/workspace/${workspace.value}`)
const confirmGuardedRun = (payload: { confirmation:string; totpCode:string }) => {
  if (!guardOperation.value) return
  mutation.mutate({ operation: guardOperation.value, headers: buildActionHeaders(payload.confirmation, payload.totpCode) })
  guardOperation.value = null
}
</script>

<template>
  <div v-if="canRead" class="mx-auto max-w-[1500px] space-y-5">
    <header>
      <div class="flex gap-2">
        <StatusBadge :status="query.isError.value ? 'critical' : 'healthy'" :label="query.isError.value ? 'worker indisponível' : 'worker efêmero'" />
        <span class="inline-flex items-center gap-1.5 font-mono text-[10px] uppercase text-signal"><ShieldCheck class="h-3.5 w-3.5" />somente operações tipadas</span>
      </div>
      <h1 class="mt-4 text-3xl font-semibold">Planos Terraform</h1>
      <p class="mt-2 text-sm text-muted">Validação, plano, outputs, aplicação e destruição executados fora da API, com estado isolado e espaços de trabalho permitidos.</p>
    </header>

    <section class="rounded-xl border border-line bg-panel/65 p-5">
      <div class="grid gap-4 md:grid-cols-[1fr_auto_auto_auto_auto_auto]">
        <label class="text-xs text-muted">Espaço de trabalho
          <select v-model="workspace" class="field">
            <option value="" disabled>Selecione um espaço de trabalho</option>
            <option v-for="item in query.data.value?.workspaces" :key="item" :value="item">{{ item }}</option>
          </select>
        </label>
        <Button class="self-end" variant="outline" :disabled="!canManage || !workspace || mutation.isPending.value" @click="run('validate')"><FileCode2 class="h-4 w-4" />Validar</Button>
        <Button class="self-end" :disabled="!canManage || !workspace || mutation.isPending.value" @click="run('plan')"><Play class="h-4 w-4" />Planejar</Button>
        <Button class="self-end" variant="outline" :disabled="!canManage || !workspace || mutation.isPending.value" @click="run('output')"><ListTree class="h-4 w-4" />Outputs</Button>
        <Button class="self-end" variant="danger" :disabled="!canApply || !workspace || mutation.isPending.value" @click="run('apply')"><Rocket class="h-4 w-4" />Aplicar</Button>
        <Button class="self-end" variant="danger" :disabled="!canApply || !workspace || mutation.isPending.value" @click="run('destroy')"><Skull class="h-4 w-4" />Destruir</Button>
      </div>
      <p v-if="query.isError.value" class="mt-4 text-xs text-danger">O worker Terraform não está disponível.</p>
      <p v-else-if="mutation.isError.value" class="mt-4 text-xs text-danger">O worker recusou a solicitação. Confirme o espaço de trabalho e as credenciais efêmeras.</p>
    </section>

    <section v-if="selected" class="grid gap-3 sm:grid-cols-3">
      <article class="rounded-xl border border-signal/20 bg-signal/[.04] p-5"><Plus class="h-4 w-4 text-signal" /><p class="mt-4 font-mono text-3xl">{{ selected.summary.add }}</p><p class="text-xs text-muted">A criar</p></article>
      <article class="rounded-xl border border-warning/20 bg-warning/[.04] p-5"><RefreshCw class="h-4 w-4 text-warning" /><p class="mt-4 font-mono text-3xl">{{ selected.summary.change }}</p><p class="text-xs text-muted">A alterar</p></article>
      <article class="rounded-xl border border-danger/20 bg-danger/[.04] p-5"><SquareMinus class="h-4 w-4 text-danger" /><p class="mt-4 font-mono text-3xl">{{ selected.summary.destroy }}</p><p class="text-xs text-muted">A destruir</p></article>
    </section>

    <section class="grid gap-5 xl:grid-cols-[380px_1fr]">
      <aside class="overflow-hidden rounded-xl border border-line bg-panel/65">
        <header class="border-b border-line p-4"><h2 class="text-sm font-medium">Histórico de execuções</h2></header>
        <div class="divide-y divide-line/60">
          <button v-for="item in runs" :key="item.id" :class="['w-full cursor-pointer p-4 text-left hover:bg-white/[.02]', selected?.id === item.id && 'bg-white/[.04]']" @click="selectedID = item.id">
            <div class="flex items-center gap-2"><p class="flex-1 text-sm text-slate-200">{{ item.workspace }}</p><StatusBadge :status="item.status === 'succeeded' ? 'healthy' : item.status === 'failed' ? 'critical' : 'info'" :label="item.status" /></div>
            <p class="mt-2 font-mono text-[9px] uppercase text-muted">{{ item.operation }} · {{ new Date(item.created_at).toLocaleString('pt-BR') }}</p>
          </button>
          <p v-if="!runs.length" class="p-10 text-center text-xs text-muted">Nenhuma execução.</p>
        </div>
      </aside>
      <article class="min-h-96 rounded-xl border border-line bg-[#070b13] p-5">
        <div class="flex items-center gap-2"><TriangleAlert v-if="selected?.summary.destroy" class="h-4 w-4 text-danger" /><FileCode2 v-else class="h-4 w-4 text-signal" /><h2 class="text-sm font-medium">Saída do plano com dados sensíveis ocultos</h2></div>
        <pre class="mt-5 max-h-[560px] overflow-auto whitespace-pre-wrap break-words font-mono text-[11px] leading-5 text-slate-400">{{ selected?.output || 'Execute validar ou planejar para visualizar a saída redigida do worker.' }}</pre>
      </article>
    </section>
    <ActionGuardModal :show="!!guardOperation" :target-name="guardedTarget" :title="guardOperation === 'destroy' ? 'Destruir workspace' : 'Aplicar plano Terraform'" @cancel="guardOperation = null" @confirm="confirmGuardedRun" />
  </div>
  <div v-else class="rounded-xl border border-warning/30 bg-warning/5 p-6 text-sm text-warning">Sua conta não possui a permissão terraform.read.</div>
</template>
