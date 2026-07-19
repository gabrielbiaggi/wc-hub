<script setup lang="ts">
import { ref } from 'vue'
import { Activity, ArrowRight, Fingerprint, Gauge, LockKeyhole, ShieldCheck } from 'lucide-vue-next'
import Button from '@/components/ui/Button.vue'
import { useAuthStore } from '@/stores/auth'

const auth = useAuthStore()
const email = ref('')
const password = ref('')
const displayName = ref('')
const showError = ref('')

const submit = async () => {
  showError.value = ''
  try {
    if (auth.bootstrapRequired) await auth.bootstrap(displayName.value, email.value, password.value)
    else await auth.login(email.value, password.value)
  } catch (error: any) {
    showError.value = error.message
  }
}
</script>

<template>
  <main class="relative grid min-h-screen overflow-hidden bg-void lg:grid-cols-[1.15fr_.85fr]">
    <div class="pointer-events-none absolute inset-0 opacity-60 [background-image:linear-gradient(rgba(86,168,255,.035)_1px,transparent_1px),linear-gradient(90deg,rgba(86,168,255,.035)_1px,transparent_1px)] [background-size:36px_36px]" />
    <section class="relative hidden flex-col justify-between border-r border-line p-12 lg:flex">
      <div class="flex items-center gap-3"><div class="grid h-10 w-10 place-items-center rounded-xl border border-signal/25 bg-signal/10 shadow-signal"><Gauge class="h-5 w-5 text-signal" /></div><div><p class="font-bold tracking-[.1em]">WC HUB</p><p class="font-mono text-[9px] uppercase tracking-[.2em] text-muted">Operações soberanas</p></div></div>
      <div class="max-w-xl"><div class="mb-6 flex items-center gap-2 font-mono text-[10px] uppercase tracking-[.18em] text-signal"><span class="h-1.5 w-1.5 animate-pulse rounded-full bg-signal" />Plano de controle protegido</div><h1 class="text-5xl font-semibold leading-[1.08] tracking-[-.045em] text-white">Infraestrutura é poder.<br><span class="text-slate-500">Controle com propósito.</span></h1><p class="mt-6 max-w-lg text-sm leading-7 text-muted">Um cockpit operacional centralizado, construído para observar tudo e impedir que uma ação errada destrua o próprio plano de controle.</p></div>
      <div class="grid grid-cols-3 gap-3"><div v-for="item in [{ icon: ShieldCheck, label: 'Autoprotegido' }, { icon: Activity, label: 'Totalmente auditado' }, { icon: Fingerprint, label: 'Identidade forte' }]" :key="item.label" class="rounded-xl border border-line bg-panel/50 p-4"><component :is="item.icon" class="h-4 w-4 text-signal" /><p class="mt-3 font-mono text-[9px] uppercase tracking-widest text-muted">{{ item.label }}</p></div></div>
    </section>

    <section class="relative flex items-center justify-center p-5 md:p-10">
      <form class="w-full max-w-md rounded-2xl border border-line bg-panel/75 p-6 shadow-2xl backdrop-blur-xl md:p-8" @submit.prevent="submit">
        <div class="grid h-11 w-11 place-items-center rounded-xl border border-pulse/20 bg-pulse/[.07] lg:hidden"><Gauge class="h-5 w-5 text-pulse" /></div>
        <div class="mt-6 lg:mt-0"><p class="font-mono text-[10px] uppercase tracking-[.2em] text-signal">{{ auth.bootstrapRequired ? 'Inicialização segura' : 'Acesso do operador' }}</p><h2 class="mt-3 text-2xl font-semibold tracking-tight">{{ auth.bootstrapRequired ? 'Criar administrador' : 'Entrar no plano de controle' }}</h2><p class="mt-2 text-sm leading-6 text-muted">{{ auth.bootstrapRequired ? 'Esta operação fecha permanentemente a inicialização pública.' : 'Use sua identidade administrativa para continuar.' }}</p></div>
        <div class="mt-7 space-y-4">
          <label v-if="auth.bootstrapRequired" class="block text-xs text-slate-300">Nome do operador<input v-model="displayName" required autocomplete="name" class="mt-2 h-11 w-full rounded-lg border border-line bg-slate-950/60 px-3 text-sm outline-none transition-colors focus:border-signal/60 focus:ring-2 focus:ring-signal/10" placeholder="Gabriel"></label>
          <label class="block text-xs text-slate-300">{{ auth.bootstrapRequired ? 'E-mail' : 'E-mail ou usuário' }}<input v-model="email" required :type="auth.bootstrapRequired ? 'email' : 'text'" :autocomplete="auth.bootstrapRequired ? 'email' : 'username'" class="mt-2 h-11 w-full rounded-lg border border-line bg-slate-950/60 px-3 text-sm outline-none transition-colors focus:border-signal/60 focus:ring-2 focus:ring-signal/10" :placeholder="auth.bootstrapRequired ? 'operador@webcreations.com.br' : 'allmight ou e-mail'"></label>
          <label class="block text-xs text-slate-300">Senha<input v-model="password" required type="password" :minlength="auth.bootstrapRequired ? 14 : 1" autocomplete="current-password" class="mt-2 h-11 w-full rounded-lg border border-line bg-slate-950/60 px-3 text-sm outline-none transition-colors focus:border-signal/60 focus:ring-2 focus:ring-signal/10" placeholder="••••••••••••••"><span v-if="auth.bootstrapRequired" class="mt-1.5 block text-[10px] text-muted">Mínimo de 14 caracteres. O TOTP será configurado em Configurações.</span></label>
        </div>
        <p v-if="showError" class="mt-4 rounded-lg border border-danger/20 bg-danger/5 px-3 py-2 text-xs text-danger">{{ showError }}</p>
        <Button class="mt-6 h-11 w-full" type="submit" :disabled="auth.loading"><LockKeyhole class="h-4 w-4" />{{ auth.bootstrapRequired ? 'Selar inicialização' : 'Autenticar' }}<ArrowRight class="ml-auto h-4 w-4" /></Button>
        <p class="mt-5 flex items-center justify-center gap-2 font-mono text-[9px] uppercase tracking-wider text-muted"><ShieldCheck class="h-3 w-3 text-signal" />HttpOnly · SameSite estrito · CSRF protegido</p>
      </form>
    </section>
  </main>
</template>
