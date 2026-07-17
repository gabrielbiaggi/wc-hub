<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'
import { useRoute } from 'vue-router'
import { Menu, Search, ShieldCheck, Bell, Command, X } from 'lucide-vue-next'
import AppSidebar from './AppSidebar.vue'
import Button from '@/components/ui/Button.vue'
import { useUiStore } from '@/stores/ui'

const ui = useUiStore()
const route = useRoute()
const now = ref(new Date())
let timer: number
const onKey = (event: KeyboardEvent) => {
  if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === 'k') { event.preventDefault(); ui.commandOpen = !ui.commandOpen }
  if (event.key === 'Escape') ui.commandOpen = false
}
onMounted(() => { timer = window.setInterval(() => now.value = new Date(), 1000); window.addEventListener('keydown', onKey) })
onUnmounted(() => { clearInterval(timer); window.removeEventListener('keydown', onKey) })
</script>

<template>
  <div class="min-h-screen bg-void text-slate-100">
    <div class="fixed inset-0 pointer-events-none opacity-50 [background-image:linear-gradient(rgba(86,168,255,.025)_1px,transparent_1px),linear-gradient(90deg,rgba(86,168,255,.025)_1px,transparent_1px)] [background-size:32px_32px]" />
    <AppSidebar />
    <div class="relative min-h-screen lg:pl-[252px]">
      <header class="sticky top-0 z-30 flex h-[70px] items-center gap-3 border-b border-line/80 bg-void/85 px-4 backdrop-blur-xl md:px-7">
        <button class="cursor-pointer rounded-lg p-2 text-muted transition-colors hover:bg-white/5 hover:text-white lg:hidden" aria-label="Abrir navegação" @click="ui.toggleSidebar"><Menu class="h-5 w-5" /></button>
        <div class="min-w-0 flex-1">
          <p class="font-mono text-[10px] uppercase tracking-[.2em] text-muted">Control plane / {{ route.name }}</p>
          <p class="mt-0.5 truncate text-sm font-medium text-slate-200">Infraestrutura unificada</p>
        </div>
        <button class="hidden h-9 min-w-56 cursor-pointer items-center gap-2 rounded-lg border border-line bg-panel/70 px-3 text-left text-xs text-muted transition-colors hover:border-slate-600 hover:text-slate-300 md:flex" @click="ui.commandOpen = true"><Search class="h-3.5 w-3.5" /><span class="flex-1">Comandos e recursos</span><kbd class="rounded border border-line px-1.5 py-0.5 font-mono text-[9px]">⌘ K</kbd></button>
        <div class="hidden border-l border-line pl-4 text-right xl:block"><p class="font-mono text-xs text-slate-300">{{ now.toLocaleTimeString('pt-BR') }}</p><p class="text-[10px] text-muted">America/Belem</p></div>
        <Button variant="ghost" class="relative px-2.5" aria-label="Notificações"><Bell class="h-4 w-4" /><span class="absolute right-2 top-1.5 h-1.5 w-1.5 rounded-full bg-warning" /></Button>
        <div class="flex h-9 w-9 items-center justify-center rounded-lg border border-signal/20 bg-signal/10 font-mono text-xs font-bold text-signal">WC</div>
      </header>
      <main class="relative p-4 md:p-7"><slot /></main>
    </div>

    <div v-if="ui.commandOpen" class="fixed inset-0 z-50 flex items-start justify-center bg-black/70 px-4 pt-[12vh] backdrop-blur-sm" @click.self="ui.commandOpen = false">
      <section class="w-full max-w-2xl overflow-hidden rounded-xl border border-slate-600/70 bg-[#0b111d] shadow-2xl" role="dialog" aria-modal="true" aria-label="Paleta de comandos">
        <div class="flex items-center gap-3 border-b border-line px-4"><Command class="h-4 w-4 text-signal" /><input autofocus class="h-14 flex-1 bg-transparent text-sm outline-none placeholder:text-muted" placeholder="Ir para módulo, host ou executar ação segura…" /><button class="cursor-pointer p-2 text-muted hover:text-white" aria-label="Fechar" @click="ui.commandOpen = false"><X class="h-4 w-4" /></button></div>
        <div class="p-3"><p class="px-2 py-2 font-mono text-[10px] uppercase tracking-[.18em] text-muted">Acesso rápido</p><RouterLink to="/telemetry" class="flex cursor-pointer items-center gap-3 rounded-lg px-3 py-3 text-sm text-slate-300 hover:bg-white/5" @click="ui.commandOpen = false"><ShieldCheck class="h-4 w-4 text-signal" />Abrir telemetria global<span class="ml-auto text-xs text-muted">T</span></RouterLink></div>
      </section>
    </div>
  </div>
</template>

