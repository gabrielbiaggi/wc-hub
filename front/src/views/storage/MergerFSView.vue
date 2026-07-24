<script setup lang="ts">
import { computed, ref } from 'vue'
import { useMutation, useQuery, useQueryClient } from '@tanstack/vue-query'
import { ChevronRight, Download, File, Folder, FolderPlus, HardDrive, Pencil, RefreshCw, Search, ShieldCheck, Trash2, Upload } from 'lucide-vue-next'
import Button from '@/components/ui/Button.vue'
import { browseStorage, createStorageDirectory, deleteStorageEntry, indexStorage, renameStorageEntry, storageStreamURL, uploadStorageFile, type StorageEntry } from '@/lib/api_storage'
import { rebalanceMergerFSPool } from '@/lib/api'

const path = ref('')
const input = ref<HTMLInputElement>()
const queryClient = useQueryClient()
const queryKey = computed(() => ['storage-browse', path.value])
const query = useQuery({ queryKey, queryFn: () => browseStorage(path.value) })
const indexed = ref<number|null>(null)
const indexing = ref(false)
const crumbs = computed(() => path.value.split('/').filter(Boolean))
const rebalanceMut = useMutation({
  mutationFn: () => rebalanceMergerFSPool(),
  onSuccess: () => alert('Rotina de rebalanceamento do pool MergerFS iniciada com sucesso!'),
})
const mutation = useMutation({
  mutationFn: async(payload:{operation:'mkdir'|'upload'|'rename'|'delete';entry?:StorageEntry;file?:File;name?:string})=>{
    if(payload.operation==='mkdir')return createStorageDirectory(path.value,payload.name??'')
    if(payload.operation==='upload'&&payload.file)return uploadStorageFile(path.value,payload.file)
    if(payload.operation==='rename'&&payload.entry)return renameStorageEntry(payload.entry.path,payload.name??'')
    if(payload.operation==='delete'&&payload.entry)return deleteStorageEntry(payload.entry.path)
  },
  onSuccess:()=>queryClient.invalidateQueries({queryKey:['storage-browse',path.value]}),
})
const open=(entry:StorageEntry)=>{if(entry.directory)path.value=entry.path}
const go=(index:number)=>{path.value=crumbs.value.slice(0,index+1).join('/')}
const format=(value:number)=>{if(!value)return'0 B';const units=['B','KB','MB','GB','TB'],index=Math.min(Math.floor(Math.log(value)/Math.log(1024)),4);return`${(value/1024**index).toFixed(index>1?1:0)} ${units[index]}`}
const runIndex=async()=>{indexing.value=true;try{indexed.value=(await indexStorage(path.value)).length}finally{indexing.value=false}}
const mkdir=()=>{const name=window.prompt('Nome da nova pasta:');if(name)mutation.mutate({operation:'mkdir',name})}
const rename=(entry:StorageEntry)=>{const name=window.prompt('Novo nome:',entry.name);if(name&&name!==entry.name)mutation.mutate({operation:'rename',entry,name})}
const remove=(entry:StorageEntry)=>{if(window.confirm(`Excluir ${entry.directory?'a pasta vazia':'o arquivo'} ${entry.name}?`))mutation.mutate({operation:'delete',entry})}
const selectedFile=()=>{const file=input.value?.files?.[0];if(file)mutation.mutate({operation:'upload',file});if(input.value)input.value.value=''}
</script>

