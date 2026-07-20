<script setup lang="ts">
import { computed, ref } from "vue";
import { useMutation, useQuery, useQueryClient } from "@tanstack/vue-query";
import {
  Boxes,
  Camera,
  Copy,
  Database,
  MemoryStick,
  Play,
  Plus,
  Power,
  RefreshCw,
  RotateCcw,
  Server,
  ShieldCheck,
  Square,
  Trash2,
} from "lucide-vue-next";
import Button from "@/components/ui/Button.vue";
import StatusBadge from "@/components/ui/StatusBadge.vue";
import {
  cloneProxmoxGuest,
  createProxmoxLXC,
  createProxmoxQEMU,
  createProxmoxSnapshot,
  deleteProxmoxGuest,
  deleteProxmoxSnapshot,
  getJobs,
  getProxmoxInventory,
  getProxmoxSnapshots,
  getProxmoxSummary,
  runProxmoxPowerAction,
  syncProxmox,
  type ProxmoxGuest,
  type ProxmoxLXCInput,
  type ProxmoxQEMUInput,
  type ProxmoxSnapshot,
} from "@/lib/api";
import { apiErrorMessage } from "@/lib/api_error";

type Tab = "qemu" | "lxc" | "storage" | "provision";
const tab = ref<Tab>("qemu");
const client = useQueryClient();
const summary = useQuery({
  queryKey: ["proxmox-summary"],
  queryFn: getProxmoxSummary,
  refetchInterval: 15000,
});
const inventory = useQuery({
  queryKey: ["proxmox-inventory"],
  queryFn: getProxmoxInventory,
  refetchInterval: 20000,
});
const jobs = useQuery({
  queryKey: ["jobs"],
  queryFn: getJobs,
  refetchInterval: 3000,
});
const sync = useMutation({
  mutationFn: syncProxmox,
  onSuccess: () => {
    client.invalidateQueries({ queryKey: ["jobs"] });
    setTimeout(() => {
      client.invalidateQueries({ queryKey: ["proxmox-summary"] });
      client.invalidateQueries({ queryKey: ["proxmox-inventory"] });
    }, 2500);
  },
});
const power = useMutation({
  mutationFn: (input: {
    guest: ProxmoxGuest;
    action: "start" | "stop" | "shutdown" | "reboot" | "reset";
  }) =>
    runProxmoxPowerAction(
      input.guest.cluster,
      input.guest.node,
      input.guest.type,
      input.guest.vmid,
      input.action,
    ),
  onSuccess: () =>
    setTimeout(
      () => client.invalidateQueries({ queryKey: ["proxmox-inventory"] }),
      1200,
    ),
});
const qemuForm = ref<ProxmoxQEMUInput>({
  cluster: "",
  node: "",
  vmid: 100,
  name: "",
  cores: 2,
  memory_mb: 2048,
  storage: "local-lvm",
  disk_gb: 32,
  iso: "",
  bridge: "vmbr0",
  start: false,
});
const lxcForm = ref<ProxmoxLXCInput>({
  cluster: "",
  node: "",
  vmid: 200,
  hostname: "",
  cores: 2,
  memory_mb: 1024,
  storage: "local-lvm",
  rootfs_gb: 16,
  template: "",
  bridge: "vmbr0",
  password: "",
  ssh_public_keys: "",
  unprivileged: true,
  start: false,
});
const provision = useMutation({
  mutationFn: (input: { kind: "qemu" | "lxc" }) =>
    input.kind === "qemu"
      ? createProxmoxQEMU(qemuForm.value)
      : createProxmoxLXC(lxcForm.value),
  onSuccess: () =>
    setTimeout(
      () => client.invalidateQueries({ queryKey: ["proxmox-inventory"] }),
      1600,
    ),
});
const destructive = useMutation({
  mutationFn: (guest: ProxmoxGuest) => deleteProxmoxGuest(guest, true),
  onSuccess: () =>
    setTimeout(
      () => client.invalidateQueries({ queryKey: ["proxmox-inventory"] }),
      1600,
    ),
});
const cloning = useMutation({
  mutationFn: (input: { guest: ProxmoxGuest; vmid: number; name: string }) =>
    cloneProxmoxGuest({
      cluster: input.guest.cluster,
      node: input.guest.node,
      kind: input.guest.type,
      source_vmid: input.guest.vmid,
      new_vmid: input.vmid,
      name: input.name,
      target_node: "",
      storage: "",
      full: true,
    }),
  onSuccess: () =>
    setTimeout(
      () => client.invalidateQueries({ queryKey: ["proxmox-inventory"] }),
      1600,
    ),
});
const snapshotGuest = ref<ProxmoxGuest | null>(null);
const snapshots = useQuery({
  queryKey: computed(() => [
    "proxmox-snapshots",
    snapshotGuest.value?.cluster,
    snapshotGuest.value?.node,
    snapshotGuest.value?.type,
    snapshotGuest.value?.vmid,
  ]),
  queryFn: () => {
    const g = snapshotGuest.value;
    if (!g) return Promise.resolve([]);
    return getProxmoxSnapshots(g.cluster, g.node, g.type, g.vmid);
  },
  enabled: computed(() => !!snapshotGuest.value),
  refetchInterval: false,
});
const snapshotForm = ref({ name: "", description: "" });
const createSnapshot = useMutation({
  mutationFn: () => {
    const g = snapshotGuest.value;
    if (!g) throw new Error("No guest selected");
    return createProxmoxSnapshot(
      g.cluster,
      g.node,
      g.type,
      g.vmid,
      snapshotForm.value.name,
      snapshotForm.value.description,
    );
  },
  onSuccess: () => {
    snapshotForm.value = { name: "", description: "" };
    setTimeout(
      () => client.invalidateQueries({ queryKey: ["proxmox-snapshots"] }),
      1000,
    );
  },
});
const removeSnapshot = useMutation({
  mutationFn: (name: string) => {
    const g = snapshotGuest.value;
    if (!g) throw new Error("No guest selected");
    return deleteProxmoxSnapshot(g.cluster, g.node, g.type, g.vmid, name);
  },
  onSuccess: () =>
    setTimeout(
      () => client.invalidateQueries({ queryKey: ["proxmox-snapshots"] }),
      1000,
    ),
});
const lastJob = computed(() =>
  jobs.data.value?.find((job) => job.kind === "proxmox.sync"),
);
const providerError = computed(() =>
  apiErrorMessage(
    inventory.error.value ??
      power.error.value ??
      sync.error.value ??
      provision.error.value ??
      cloning.error.value ??
      destructive.error.value ??
      createSnapshot.error.value ??
      removeSnapshot.error.value,
    "A operação Proxmox falhou.",
  ),
);
const guests = computed(() =>
  tab.value === "qemu"
    ? (inventory.data.value?.virtual_machines ?? [])
    : (inventory.data.value?.containers ?? []),
);
const machines = computed(() => {
  const snapshot = inventory.data.value;
  if (!snapshot) return [];
  const nodes = snapshot.nodes ?? [];
  const virtualMachines = snapshot.virtual_machines ?? [];
  const containers = snapshot.containers ?? [];
  const storage = snapshot.storage ?? [];
  return [...new Set(nodes.map((node) => node.cluster))]
    .sort()
    .map((cluster) => {
      const clusterNodes = nodes.filter((node) => node.cluster === cluster);
      return {
        cluster,
        nodes: clusterNodes,
        online: clusterNodes.every((node) => node.status === "online"),
        qemu: virtualMachines.filter(
          (guest) => guest.cluster === cluster,
        ).length,
        lxc: containers.filter((guest) => guest.cluster === cluster).length,
        storage: storage.filter((store) => store.cluster === cluster).length,
      };
    });
});
const formatBytes = (value = 0) => {
  if (!value) return "0 B";
  const units = ["B", "KB", "MB", "GB", "TB"],
    index = Math.min(Math.floor(Math.log(value) / Math.log(1024)), 4);
  return `${(value / 1024 ** index).toFixed(index > 2 ? 1 : 0)} ${units[index]}`;
};
const uptime = (seconds = 0) =>
  `${Math.floor(seconds / 86400)}d ${Math.floor((seconds % 86400) / 3600)}h`;
