<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, ref } from 'vue'
import { useQuery } from '@tanstack/vue-query'
import { Terminal as XTerminal } from '@xterm/xterm'
import { FitAddon } from '@xterm/addon-fit'
import { Splitpanes, Pane } from 'splitpanes'
import { LockKeyhole, Monitor, PlugZap, ShieldCheck, TerminalSquare } from 'lucide-vue-next'
import Button from '@/components/ui/Button.vue'
import StatusBadge from '@/components/ui/StatusBadge.vue'
import { createTerminalTicket, getHosts } from '@/lib/api'
import '@xterm/xterm/css/xterm.css'
import 'splitpanes/dist/splitpanes.css'

const query = useQuery({ queryKey: ['hosts'], queryFn: getHosts })
const hostID = ref('')
const confirmation = ref('')
const code = ref('')
const error = ref('')
const terminalRoot = ref<HTMLElement>()
const connected = ref(false)
let terminal: XTerminal | undefined
let socket: WebSocket | undefined
let fit: FitAddon | undefined

const targets = computed(() => query.data.value?.filter((host) => host.scope !== 'local' && !host.self_protected) ?? [])
const selected = computed(() => targets.value.find((host) => host.id === hostID.value))
const resize = () => fit?.fit()
const disconnect = () => {
  socket?.close()
  terminal?.dispose()
  terminal = undefined
  connected.value = false
  window.removeEventListener('resize', resize)
}
const connect = async () => {
  error.value = ''
  try {
    const issued = await createTerminalTicket(hostID.value, confirmation.value, code.value)
    await nextTick()
    terminal = new XTerminal({ cursorBlink: true, convertEol: true, fontFamily: 'JetBrains Mono, monospace', fontSize: 13, theme: { background: '#070b13', foreground: '#d8e0eb', cursor: '#49e29d', selectionBackground: '#49e29d33', black: '#05080f', green: '#49e29d', blue: '#56a8ff', red: '#ff6577' } })
    fit = new FitAddon()
    terminal.loadAddon(fit)
    terminal.open(terminalRoot.value!)
    fit.fit()
    const protocol = location.protocol === 'https:' ? 'wss' : 'ws'
    // The one-use ticket travels in a WebSocket subprotocol, keeping it out of URLs and access logs.
    socket = new WebSocket(`${protocol}://${location.host}/ws/terminal`, ['wc-hub-terminal', issued.ticket])
    socket.onopen = () => { connected.value = true; terminal?.focus(); socket?.send(JSON.stringify({ type: 'resize', cols: terminal?.cols, rows: terminal?.rows })) }
    socket.onmessage = (event) => { const message = JSON.parse(event.data); if (message.type === 'output') terminal?.write(message.data); if (message.type === 'error') terminal?.writeln(`\r\n\x1b[31m${message.data}\x1b[0m`) }
    socket.onclose = () => { connected.value = false; terminal?.writeln('\r\n\x1b[90m[sessão encerrada]\x1b[0m') }
    terminal.onData((data) => socket?.readyState === WebSocket.OPEN && socket.send(JSON.stringify({ type: 'input', data })))
    terminal.onResize((size) => socket?.readyState === WebSocket.OPEN && socket.send(JSON.stringify({ type: 'resize', ...size })))
    window.addEventListener('resize', resize)
  } catch (cause: any) {
    error.value = cause?.response?.data?.error?.message || 'Não foi possível emitir o ticket SSH.'
  }
}
onBeforeUnmount(disconnect)
</script>

<template>
  <div class="mx-auto max-w-[1680px] space-y-5">
    <header class="flex items-end justify-between"><div><div class="flex items-center gap-2 font-mono text-[10px] uppercase tracking-widest text-signal"><ShieldCheck class="h-3.5 w-3.5"/>ticket de uso único · known_hosts aplicado</div><h1 class="mt-3 text-3xl font-semibold tracking-tight">Terminal remoto</h1><p class="mt-2 text-sm text-muted">SSH intermediado pelo backend, sem chave privada no navegador.</p></div><StatusBadge :status="connected?'healthy':'warning'" :label="connected?'conectado':'desconectado'"/></header>
    <section class="h-[calc(100vh-210px)] min-h-[560px] overflow-hidden rounded-xl border border-line bg-[#070b13]"><Splitpanes class="default-theme"><Pane :size="24" :min-size="18" :max-size="35"><aside class="h-full border-r border-line bg-panel/60 p-4"><div class="flex items-center gap-2"><Monitor class="h-4 w-4 text-pulse"/><h2 class="text-sm font-medium">Conexão</h2></div><form class="mt-5 space-y-4" @submit.prevent="connect"><label class="block text-xs text-muted">Alvo remoto<select v-model="hostID" required class="field"><option value="" disabled>Selecione um host</option><option v-for="host in targets" :key="host.id" :value="host.id">{{host.name}}</option></select></label><label class="block text-xs text-muted">Digite o nome exato<input v-model="confirmation" required class="field" :placeholder="selected?.name||'nome-do-alvo'" autocomplete="off"/></label><label class="block text-xs text-muted">Código TOTP<input v-model="code" required class="field font-mono tracking-[.25em]" inputmode="numeric" pattern="[0-9]{6}" maxlength="6" placeholder="000000" autocomplete="one-time-code"/></label><p v-if="error" class="rounded-lg border border-danger/20 bg-danger/5 p-3 text-[11px] leading-5 text-danger">{{error}}</p><Button v-if="!connected" class="w-full" type="submit"><PlugZap class="h-4 w-4"/>Emitir ticket e conectar</Button><Button v-else class="w-full" variant="danger" type="button" @click="disconnect">Encerrar sessão</Button></form><div class="mt-6 border-t border-line pt-4"><div class="flex items-start gap-2 text-[11px] leading-5 text-muted"><LockKeyhole class="mt-0.5 h-3.5 w-3.5 shrink-0 text-signal"/>O próprio sistema e hosts locais não aparecem nesta lista.</div></div></aside></Pane><Pane :size="76"><div class="relative flex h-full flex-col"><div class="flex h-10 items-center border-b border-line px-4 font-mono text-[10px] text-muted"><TerminalSquare class="mr-2 h-3.5 w-3.5"/>{{selected?.name||'sem-sessão'}}<span class="ml-auto">xterm-256color</span></div><div ref="terminalRoot" class="min-h-0 flex-1 p-2"/><div v-if="!connected" class="pointer-events-none absolute inset-0 grid place-items-center"><div class="text-center"><TerminalSquare class="mx-auto h-8 w-8 text-slate-700"/><p class="mt-3 font-mono text-xs text-muted">Aguardando ticket seguro</p></div></div></div></Pane></Splitpanes></section>
  </div>
</template>
