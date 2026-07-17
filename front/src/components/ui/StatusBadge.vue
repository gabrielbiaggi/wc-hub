<script setup lang="ts">
import { computed } from 'vue'
const props = defineProps<{ status: 'healthy' | 'warning' | 'critical' | 'info' | string; label?: string }>()
const color = computed(() => ({ healthy: 'bg-signal', success: 'bg-signal', warning: 'bg-warning', critical: 'bg-danger', info: 'bg-pulse' }[props.status] ?? 'bg-muted'))
</script>
<template><span class="inline-flex items-center gap-2 rounded-full border border-line bg-white/[.025] px-2.5 py-1 font-mono text-[10px] uppercase tracking-[.13em] text-slate-300"><span :class="['h-1.5 w-1.5 rounded-full', color]" />{{ label ?? status }}</span></template>

