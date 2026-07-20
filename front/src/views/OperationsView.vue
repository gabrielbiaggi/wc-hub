<script setup lang="ts">
import { computed, ref } from 'vue'
import { useQuery } from '@tanstack/vue-query'
import { CheckCircle2, Clock3, Filter, ShieldAlert, Workflow } from 'lucide-vue-next'
import StatusBadge from '@/components/ui/StatusBadge.vue'
import { getOperationCatalog } from '@/lib/api_operations'

const provider = ref('todos')
const catalog = useQuery({ queryKey:['operations-catalog'], queryFn:getOperationCatalog })
const providers = computed(()=>['todos', ...new Set((catalog.data.value??[]).map(item=>item.provider))])
const items = computed(()=> (catalog.data.value??[]).filter(item=>provider.value==='todos'||item.provider===provider.value))
</script>
<template>
  <div class="mx-auto max-w-[1500px] space-y-5">
    <header class="flex flex-col justify-between gap-4 md:flex-row md:items-end"><div><div class="flex items-center gap-2 font-mono text-[10px] uppercase tracking-widest text-signal"><Workflow class="h-3.5 w-3.5"/>plano de paridade operacional</div><h1 class="mt-3 text-3xl font-semibold">Catálogo de operações</h1><p class="mt-2 max-w-3xl text-sm text-muted">Cada operação só vira botão quando tiver contrato tipado, RBAC, confirmação, auditoria e teste. Itens planejados não executam nada.</p></div><div class="flex items-center gap-2"><Filter class="h-4 w-4 text-muted"/><select v-model="provider" class="rounded-lg border border-line bg-panel px-3 py-2 text-sm text-slate-100"><option v-for="item in providers" :key="item" :value="item">{{item}}</option></select></div></header>
    <p v-if="catalog.isError.value" class="rounded-xl border border-danger/30 bg-danger/5 p-4 text-sm text-danger">Não foi possível carregar o catálogo de operações.</p>
    <div v-else class="grid gap-3 lg:grid-cols-2"><article v-for="item in items" :key="item.id" class="rounded-xl border border-line bg-panel/65 p-5"><div class="flex items-start justify-between gap-4"><div><p class="font-mono text-[10px] uppercase text-signal">{{item.provider}} · {{item.resource}}</p><h2 class="mt-2 text-base text-slate-100">{{item.name}}</h2></div><StatusBadge :status="item.status==='available'?'healthy':'warning'" :label="item.status==='available'?'disponível':'planejado'"/></div><dl class="mt-5 grid grid-cols-3 gap-3 border-t border-line pt-4 font-mono text-[10px]"><div><dt class="text-muted">PERMISSÃO</dt><dd class="mt-1 text-slate-200">{{item.permission}}</dd></div><div><dt class="text-muted">CONFIRMAÇÃO</dt><dd class="mt-1 text-slate-200">{{item.confirmation}}</dd></div><div><dt class="text-muted">EXECUÇÃO</dt><dd class="mt-1 flex items-center gap-1 text-slate-200"><CheckCircle2 v-if="item.status==='available'" class="h-3 w-3 text-signal"/><Clock3 v-else class="h-3 w-3 text-warning"/>{{item.execution}}</dd></div></dl><p v-if="item.route" class="mt-4 break-all rounded bg-slate-950/70 p-2 font-mono text-[10px] text-muted">{{item.route}}</p><p v-else class="mt-4 flex items-center gap-2 text-xs text-warning"><ShieldAlert class="h-3.5 w-3.5"/>Aguardando adapter, UI e validação de infraestrutura.</p></article></div>
  </div>
</template>
