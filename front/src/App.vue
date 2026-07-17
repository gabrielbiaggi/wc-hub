<script setup lang="ts">
import { onMounted } from 'vue'
import { RouterView } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import AppShell from '@/components/layout/AppShell.vue'
import LoginView from '@/views/LoginView.vue'
const auth = useAuthStore()
onMounted(auth.initialize)
</script>

<template>
  <div v-if="auth.loading && !auth.authenticated" class="grid min-h-screen place-items-center bg-void"><div class="h-8 w-8 animate-spin rounded-full border-2 border-line border-t-signal" /></div>
  <LoginView v-else-if="!auth.authenticated" />
  <AppShell v-else><RouterView /></AppShell>
</template>