const execute = (
  guest: ProxmoxGuest,
  action: "start" | "stop" | "shutdown" | "reboot" | "reset",
) => {
  if (
    window.confirm(
      `Confirma ${action} em ${guest.type.toUpperCase()} ${guest.vmid} (${guest.name})?`,
    )
  )
    power.mutate({ guest, action });
};
const createGuest = (kind: "qemu" | "lxc") => {
  const name = kind === "qemu" ? qemuForm.value.name : lxcForm.value.hostname;
  const phrase = `CRIAR ${kind.toUpperCase()} ${name}`;
  if (
    window.prompt(
      `Esta operação altera o bare metal. Digite exatamente: ${phrase}`,
    ) === phrase
  )
    provision.mutate({ kind });
};
const cloneGuest = (guest: ProxmoxGuest) => {
  const name = window.prompt("Nome do clone completo:");
  if (!name) return;
  const raw = window.prompt("Novo VMID:");
  const vmid = Number(raw);
  if (
    Number.isInteger(vmid) &&
    vmid > 0 &&
    window.confirm(`Clonar ${guest.vmid} como ${vmid} (${name})?`)
  )
    cloning.mutate({ guest, vmid, name });
};
const removeGuest = (guest: ProxmoxGuest) => {
  const phrase = `EXCLUIR ${guest.vmid}`;
  if (
    window.prompt(
      `Exclusão permanente com purge. Digite exatamente: ${phrase}`,
    ) === phrase
  )
    destructive.mutate(guest);
};
const openSnapshots = (guest: ProxmoxGuest) => {
  snapshotGuest.value = guest;
};
const closeSnapshots = () => {
  snapshotGuest.value = null;
  snapshotForm.value = { name: "", description: "" };
};
const submitSnapshot = () => {
  if (
    window.confirm(
      `Criar snapshot "${snapshotForm.value.name}" de ${snapshotGuest.value?.type.toUpperCase()} ${snapshotGuest.value?.vmid}?`,
    )
  )
    createSnapshot.mutate();
};
const deleteSnapshot = (name: string) => {
  const phrase = `EXCLUIR ${name}`;
  if (
    window.prompt(
      `Excluir snapshot permanentemente. Digite exatamente: ${phrase}`,
    ) === phrase
  )
    removeSnapshot.mutate(name);
};
</script>

