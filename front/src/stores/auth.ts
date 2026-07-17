import { defineStore } from 'pinia'
import { computed, ref } from 'vue'
import { api, setCSRFToken, type AuthResponse, type User } from '@/lib/api'

export const useAuthStore = defineStore('auth', () => {
  const user = ref<User | null>(null)
  const loading = ref(true)
  const bootstrapRequired = ref(false)
  const error = ref('')
  const authenticated = computed(() => !!user.value)

  const accept = (response: AuthResponse) => { user.value = response.user; setCSRFToken(response.csrf_token); error.value = '' }
  const initialize = async () => {
    loading.value = true
    try {
      const status = await api.get<{bootstrap_required:boolean}>('/v1/auth/bootstrap-status')
      bootstrapRequired.value = status.data.bootstrap_required
      if (!bootstrapRequired.value) accept((await api.get<AuthResponse>('/v1/auth/session')).data)
    } catch { user.value = null }
    finally { loading.value = false }
  }
  const login = async (email:string,password:string) => { loading.value=true; try{accept((await api.post<AuthResponse>('/v1/auth/login',{email,password})).data)}catch{error.value='Credenciais inválidas ou acesso bloqueado.';throw new Error(error.value)}finally{loading.value=false} }
  const bootstrap = async (display_name:string,email:string,password:string) => { loading.value=true; try{accept((await api.post<AuthResponse>('/v1/auth/bootstrap',{display_name,email,password})).data);bootstrapRequired.value=false}catch(error:any){const message=error?.response?.data?.error?.message;error.value=message||'Não foi possível criar o administrador.';throw new Error(error.value)}finally{loading.value=false} }
  const logout = async () => { try{await api.post('/v1/auth/logout')}finally{user.value=null;setCSRFToken('')} }
  return { user, loading, bootstrapRequired, error, authenticated, initialize, login, bootstrap, logout }
})

