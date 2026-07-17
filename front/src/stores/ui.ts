import { defineStore } from 'pinia'
import { ref } from 'vue'
export const useUiStore = defineStore('ui', () => {
  const sidebarOpen = ref(false)
  const commandOpen = ref(false)
  const toggleSidebar = () => { sidebarOpen.value = !sidebarOpen.value }
  return { sidebarOpen, commandOpen, toggleSidebar }
})

