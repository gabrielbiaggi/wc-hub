<script setup lang="ts">
import { cn } from '@/lib/utils'
defineProps<{ variant?: 'default' | 'outline' | 'ghost' | 'danger'; class?: string; type?: 'button' | 'submit' }>()
</script>
<template><button :type="type ?? 'button'" :class="cn('inline-flex h-9 cursor-pointer items-center justify-center gap-2 rounded-lg px-3 text-sm font-medium transition-colors duration-200 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-signal disabled:pointer-events-none disabled:opacity-50', variant === 'outline' && 'border border-line bg-panel hover:border-slate-500 hover:bg-slate-900', variant === 'ghost' && 'hover:bg-white/[.06]', variant === 'danger' && 'bg-danger/10 text-danger hover:bg-danger/20', (!variant || variant === 'default') && 'bg-signal text-slate-950 hover:bg-emerald-300', $props.class)"><slot /></button></template>