<template>
  <div class="mx-auto max-w-[1680px] space-y-5">
    <header
      class="flex flex-col justify-between gap-4 md:flex-row md:items-end"
    >
      <div>
        <div class="flex flex-wrap gap-2">
          <StatusBadge
            :status="inventory.isError.value ? 'critical' : 'healthy'"
            :label="
              inventory.isError.value
                ? 'provedor indisponível'
                : 'administrador do cluster'
            "
          /><span
            class="inline-flex items-center gap-1.5 font-mono text-[10px] uppercase text-signal"
            ><ShieldCheck class="h-3.5 w-3.5" />token da API PVE · operações
            auditadas</span
          >
        </div>
        <h1 class="mt-4 text-3xl font-semibold">Malha Proxmox</h1>
        <p class="mt-2 text-sm text-muted">
          Nodes, QEMU, LXC, storage e controle de energia diretamente pela API
          PVE.
        </p>
      </div>
      <Button
        :disabled="!summary.data.value?.configured || sync.isPending.value"
        @click="sync.mutate()"
        ><RefreshCw
          :class="['h-4 w-4', sync.isPending.value && 'animate-spin']"
        />Sincronizar</Button
      >
    </header>
    <div
      v-if="
        inventory.isError.value ||
        power.isError.value ||
        sync.isError.value ||
        provision.isError.value ||
        cloning.isError.value ||
        destructive.isError.value
      "
      class="rounded-xl border border-danger/20 bg-danger/5 p-4 text-sm text-danger"
    >
      <p class="font-medium">A operação Proxmox falhou</p>
      <p class="mt-1 break-words font-mono text-xs">{{ providerError }}</p>
    </div>
    <div
      v-if="inventory.data.value?.warnings?.length"
      class="rounded-xl border border-warning/20 bg-warning/5 p-4 text-xs text-warning"
    >
      {{ inventory.data.value.warnings.join(" · ") }}
    </div>
    <section class="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
      <article
        v-for="item in [
          {
            label: 'Nós',
            value: inventory.data.value?.nodes?.length ?? 0,
            icon: Server,
          },
          {
            label: 'VMs QEMU',
            value: inventory.data.value?.virtual_machines?.length ?? 0,
            icon: Boxes,
          },
          {
            label: 'Containers LXC',
            value: inventory.data.value?.containers?.length ?? 0,
            icon: Database,
          },
          {
            label: 'Pools de armazenamento',
            value: inventory.data.value?.storage?.length ?? 0,
            icon: MemoryStick,
          },
        ]"
        :key="item.label"
        class="rounded-xl border border-line bg-panel/65 p-5"
      >
        <component :is="item.icon" class="h-4 w-4 text-muted" />
        <p class="mt-5 font-mono text-2xl text-white">{{ item.value }}</p>
        <p class="mt-1 text-xs text-muted">{{ item.label }}</p>
      </article>
    </section>
    <section class="grid gap-5 xl:grid-cols-[1fr_320px]">
      <div class="space-y-5">
        <article class="overflow-hidden rounded-xl border border-line bg-panel/65">
          <header class="border-b border-line p-4">
            <h2 class="text-sm font-medium">Conexões por máquina</h2>
            <p class="mt-1 text-[10px] text-muted">
              Cada painel representa um host/cluster Proxmox independente.
            </p>
          </header>
          <div class="divide-y divide-line/60">
            <section
              v-for="machine in machines"
              :key="machine.cluster"
              class="bg-panel p-4"
            >
              <header class="flex flex-wrap items-center justify-between gap-3">
                <div>
                  <p class="font-mono text-xs text-signal">{{ machine.cluster }}</p>
                  <p class="mt-1 text-[10px] text-muted">
                    {{ machine.nodes.length }} nó(s) · {{ machine.qemu }} QEMU ·
                    {{ machine.lxc }} LXC · {{ machine.storage }} storage(s)
                  </p>
                </div>
                <StatusBadge
                  :status="machine.online ? 'healthy' : 'critical'"
                  :label="machine.online ? 'conectado' : 'atenção necessária'"
                />
              </header>
              <div class="mt-4 grid gap-px bg-line/60 md:grid-cols-2">
                <div
                  v-for="node in machine.nodes"
                  :key="`${machine.cluster}-${node.node}`"
                  class="bg-slate-950/20 p-4"
                >
                  <div class="flex items-center justify-between">
                    <div>
                      <p class="text-sm text-slate-200">{{ node.node }}</p>
                      <p class="mt-1 font-mono text-[9px] text-muted">
                        {{ node.maxcpu }} CPU · {{ formatBytes(node.maxmem) }} ·
                        {{ uptime(node.uptime) }}
                      </p>
                    </div>
                    <StatusBadge
                      :status="node.status === 'online' ? 'healthy' : 'critical'"
                      :label="node.status"
                    />
                  </div>
                  <div class="mt-4 grid grid-cols-2 gap-3 text-[10px]">
                    <span class="text-muted"
                      >CPU
                      <b class="float-right text-slate-300"
                        >{{ (node.cpu * 100).toFixed(1) }}%</b
                      ></span
                    ><span class="text-muted"
                      >RAM
                      <b class="float-right text-slate-300"
                        >{{
                          node.maxmem
                            ? ((node.mem / node.maxmem) * 100).toFixed(1)
                            : 0
                        }}%</b
                      ></span
                    >
                  </div>
                </div>
              </div>
            </section>
            <p v-if="!machines.length" class="p-10 text-center text-sm text-muted">
              Nenhuma máquina Proxmox retornada.
            </p>
          </div>
        </article>
        <article
          class="overflow-hidden rounded-xl border border-line bg-panel/65"
        >
          <header
            class="flex items-center justify-between border-b border-line p-4"
          >
            <div>
              <h2 class="text-sm font-medium">Recursos</h2>
              <p class="mt-1 text-[10px] text-muted">
                Captura
                {{
                  inventory.data.value
                    ? new Date(inventory.data.value.captured_at).toLocaleString(
                        "pt-BR",
                      )
                    : "—"
                }}
              </p>
            </div>
            <nav class="flex flex-wrap rounded-lg border border-line p-1">
              <button
                v-for="item in [
                  { id: 'qemu', label: 'QEMU' },
                  { id: 'lxc', label: 'LXC' },
                  { id: 'storage', label: 'Armazenamento' },
                  { id: 'provision', label: 'Criar' },
                ]"
                :key="item.id"
                :class="[
                  'rounded px-3 py-1.5 text-xs',
                  tab === item.id ? 'bg-signal/10 text-signal' : 'text-muted',
                ]"
                @click="tab = item.id as Tab"
              >
                {{ item.label }}
              </button>
            </nav>
          </header>
          <div
            v-if="tab === 'qemu' || tab === 'lxc'"
            class="divide-y divide-line/60"
          >
            <div
              v-for="guest in guests"
              :key="`${guest.cluster}-${guest.type}-${guest.vmid}`"
              class="grid gap-4 p-4 lg:grid-cols-[1fr_200px_420px] lg:items-center"
            >
              <div>
                <p class="text-sm text-slate-200">
                  {{ guest.vmid }} · {{ guest.name || "sem nome" }}
                </p>
                <p class="mt-1 font-mono text-[9px] text-muted">
                  {{ guest.cluster }} / {{ guest.node }} · {{ guest.cpus }} CPU
                  · {{ formatBytes(guest.maxmem) }} RAM ·
                  {{ formatBytes(guest.maxdisk) }} disco ·
                  {{ uptime(guest.uptime) }}
                </p>
              </div>
              <div>
                <StatusBadge
                  :status="guest.status === 'running' ? 'healthy' : 'warning'"
                  :label="guest.status"
                />
                <p class="mt-2 font-mono text-[9px] text-muted">
                  CPU {{ (guest.cpu * 100).toFixed(1) }}% · MEM
                  {{
                    guest.maxmem
                      ? ((guest.mem / guest.maxmem) * 100).toFixed(1)
                      : 0
                  }}%
                </p>
              </div>
              <div class="flex flex-wrap justify-end gap-2">
                <Button variant="outline" @click="openSnapshots(guest)"
                  ><Camera class="h-3.5 w-3.5" />Snapshots</Button
                ><Button variant="outline" @click="cloneGuest(guest)"
                  ><Copy class="h-3.5 w-3.5" />Clonar</Button
                ><Button
                  v-if="guest.status !== 'running'"
                  variant="outline"
                  :disabled="power.isPending.value"
                  @click="execute(guest, 'start')"
                  ><Play class="h-3.5 w-3.5" />Iniciar</Button
                ><template v-else
                  ><Button
                    variant="outline"
                    :disabled="power.isPending.value"
                    @click="execute(guest, 'reboot')"
                    ><RotateCcw class="h-3.5 w-3.5" />Reiniciar</Button
                  ><Button
                    variant="outline"
                    :disabled="power.isPending.value"
                    @click="execute(guest, 'shutdown')"
                    ><Power class="h-3.5 w-3.5" />Desligar</Button
                  ><Button
                    variant="danger"
                    :disabled="power.isPending.value"
                    @click="execute(guest, 'stop')"
                    ><Square class="h-3.5 w-3.5" />Parar</Button
                  ></template
                ><Button
                  variant="danger"
                  :disabled="destructive.isPending.value"
                  @click="removeGuest(guest)"
                  ><Trash2 class="h-3.5 w-3.5" />Excluir</Button
                >
              </div>
            </div>
            <p
              v-if="!guests.length"
              class="p-10 text-center text-sm text-muted"
            >
              Nenhum recurso retornado.
            </p>
          </div>
          <div v-else-if="tab === 'storage'" class="divide-y divide-line/60">
            <div
              v-for="store in inventory.data.value?.storage"
              :key="`${store.cluster}-${store.node}-${store.storage}`"
              class="grid gap-3 p-4 md:grid-cols-[1fr_180px_180px]"
            >
              <div>
                <p class="text-sm text-slate-200">{{ store.storage }}</p>
                <p class="mt-1 font-mono text-[9px] text-muted">
                  {{ store.cluster }} / {{ store.node }} · {{ store.type }} ·
                  {{ store.shared ? "compartilhado" : "local" }}
                </p>
              </div>
              <StatusBadge
                :status="store.active ? 'healthy' : 'critical'"
                :label="store.status"
              />
              <p class="font-mono text-xs text-muted">
                {{ formatBytes(store.used) }} /
                {{ formatBytes(store.total) }} ({{
                  store.total
                    ? ((store.used / store.total) * 100).toFixed(1)
                    : 0
                }}%)
              </p>
            </div>
          </div>
          <div v-else class="grid gap-px bg-line/60 xl:grid-cols-2">
            <form
              class="space-y-4 bg-panel p-5"
              @submit.prevent="createGuest('qemu')"
            >
              <div>
                <h3 class="flex items-center gap-2 text-sm text-white">
                  <Plus class="h-4 w-4 text-signal" />Nova VM QEMU
                </h3>
                <p class="mt-1 text-[10px] text-muted">
                  Cria disco SCSI, rede VirtIO e boot Linux. Informe um volume
                  ISO no formato storage:iso/arquivo.iso se necessário.
                </p>
              </div>
              <div class="grid gap-3 md:grid-cols-2">
                <label
                  v-for="field in [
                    { k: 'name', l: 'Nome' },
                    { k: 'vmid', l: 'VMID', t: 'number' },
                    { k: 'cores', l: 'vCPUs', t: 'number' },
                    { k: 'memory_mb', l: 'Memória MB', t: 'number' },
                    { k: 'storage', l: 'Storage' },
                    { k: 'disk_gb', l: 'Disco GB', t: 'number' },
                    { k: 'iso', l: 'ISO (opcional)' },
                    { k: 'bridge', l: 'Bridge' },
                  ]"
                  :key="field.k"
                  class="text-xs text-muted"
                  >{{ field.l
                  }}<input
                    v-model="(qemuForm as any)[field.k]"
                    :type="field.t || 'text'"
                    :required="field.k !== 'iso'"
                    class="mt-1 w-full rounded-lg border border-line bg-slate-950 p-2" /></label
                ><label class="text-xs text-muted"
                  >Cluster<select
                    v-model="qemuForm.cluster"
                    required
                    class="mt-1 w-full rounded-lg border border-line bg-slate-950 p-2"
                  >
                    <option value="" disabled>Selecione</option>
                    <option
                      v-for="value in [
                        ...new Set(
                          (inventory.data.value?.nodes ?? []).map(
                            (n) => n.cluster,
                          ),
                        ),
                      ]"
                      :key="value"
                    >
                      {{ value }}
                    </option>
                  </select></label
                ><label class="text-xs text-muted"
                  >Node<select
                    v-model="qemuForm.node"
                    required
                    class="mt-1 w-full rounded-lg border border-line bg-slate-950 p-2"
                  >
                    <option value="" disabled>Selecione</option>
                    <option
                      v-for="node in (inventory.data.value?.nodes ?? []).filter(
                        (n) =>
                          !qemuForm.cluster || n.cluster === qemuForm.cluster,
                      )"
                      :key="node.cluster + node.node"
                    >
                      {{ node.node }}
                    </option>
                  </select></label
                >
              </div>
              <label class="flex gap-2 text-xs text-muted"
                ><input v-model="qemuForm.start" type="checkbox" />Iniciar após
                criar</label
              ><Button type="submit">Criar QEMU</Button>
            </form>
            <form
              class="space-y-4 bg-panel p-5"
              @submit.prevent="createGuest('lxc')"
            >
              <div>
                <h3 class="flex items-center gap-2 text-sm text-white">
                  <Plus class="h-4 w-4 text-pulse" />Novo container LXC
                </h3>
                <p class="mt-1 text-[10px] text-muted">
                  Cria um container não privilegiado por padrão, com DHCP. Senha
                  e chave SSH nunca são auditadas.
                </p>
              </div>
              <div class="grid gap-3 md:grid-cols-2">
                <label
                  v-for="field in [
                    { k: 'hostname', l: 'Hostname' },
                    { k: 'vmid', l: 'VMID', t: 'number' },
                    { k: 'cores', l: 'vCPUs', t: 'number' },
                    { k: 'memory_mb', l: 'Memória MB', t: 'number' },
                    { k: 'storage', l: 'Storage' },
                    { k: 'rootfs_gb', l: 'RootFS GB', t: 'number' },
                    { k: 'template', l: 'Template' },
                    { k: 'bridge', l: 'Bridge' },
                    { k: 'password', l: 'Senha', t: 'password' },
                    { k: 'ssh_public_keys', l: 'Chave pública SSH' },
                  ]"
                  :key="field.k"
                  class="text-xs text-muted"
                  >{{ field.l
                  }}<input
                    v-model="(lxcForm as any)[field.k]"
                    :type="field.t || 'text'"
                    :required="
                      !['password', 'ssh_public_keys'].includes(field.k)
                    "
                    class="mt-1 w-full rounded-lg border border-line bg-slate-950 p-2" /></label
                ><label class="text-xs text-muted"
                  >Cluster<select
                    v-model="lxcForm.cluster"
                    required
                    class="mt-1 w-full rounded-lg border border-line bg-slate-950 p-2"
                  >
                    <option value="" disabled>Selecione</option>
                    <option
                      v-for="value in [
                        ...new Set(
                          (inventory.data.value?.nodes ?? []).map(
                            (n) => n.cluster,
                          ),
                        ),
                      ]"
                      :key="value"
                    >
                      {{ value }}
                    </option>
                  </select></label
                ><label class="text-xs text-muted"
                  >Node<select
                    v-model="lxcForm.node"
                    required
                    class="mt-1 w-full rounded-lg border border-line bg-slate-950 p-2"
                  >
                    <option value="" disabled>Selecione</option>
                    <option
                      v-for="node in (inventory.data.value?.nodes ?? []).filter(
                        (n) =>
                          !lxcForm.cluster || n.cluster === lxcForm.cluster,
                      )"
                      :key="node.cluster + node.node"
                    >
                      {{ node.node }}
                    </option>
                  </select></label
                >
              </div>
              <div class="flex gap-4">
                <label class="flex gap-2 text-xs text-muted"
                  ><input v-model="lxcForm.unprivileged" type="checkbox" />Não
                  privilegiado</label
                ><label class="flex gap-2 text-xs text-muted"
                  ><input
                    v-model="lxcForm.start"
                    type="checkbox"
                  />Iniciar</label
                >
              </div>
              <Button type="submit">Criar LXC</Button>
            </form>
          </div>
        </article>
      </div>
      <aside class="rounded-xl border border-line bg-panel/50 p-5">
        <p class="font-mono text-[9px] uppercase tracking-widest text-muted">
          Última tarefa de sincronização
        </p>
        <div v-if="lastJob" class="mt-4">
          <StatusBadge
            :status="
              lastJob.status === 'succeeded'
                ? 'healthy'
                : lastJob.status === 'failed'
                  ? 'critical'
                  : 'info'
            "
            :label="lastJob.status"
          />
          <p class="mt-4 break-all font-mono text-[10px] text-muted">
            {{ lastJob.id }}
          </p>
          <p class="mt-2 text-xs text-slate-300">
            Tentativa {{ lastJob.attempts }} / {{ lastJob.max_attempts }}
          </p>
          <p
            v-if="lastJob.last_error"
            class="mt-3 text-xs leading-5 text-danger"
          >
            {{ lastJob.last_error }}
          </p>
        </div>
        <p v-else class="mt-4 text-xs text-muted">
          Nenhuma sincronização enfileirada.
        </p>
      </aside>
    </section>
    <div
      v-if="snapshotGuest"
      class="fixed inset-0 z-50 flex items-center justify-center bg-black/80 p-4"
      @click.self="closeSnapshots"
    >
      <article
        class="w-full max-w-2xl overflow-hidden rounded-xl border border-line bg-panel shadow-2xl"
      >
        <header
          class="flex items-center justify-between border-b border-line bg-slate-950/40 p-5"
        >
          <div>
            <h2 class="text-lg font-medium">
              Snapshots · {{ snapshotGuest.type.toUpperCase() }}
              {{ snapshotGuest.vmid }}
            </h2>
            <p class="mt-1 text-xs text-muted">
              {{ snapshotGuest.name || "sem nome" }} ·
              {{ snapshotGuest.cluster }} / {{ snapshotGuest.node }}
            </p>
          </div>
          <Button variant="outline" @click="closeSnapshots">Fechar</Button>
        </header>
        <div
          v-if="snapshots.isError.value"
          class="m-5 rounded-lg border border-danger/20 bg-danger/5 p-4 text-sm text-danger"
        >
          Erro ao carregar snapshots:
          {{ snapshots.error.value?.message }}
        </div>
        <div class="max-h-[60vh] divide-y divide-line/60 overflow-y-auto">
          <div
            v-for="snap in snapshots.data.value"
            :key="snap.name"
            class="flex items-center justify-between gap-4 p-5"
          >
            <div class="min-w-0 flex-1">
              <p class="font-mono text-sm text-white">{{ snap.name }}</p>
              <p class="mt-1 text-xs text-muted">
                {{ snap.description || "sem descrição" }}
              </p>
              <p class="mt-1 font-mono text-[10px] text-muted">
                {{
                  snap.snaptime
                    ? new Date(snap.snaptime * 1000).toLocaleString("pt-BR")
                    : "—"
                }}
                <template v-if="snap.parent">· Parent: {{ snap.parent }}</template>
                <template v-if="snap.vmstate">· COM estado de RAM</template>
              </p>
            </div>
            <Button
              variant="danger"
              size="sm"
              :disabled="removeSnapshot.isPending.value"
              @click="deleteSnapshot(snap.name)"
              ><Trash2 class="h-3.5 w-3.5" />Excluir</Button
            >
          </div>
          <p
            v-if="
              !snapshots.isLoading.value &&
              !snapshots.data.value?.length
            "
            class="p-10 text-center text-sm text-muted"
          >
            Nenhum snapshot encontrado.
          </p>
        </div>
        <form
          class="border-t border-line bg-slate-950/20 p-5"
          @submit.prevent="submitSnapshot"
        >
          <h3 class="mb-4 text-sm font-medium">Criar novo snapshot</h3>
          <div class="grid gap-3 md:grid-cols-2">
            <label class="text-xs text-muted"
              >Nome<input
                v-model="snapshotForm.name"
                type="text"
                required
                placeholder="backup-20260720"
                class="mt-1 w-full rounded-lg border border-line bg-slate-950 p-2" /></label
            ><label class="text-xs text-muted"
              >Descrição<input
                v-model="snapshotForm.description"
                type="text"
                placeholder="Antes da atualização"
                class="mt-1 w-full rounded-lg border border-line bg-slate-950 p-2"
            /></label>
          </div>
          <p class="mt-3 text-[10px] text-muted">
            Snapshots são criados assincronamente pelo Proxmox. Atualize a lista
            após alguns segundos.
          </p>
          <Button
            type="submit"
            :disabled="createSnapshot.isPending.value"
            class="mt-4"
            ><Camera class="h-4 w-4" />Criar Snapshot</Button
          >
        </form>
      </article>
    </div>
  </div>
</template>
