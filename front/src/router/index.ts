import { createRouter, createWebHistory } from 'vue-router'

const modules = ['proxmox','cloud','kubernetes','docker','github','tunnels','terraform','telemetry','remote-access','storage','audit','settings']
export default createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/', name: 'overview', component: () => import('@/views/OverviewView.vue') },
    ...modules.map((name) => ({ path: `/${name}`, name, component: () => import('@/views/ModuleView.vue'), props: { module: name } })),
  ],
})
