import { createRouter, createWebHistory } from 'vue-router'

const modules = ['cloud','kubernetes','docker','github','tunnels','terraform','storage']
export default createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/', name: 'overview', component: () => import('@/views/OverviewView.vue') },
    { path: '/inventory', name: 'inventory', component: () => import('@/views/InventoryView.vue') },
    { path: '/audit', name: 'audit', component: () => import('@/views/AuditView.vue') },
    { path: '/admin', name: 'admin', component: () => import('@/views/AdminView.vue') },
    { path: '/integrations', name: 'integrations', component: () => import('@/views/IntegrationsView.vue') },
    { path: '/notifications', name: 'notifications', component: () => import('@/views/NotificationsView.vue') },
    { path: '/settings', name: 'settings', component: () => import('@/views/SettingsView.vue') },
    { path: '/proxmox', name: 'proxmox', component: () => import('@/views/ProxmoxView.vue') },
    { path: '/telemetry', name: 'telemetry', component: () => import('@/views/TelemetryView.vue') },
    { path: '/jobs', name: 'jobs', component: () => import('@/views/JobsView.vue') },
    { path: '/remote-access', name: 'remote-access', component: () => import('@/views/TerminalView.vue') },
    ...modules.map((name) => ({ path: `/${name}`, name, component: () => import('@/views/ModuleView.vue'), props: { module: name } })),
  ],
})
