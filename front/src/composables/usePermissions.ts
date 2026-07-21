import { computed } from 'vue'
import { useAuthStore } from '@/stores/auth'

export function usePermissions() {
  const authStore = useAuthStore()

  const permissions = computed<string[]>(() => {
    return authStore.user?.permissions ?? []
  })

  const roles = computed<string[]>(() => {
    return authStore.user?.roles ?? []
  })

  function hasPermission(permission: string): boolean {
    if (!permission) return true
    if (roles.value.includes('admin') || roles.value.includes('superadmin')) return true
    return permissions.value.includes(permission)
  }

  function hasAnyPermission(permissionList: string[]): boolean {
    if (!permissionList || permissionList.length === 0) return true
    if (roles.value.includes('admin') || roles.value.includes('superadmin')) return true
    return permissionList.some((p) => permissions.value.includes(p))
  }

  const isReadonly = computed(() => {
    if (roles.value.includes('admin') || roles.value.includes('superadmin')) return false
    return !permissions.value.some((p) => p.endsWith('.manage') || p.endsWith('.write'))
  })

  return {
    permissions,
    roles,
    hasPermission,
    hasAnyPermission,
    isReadonly,
  }
}