<template>
  <div class="mx-auto max-w-[1500px] space-y-5">
    <header class="flex flex-col justify-between gap-4 md:flex-row md:items-end"><div><div class="flex items-center gap-2 font-mono text-[10px] uppercase text-signal"><ShieldCheck class="h-3.5 w-3.5"/>raiz gravável e restrita</div><h1 class="mt-3 text-3xl font-semibold">Navegador MergerFS</h1><p class="mt-2 text-sm text-muted">Navegação, envio, organização, indexação e transmissão dentro da raiz autorizada.</p></div><div class="flex flex-wrap gap-2"><input ref="input" type="file" class="hidden" @change="selectedFile"/><Button variant="outline" :disabled="rebalanceMut.isPending.value" @click="rebalanceMut.mutate()"><RefreshCw class="h-4 w-4"/>Rebalancear Pool</Button><Button variant="outline" :disabled="mutation.isPending.value" @click="input?.click()"><Upload class="h-4 w-4"/>Enviar arquivo</Button><Button variant="outline" :disabled="mutation.isPending.value" @click="mkdir"><FolderPlus class="h-4 w-4"/>Nova pasta</Button><Button variant="outline" :disabled="indexing" @click="runIndex"><Search class="h-4 w-4"/>{{indexing?'Indexando…':'Indexar'}}</Button><Button variant="outline" @click="query.refetch()"><RefreshCw class="h-4 w-4"/>Atualizar</Button></div></header>
    <div v-if="indexed!==null" class="rounded-xl border border-signal/15 bg-signal/[.04] p-3 text-xs text-signal">{{indexed}} entradas indexadas.</div>
    <div v-if="query.isError.value||mutation.isError.value" class="rounded-xl border border-danger/20 bg-danger/5 p-4 text-sm text-danger">A operação de armazenamento falhou. Pastas só podem ser excluídas quando vazias e todos os caminhos devem permanecer dentro da raiz.</div>
    <section class="overflow-hidden rounded-xl border border-line bg-panel/65">
      <header class="flex items-center gap-2 border-b border-line p-4 font-mono text-[10px]"><button class="cursor-pointer text-signal" @click="path=''">RAIZ</button><template v-for="(crumb,index) in crumbs" :key="`${crumb}-${index}`"><ChevronRight class="h-3 w-3 text-muted"/><button class="cursor-pointer text-slate-300 hover:text-white" @click="go(index)">{{crumb}}</button></template></header>
      <div v-if="query.isLoading.value" class="grid min-h-72 place-items-center"><div class="h-8 w-8 animate-spin rounded-full border-2 border-line border-t-signal"/></div>
      <div v-else class="divide-y divide-line/60">
        <div v-for="entry in query.data.value?.items" :key="entry.path" class="grid gap-3 px-5 py-3.5 hover:bg-white/[.02] md:grid-cols-[1fr_110px_170px_130px] md:items-center">
          <button class="flex min-w-0 cursor-pointer items-center gap-3 text-left" @click="open(entry)"><div class="grid h-9 w-9 shrink-0 place-items-center rounded-lg border border-line bg-slate-950/50"><Folder v-if="entry.directory" class="h-4 w-4 text-warning"/><File v-else class="h-4 w-4 text-pulse"/></div><div class="min-w-0"><p class="truncate text-sm text-slate-200">{{entry.name}}</p><p class="mt-1 truncate font-mono text-[9px] text-muted">{{entry.path}} · {{entry.mime_type||(entry.directory?'diretório':'binário')}}</p></div></button>
          <p class="font-mono text-[10px] text-muted">{{entry.directory?'—':format(entry.size)}}</p><p class="text-[10px] text-muted">{{new Date(entry.modified_at).toLocaleString('pt-BR')}}</p>
          <div class="flex justify-end gap-1"><a v-if="!entry.directory" :href="storageStreamURL(entry.path)" target="_blank" rel="noreferrer" class="grid h-8 w-8 place-items-center rounded-lg text-muted hover:bg-white/5 hover:text-white" title="Abrir transmissão"><Download class="h-4 w-4"/></a><button class="grid h-8 w-8 place-items-center rounded-lg text-muted hover:bg-white/5 hover:text-white" title="Renomear" @click="rename(entry)"><Pencil class="h-3.5 w-3.5"/></button><button class="grid h-8 w-8 place-items-center rounded-lg text-danger hover:bg-danger/10" title="Excluir" @click="remove(entry)"><Trash2 class="h-3.5 w-3.5"/></button></div>
        </div>
        <div v-if="!query.data.value?.items.length" class="grid min-h-64 place-items-center text-center"><div><HardDrive class="mx-auto h-7 w-7 text-muted"/><p class="mt-3 text-sm text-muted">Pasta vazia.</p></div></div>
      </div>
    </section>
  </div>
</template>
