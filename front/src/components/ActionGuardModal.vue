<template>
  <div v-if="show" class="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm p-4">
    <div class="bg-gray-900 border border-red-500/30 rounded-xl shadow-2xl max-w-md w-full p-6 text-gray-100 animate-in fade-in zoom-in duration-200">
      <div class="flex items-center space-x-3 text-red-400 mb-4">
        <svg class="w-7 h-7" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
        </svg>
        <h3 class="text-xl font-bold tracking-wide text-white">{{ title || 'Ação Crítica Protegida' }}</h3>
      </div>

      <p class="text-sm text-gray-300 mb-4">
        {{ actionDescription || 'Esta é uma operação destrutiva e requer confirmação forte.' }}
      </p>

      <div class="bg-red-950/40 border border-red-800/40 rounded-lg p-3 text-xs text-red-300 mb-5">
        Para confirmar, digite exatamente o nome do alvo: <span class="font-mono font-bold text-white select-all">{{ targetName }}</span>
      </div>

      <div class="space-y-4 mb-6">
        <div>
          <label class="block text-xs font-medium text-gray-400 mb-1">Confirmação do Alvo</label>
          <input
            v-model="confirmation"
            type="text"
            :placeholder="targetName"
            class="w-full px-3 py-2 bg-gray-800 border border-gray-700 rounded-lg text-white font-mono text-sm focus:outline-none focus:border-red-500 transition-colors"
          />
        </div>

        <div>
          <label class="block text-xs font-medium text-gray-400 mb-1">Código TOTP (2FA)</label>
          <input
            v-model="totpCode"
            type="text"
            maxlength="6"
            placeholder="000000"
            class="w-full px-3 py-2 bg-gray-800 border border-gray-700 rounded-lg text-white font-mono text-sm tracking-widest focus:outline-none focus:border-red-500 transition-colors"
          />
        </div>
      </div>

      <div class="flex justify-end space-x-3">
        <button
          @click="onCancel"
          class="px-4 py-2 bg-gray-800 hover:bg-gray-700 text-gray-300 text-sm font-medium rounded-lg transition-colors"
        >
          Cancelar
        </button>
        <button
          @click="onConfirm"
          :disabled="!isValid"
          class="px-4 py-2 bg-red-600 hover:bg-red-500 disabled:opacity-40 disabled:hover:bg-red-600 text-white text-sm font-medium rounded-lg shadow-lg transition-all"
        >
          Confirmar Operação
        </button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue'

const props = defineProps<{
  show: boolean
  title?: string
  targetName: string
  actionDescription?: string
}>()

const emit = defineEmits<{
  (e: 'cancel'): void
  (e: 'confirm', payload: { confirmation: string; totpCode: string }): void
}>()

const confirmation = ref('')
const totpCode = ref('')

const isValid = computed(() => {
  return confirmation.value.trim() === props.targetName.trim() && totpCode.value.trim().length >= 6
})

function onCancel() {
  confirmation.value = ''
  totpCode.value = ''
  emit('cancel')
}

function onConfirm() {
  if (isValid.value) {
    emit('confirm', {
      confirmation: confirmation.value.trim(),
      totpCode: totpCode.value.trim(),
    })
    confirmation.value = ''
    totpCode.value = ''
  }
}
</script>
