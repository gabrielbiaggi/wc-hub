<script setup lang="ts">
import { computed } from "vue";
import { useRoute } from "vue-router";
import {
  Activity,
  Archive,
  BatteryCharging,
  Bell,
  Boxes,
  Cloud,
  CloudCog,
  Container,
  FolderGit2,
  Gauge,
  GitBranch,
  HardDrive,
  Monitor,
  Network,
  PanelsTopLeft,
  RadioTower,
  Server,
  Settings,
  ShieldCheck,
  TerminalSquare,
  UserRoundCog,
  Waypoints,
  Workflow,
  X,
} from "lucide-vue-next";
import { useUiStore } from "@/stores/ui";
import { useQuery } from "@tanstack/vue-query";
import { getOverview } from "@/lib/api";
const route = useRoute();
const ui = useUiStore();
const overview = useQuery({ queryKey:["overview"], queryFn:getOverview, refetchInterval:30_000 });
const selfProtectionActive = computed(() => overview.data.value?.self_protected === true);
const groups = [
  {
    label: "Comando",
    items: [
      { to: "/", label: "Visão geral", icon: PanelsTopLeft },
      { to: "/inventory", label: "Inventário", icon: Boxes },
      { to: "/telemetry", label: "Telemetria", icon: Activity },
      { to: "/uptime", label: "Uptime e alertas", icon: Monitor },
    ],
  },
  {
    label: "Infraestrutura",
    items: [
      { to: "/proxmox", label: "Proxmox", icon: Server },
      { to: "/backups", label: "Backups e DR", icon: Archive },
      { to: "/power", label: "Energia e WOL", icon: BatteryCharging },
      { to: "/cloud", label: "Oracle / Nuvem", icon: Cloud },
      { to: "/kubernetes", label: "K3s / Kubernetes", icon: Waypoints },
      { to: "/docker", label: "Docker", icon: Container },
    ],
  },
  {
    label: "Entrega",
    items: [
      { to: "/github", label: "GitHub", icon: FolderGit2 },
      { to: "/cloudflare", label: "Cloudflare", icon: Network },
      { to: "/terraform", label: "Terraform", icon: GitBranch },
      { to: "/jobs", label: "Tarefas", icon: RadioTower },
      { to: "/operations", label: "Operações", icon: Workflow },
    ],
  },
  {
    label: "Acesso",
    items: [
      { to: "/remote-access", label: "Terminal SSH", icon: TerminalSquare },
      { to: "/vnc", label: "Console gráfico (noVNC)", icon: Monitor },
      { to: "/storage", label: "Armazenamento", icon: HardDrive },
    ],
  },
  {
    label: "Governança",
    items: [
      { to: "/admin", label: "Controle de acesso", icon: UserRoundCog },
      { to: "/integrations", label: "Integrações", icon: CloudCog },
      { to: "/notifications", label: "Notificações", icon: Bell },
      { to: "/audit", label: "Logs de auditoria", icon: ShieldCheck },
      { to: "/settings", label: "Configurações", icon: Settings },
    ],
  },
];
const isActive = (to: string) => computed(() => route.path === to).value;
</script>
<template>
  <div
    v-if="ui.sidebarOpen"
    class="fixed inset-0 z-40 bg-black/70 lg:hidden"
    @click="ui.sidebarOpen = false"
  />
  <aside
    :class="[
      'fixed inset-y-0 left-0 z-50 flex w-[252px] flex-col border-r border-line bg-[#070b13]/95 backdrop-blur-xl transition-transform duration-200 lg:translate-x-0',
      ui.sidebarOpen ? 'translate-x-0' : '-translate-x-full',
    ]"
  >
    <div class="flex h-[70px] items-center gap-3 border-b border-line px-5">
      <div
        class="grid h-9 w-9 place-items-center rounded-lg border border-signal/25 bg-signal/10 shadow-signal"
      >
        <Gauge class="h-5 w-5 text-signal" />
      </div>
      <div>
        <p class="text-sm font-bold tracking-[.08em]">WC HUB</p>
        <p class="font-mono text-[9px] uppercase tracking-[.18em] text-muted">
          Operações soberanas
        </p>
      </div>
      <button
        class="ml-auto cursor-pointer p-2 text-muted hover:text-white lg:hidden"
        aria-label="Fechar navegação"
        @click="ui.sidebarOpen = false"
      >
        <X class="h-4 w-4" />
      </button>
    </div>
    <nav
      class="scrollbar-thin flex-1 overflow-y-auto px-3 py-4"
      aria-label="Navegação principal"
    >
      <section v-for="group in groups" :key="group.label" class="mb-5">
        <h2
          class="mb-1.5 px-3 font-mono text-[9px] uppercase tracking-[.2em] text-slate-600"
        >
          {{ group.label }}
        </h2>
        <RouterLink
          v-for="item in group.items"
          :key="item.to"
          :to="item.to"
          :class="[
            'group mb-0.5 flex cursor-pointer items-center gap-3 rounded-lg border px-3 py-2.5 text-[13px] transition-colors duration-200',
            isActive(item.to)
              ? 'border-signal/15 bg-signal/[.07] text-signal'
              : 'border-transparent text-muted hover:bg-white/[.035] hover:text-slate-200',
          ]"
          @click="ui.sidebarOpen = false"
          ><component :is="item.icon" class="h-4 w-4" /><span>{{
            item.label
          }}</span
          ><span
            v-if="isActive(item.to)"
            class="ml-auto h-1 w-1 rounded-full bg-signal shadow-[0_0_8px_#49e29d]"
        /></RouterLink>
      </section>
    </nav>
    <div class="m-3 rounded-xl border border-line bg-panel/60 p-3">
      <div class="flex items-center gap-2">
        <RadioTower class="h-3.5 w-3.5 text-signal" /><span
          class="font-mono text-[10px] uppercase tracking-wider text-slate-300"
          >Plano de controle</span
        ><span
          class="ml-auto h-1.5 w-1.5 animate-pulse rounded-full bg-signal"
        />
      </div>
      <div class="mt-3 flex items-center gap-2 text-[11px] text-muted">
        <Boxes class="h-3.5 w-3.5" /><span>Autoprotegido</span
        ><span :class="['ml-auto font-mono', selfProtectionActive ? 'text-signal' : 'text-warning']">{{ selfProtectionActive ? 'ATIVO' : overview.isLoading.value ? 'VERIFICANDO' : 'DEGRADADO' }}</span>
      </div>
    </div>
  </aside>
</template>
