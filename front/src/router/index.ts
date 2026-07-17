import { createRouter, createWebHistory } from 'vue-router'

const modules = ['proxmox','cloud','kubernetes','docker','github','tunnels','terraform','telemetry','remote-access','storage']
export default createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/', name: 'overview', component: () => import('@/views/OverviewView.vue') },
    { path: '/inventory', name: 'inventory', component: () => import('@/views/InventoryView.vue') },
    { path: '/audit', name: 'audit', component: () => import('@/views/AuditView.vue') },
    { path: '/settings', name: 'settings', component: () => import('@/views/SettingsView.vue') },
    ...modules.map((name) => ({ path: `/${name}`, name, component: () => import('@/views/ModuleView.vue'), props: { module: name } })),
  ],
})
