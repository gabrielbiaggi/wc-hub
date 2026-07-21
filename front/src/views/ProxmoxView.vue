<script setup lang="ts">
import { computed, ref } from "vue";
import { useMutation, useQuery, useQueryClient } from "@tanstack/vue-query";
import {
  Boxes,
  Camera,
  Copy,
  Database,
  ArrowRightLeft,
  HardDrive,
  MemoryStick,
  Network,
  Play,
  Plus,
  Power,
  RefreshCw,
  RotateCcw,
  Server,
  Settings,
  Shield,
  ShieldCheck,
  Square,
  Trash2,
  Undo2,
} from "lucide-vue-next";
import Button from "@/components/ui/Button.vue";
import StatusBadge from "@/components/ui/StatusBadge.vue";
import ActionGuardModal from "@/components/ActionGuardModal.vue";
import { usePermissions } from "@/composables/usePermissions";
import {
  cloneProxmoxGuest,
  createProxmoxBackup,
  createProxmoxFirewallRule,
  createProxmoxLXC,
  createProxmoxQEMU,
  createProxmoxSnapshot,
  deleteProxmoxFirewallRule,
  deleteProxmoxGuest,
  deleteProxmoxSnapshot,
  getJobs,
  getProxmoxGuestFirewallRules,
  getProxmoxInventory,
  getProxmoxNodeBackups,
  getProxmoxNodeNetwork,
  getProxmoxSnapshots,
  getProxmoxSummary,
  migrateProxmoxGuest,
  resizeProxmoxDisk,
  rollbackProxmoxSnapshot,
  runProxmoxPowerAction,
  syncProxmox,
  updateProxmoxConfig,
  type ProxmoxBackupInfo,
  type ProxmoxFirewallRule,
  type ProxmoxGuest,
  type ProxmoxLXCInput,
  type ProxmoxNetworkInterface,
  type ProxmoxQEMUInput,
  type ProxmoxSnapshot,
  buildActionHeaders,
} from "@/lib/api";
import { apiErrorMessage } from "@/lib/api_error";

type Tab = "qemu" | "lxc" | "storage" | "network" | "provision";
const tab = ref<Tab>("qemu");
const { hasPermission } = usePermissions();
const canManage = computed(() => hasPermission("proxmox.manage"));
type GuardedProxmoxAction =
  | {
      kind: "power";
      guest: ProxmoxGuest;
      action: "stop" | "shutdown" | "reboot" | "reset";
      target: string;
    }
  | { kind: "delete"; guest: ProxmoxGuest; target: string }
  | {
      kind: "snapshot-delete" | "snapshot-rollback";
      guest: ProxmoxGuest;
      snapshot: string;
      target: string;
    }
  | {
      kind: "migrate";
      guest: ProxmoxGuest;
      targetNode: string;
      online: boolean;
      target: string;
    }
  | {
      kind: "config";
      guest: ProxmoxGuest;
      config: Record<string, string>;
      target: string;
    }
  | {
      kind: "resize";
      guest: ProxmoxGuest;
      disk: string;
      size: string;
      target: string;
    };
const guardedAction = ref<GuardedProxmoxAction | null>(null);
const guardedTarget = computed(() => guardedAction.value?.target ?? "");
const networkNode = ref<{ cluster: string; node: string } | null>(null);
const firewallGuest = ref<ProxmoxGuest | null>(null);
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
    headers?: Record<string, string>;
  }) =>
    runProxmoxPowerAction(
      input.guest.cluster,
      input.guest.node,
      input.guest.type,
      input.guest.vmid,
      input.action,
      input.headers,
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
  mutationFn: (input: {
    guest: ProxmoxGuest;
    headers?: Record<string, string>;
  }) => deleteProxmoxGuest(input.guest, true, input.headers),
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
  mutationFn: (input: { name: string; headers?: Record<string, string> }) => {
    const g = snapshotGuest.value;
    if (!g) throw new Error("No guest selected");
    return deleteProxmoxSnapshot(
      g.cluster,
      g.node,
      g.type,
      g.vmid,
      input.name,
      input.headers,
    );
  },
  onSuccess: () =>
    setTimeout(
      () => client.invalidateQueries({ queryKey: ["proxmox-snapshots"] }),
      1000,
    ),
});
const restoreSnapshot = useMutation({
  mutationFn: (input: { name: string; headers?: Record<string, string> }) => {
    const g = snapshotGuest.value;
    if (!g) throw new Error("No guest selected");
    return rollbackProxmoxSnapshot(
      g.cluster,
      g.node,
      g.type,
      g.vmid,
      input.name,
      input.headers,
    );
  },
  onSuccess: () => {
    setTimeout(() => {
      client.invalidateQueries({ queryKey: ["proxmox-snapshots"] });
      client.invalidateQueries({ queryKey: ["proxmox-inventory"] });
    }, 1500);
  },
});
const migrateGuest = useMutation({
  mutationFn: (input: {
    guest: ProxmoxGuest;
    targetNode: string;
    online: boolean;
    headers?: Record<string, string>;
  }) =>
    migrateProxmoxGuest(
      input.guest.cluster,
      input.guest.node,
      input.guest.type,
      input.guest.vmid,
      input.targetNode,
      input.online,
      input.headers,
    ),
  onSuccess: () =>
    setTimeout(
      () => client.invalidateQueries({ queryKey: ["proxmox-inventory"] }),
      2000,
    ),
});
const updateConfig = useMutation({
  mutationFn: (input: {
    guest: ProxmoxGuest;
    config: Record<string, string>;
    headers?: Record<string, string>;
  }) =>
    updateProxmoxConfig(
      input.guest.cluster,
      input.guest.node,
      input.guest.type,
      input.guest.vmid,
      input.config,
      input.headers,
    ),
  onSuccess: () =>
    setTimeout(
      () => client.invalidateQueries({ queryKey: ["proxmox-inventory"] }),
      1500,
    ),
});
const resizeDisk = useMutation({
  mutationFn: (input: {
    guest: ProxmoxGuest;
    disk: string;
    size: string;
    headers?: Record<string, string>;
  }) =>
    resizeProxmoxDisk(
      input.guest.cluster,
      input.guest.node,
      input.guest.type,
      input.guest.vmid,
      input.disk,
      input.size,
      input.headers,
    ),
  onSuccess: () =>
    setTimeout(
      () => client.invalidateQueries({ queryKey: ["proxmox-inventory"] }),
      1500,
    ),
});
const networkInterfaces = useQuery({
  queryKey: computed(() => [
    "proxmox-network",
    networkNode.value?.cluster,
    networkNode.value?.node,
  ]),
  queryFn: () => {
    const n = networkNode.value;
    if (!n) return Promise.resolve([]);
    return getProxmoxNodeNetwork(n.cluster, n.node);
  },
  enabled: computed(() => !!networkNode.value),
  refetchInterval: false,
});
const firewallRules = useQuery({
  queryKey: computed(() => [
    "proxmox-firewall",
    firewallGuest.value?.cluster,
    firewallGuest.value?.node,
    firewallGuest.value?.type,
    firewallGuest.value?.vmid,
  ]),
  queryFn: () => {
    const g = firewallGuest.value;
    if (!g) return Promise.resolve([]);
    return getProxmoxGuestFirewallRules(g.cluster, g.node, g.type, g.vmid);
  },
  enabled: computed(() => !!firewallGuest.value),
  refetchInterval: false,
});
const firewallRuleForm = ref({
  type: "in",
  action: "ACCEPT",
  proto: "tcp",
  dport: "",
  source: "",
  comment: "",
});
const createFirewallRule = useMutation({
  mutationFn: () => {
    const g = firewallGuest.value;
    if (!g) throw new Error("No guest selected");
    const rule: Record<string, string> = {
      type: firewallRuleForm.value.type,
      action: firewallRuleForm.value.action,
      enable: "1",
    };
    if (firewallRuleForm.value.proto) rule.proto = firewallRuleForm.value.proto;
    if (firewallRuleForm.value.dport) rule.dport = firewallRuleForm.value.dport;
    if (firewallRuleForm.value.source)
      rule.source = firewallRuleForm.value.source;
    if (firewallRuleForm.value.comment)
      rule.comment = firewallRuleForm.value.comment;
    return createProxmoxFirewallRule(g.cluster, g.node, g.type, g.vmid, rule);
  },
  onSuccess: () => {
    firewallRuleForm.value = {
      type: "in",
      action: "ACCEPT",
      proto: "tcp",
      dport: "",
      source: "",
      comment: "",
    };
    setTimeout(
      () => client.invalidateQueries({ queryKey: ["proxmox-firewall"] }),
      800,
    );
  },
});
const removeFirewallRule = useMutation({
  mutationFn: (pos: number) => {
    const g = firewallGuest.value;
    if (!g) throw new Error("No guest selected");
    return deleteProxmoxFirewallRule(g.cluster, g.node, g.type, g.vmid, pos);
  },
  onSuccess: () =>
    setTimeout(
      () => client.invalidateQueries({ queryKey: ["proxmox-firewall"] }),
      800,
    ),
});
const backupForm = ref({
  storage: "local",
  mode: "snapshot",
  compress: "zstd",
});
const createBackup = useMutation({
  mutationFn: (guest: ProxmoxGuest) =>
    createProxmoxBackup(
      guest.cluster,
      guest.node,
      guest.type,
      guest.vmid,
      backupForm.value.storage,
      backupForm.value.mode,
      backupForm.value.compress,
    ),
  onSuccess: () => {
    client.invalidateQueries({ queryKey: ["jobs"] });
    setTimeout(
      () => client.invalidateQueries({ queryKey: ["proxmox-backups"] }),
      3000,
    );
  },
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
      removeSnapshot.error.value ??
      restoreSnapshot.error.value ??
      migrateGuest.error.value ??
      updateConfig.error.value ??
      resizeDisk.error.value ??
      createFirewallRule.error.value ??
      removeFirewallRule.error.value ??
      createBackup.error.value,
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
        qemu: virtualMachines.filter((guest) => guest.cluster === cluster)
          .length,
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
  if (!canManage.value) return;
  if (action === "start") {
    power.mutate({ guest, action });
    return;
  }
  guardedAction.value = {
    kind: "power",
    guest,
    action,
    target: `${guest.cluster}/${guest.node}/${guest.type}/${guest.vmid}`,
  };
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
  if (!canManage.value) return;
  guardedAction.value = {
    kind: "delete",
    guest,
    target: `${guest.cluster}/${guest.node}/${guest.type}/${guest.vmid}`,
  };
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
  const guest = snapshotGuest.value;
  if (!guest || !canManage.value) return;
  guardedAction.value = {
    kind: "snapshot-delete",
    guest,
    snapshot: name,
    target: `${guest.cluster}/${guest.node}/${guest.type}/${guest.vmid}/snapshot/${name}`,
  };
};
const rollbackSnapshot = (name: string) => {
  const g = snapshotGuest.value;
  if (!g) return;
  if (!canManage.value) return;
  guardedAction.value = {
    kind: "snapshot-rollback",
    guest: g,
    snapshot: name,
    target: `${g.cluster}/${g.node}/${g.type}/${g.vmid}/snapshot/${name}`,
  };
};
const confirmGuardedAction = (payload: {
  confirmation: string;
  totpCode: string;
}) => {
  const guarded = guardedAction.value;
  if (!guarded) return;
  const headers = buildActionHeaders(payload.confirmation, payload.totpCode);
  if (guarded.kind === "power")
    power.mutate({ guest: guarded.guest, action: guarded.action, headers });
  else if (guarded.kind === "delete")
    destructive.mutate({ guest: guarded.guest, headers });
  else if (guarded.kind === "snapshot-delete")
    removeSnapshot.mutate({ name: guarded.snapshot, headers });
  else if (guarded.kind === "snapshot-rollback")
    restoreSnapshot.mutate({ name: guarded.snapshot, headers });
  else if (guarded.kind === "migrate")
    migrateGuest.mutate({
      guest: guarded.guest,
      targetNode: guarded.targetNode,
      online: guarded.online,
      headers,
    });
  else if (guarded.kind === "config")
    updateConfig.mutate({
      guest: guarded.guest,
      config: guarded.config,
      headers,
    });
  else if (guarded.kind === "resize")
    resizeDisk.mutate({
      guest: guarded.guest,
      disk: guarded.disk,
      size: guarded.size,
      headers,
    });
  guardedAction.value = null;
};
const migrate = (guest: ProxmoxGuest) => {
  const availableNodes = (inventory.data.value?.nodes ?? [])
    .filter(
      (n) =>
        n.cluster === guest.cluster &&
        n.node !== guest.node &&
        n.status === "online",
    )
    .map((n) => n.node);

  if (availableNodes.length === 0) {
    window.alert("Não há nós disponíveis para migração neste cluster.");
    return;
  }

  const targetNode = window.prompt(
    `Migrar ${guest.type.toUpperCase()} ${guest.vmid} (${guest.name}) para qual nó?\n\nNós disponíveis: ${availableNodes.join(", ")}\n\nDigite o nome do nó:`,
  );

  if (!targetNode || !availableNodes.includes(targetNode)) {
    if (targetNode) window.alert("Nó inválido ou indisponível.");
    return;
  }

  const online =
    guest.status === "running" &&
    window.confirm(
      `VM está em execução. Deseja realizar migração online (live migration)?\n\nSim = migração online (sem downtime)\nNão = migração offline (VM será desligada)`,
    );

  guardedAction.value = {
    kind: "migrate",
    guest,
    targetNode,
    online,
    target: `${guest.cluster}/${guest.node}/${guest.type}/${guest.vmid}`,
  };
};
const editResources = (guest: ProxmoxGuest) => {
  const currentCores = guest.cpus;
  const currentMem = Math.floor(guest.maxmem / (1024 * 1024));

  const coresInput = window.prompt(
    `Editar recursos de ${guest.type.toUpperCase()} ${guest.vmid} (${guest.name})\n\nCPU atual: ${currentCores} cores\nNovo valor (ou deixe em branco para manter):`,
    currentCores.toString(),
  );

  if (coresInput === null) return;

  const memInput = window.prompt(
    `RAM atual: ${currentMem} MB\nNovo valor em MB (ou deixe em branco para manter):`,
    currentMem.toString(),
  );

  if (memInput === null) return;

  const tagsInput = window.prompt(
    `Tags (separadas por ;):\nExemplo: production;web;backup`,
    "",
  );

  if (tagsInput === null) return;

  const config: Record<string, string> = {};

  if (coresInput && coresInput !== currentCores.toString()) {
    config.cores = coresInput;
  }

  if (memInput && memInput !== currentMem.toString()) {
    config.memory = memInput;
  }

  if (tagsInput.trim()) {
    config.tags = tagsInput.trim();
  }

  if (Object.keys(config).length === 0) {
    window.alert("Nenhuma alteração foi feita.");
    return;
  }

  guardedAction.value = {
    kind: "config",
    guest,
    config,
    target: `${guest.cluster}/${guest.node}/${guest.type}/${guest.vmid}`,
  };
};
const resizeGuestDisk = (guest: ProxmoxGuest) => {
  const disk = window.prompt(
    `Disco de ${guest.type.toUpperCase()} ${guest.vmid}. Informe o identificador Proxmox (ex.: ${guest.type === "qemu" ? "scsi0" : "rootfs"}).`,
    guest.type === "qemu" ? "scsi0" : "rootfs",
  );
  if (!disk) return;
  const size = window.prompt(
    "Aumento do disco (somente crescimento; ex.: +20G):",
    "+20G",
  );
  if (!size || !/^\+[1-9][0-9]*(?:[KMGT])?$/i.test(size.trim())) {
    if (size) window.alert("Use apenas aumento positivo, por exemplo +20G.");
    return;
  }
  guardedAction.value = {
    kind: "resize",
    guest,
    disk: disk.trim(),
    size: size.trim(),
    target: `${guest.cluster}/${guest.node}/${guest.type}/${guest.vmid}`,
  };
};
const viewNodeNetwork = (cluster: string, node: string) => {
  networkNode.value = { cluster, node };
  tab.value = "network";
};
const openFirewall = (guest: ProxmoxGuest) => {
  firewallGuest.value = guest;
};
const closeFirewall = () => {
  firewallGuest.value = null;
  firewallRuleForm.value = {
    type: "in",
    action: "ACCEPT",
    proto: "tcp",
    dport: "",
    source: "",
    comment: "",
  };
};
const submitFirewallRule = () => {
  if (
    window.confirm(
      `Criar regra de firewall ${firewallRuleForm.value.action} para ${firewallGuest.value?.type.toUpperCase()} ${firewallGuest.value?.vmid}?`,
    )
  )
    createFirewallRule.mutate();
};
const deleteFirewallRule = (pos: number) => {
  const phrase = `EXCLUIR ${pos}`;
  if (
    window.prompt(
      `Excluir regra de firewall na posição ${pos}. Digite exatamente: ${phrase}`,
    ) === phrase
  ) {
    removeFirewallRule.mutate(pos);
  }
};
const backupGuest = (guest: ProxmoxGuest) => {
  const availableStorages = (inventory.data.value?.storage ?? [])
    .filter(
      (s) =>
        s.cluster === guest.cluster &&
        s.active &&
        s.type !== "lvm" &&
        s.type !== "lvmthin",
    )
    .map((s) => s.storage);

  if (availableStorages.length === 0) {
    window.alert("Nenhum storage disponível para backup neste cluster.");
    return;
  }

  const storage = window.prompt(
    `Criar backup de ${guest.type.toUpperCase()} ${guest.vmid} (${guest.name})\n\nStorages disponíveis: ${availableStorages.join(", ")}\n\nDigite o nome do storage:`,
    availableStorages[0],
  );

  if (!storage || !availableStorages.includes(storage)) {
    if (storage) window.alert("Storage inválido ou indisponível.");
    return;
  }

  backupForm.value.storage = storage;

  if (
    window.confirm(
      `Confirma backup de ${guest.type.toUpperCase()} ${guest.vmid} (${guest.name})?\n\nStorage: ${storage}\nModo: ${backupForm.value.mode}\nCompressão: ${backupForm.value.compress}\n\nO backup será executado em background.`,
    )
  ) {
    createBackup.mutate(guest);
  }
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
        :disabled="
          !canManage || !summary.data.value?.configured || sync.isPending.value
        "
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
        <article
          class="overflow-hidden rounded-xl border border-line bg-panel/65"
        >
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
                  <p class="font-mono text-xs text-signal">
                    {{ machine.cluster }}
                  </p>
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
                      :status="
                        node.status === 'online' ? 'healthy' : 'critical'
                      "
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
            <p
              v-if="!machines.length"
              class="p-10 text-center text-sm text-muted"
            >
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
                  { id: 'network', label: 'Rede' },
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
                ><Button variant="outline" @click="openFirewall(guest)"
                  ><Shield class="h-3.5 w-3.5" />Firewall</Button
                ><Button
                  variant="outline"
                  :disabled="!canManage"
                  @click="backupGuest(guest)"
                  ><HardDrive class="h-3.5 w-3.5" />Backup</Button
                ><Button
                  variant="outline"
                  :disabled="!canManage"
                  @click="migrate(guest)"
                  ><ArrowRightLeft class="h-3.5 w-3.5" />Migrar</Button
                ><Button
                  variant="outline"
                  :disabled="!canManage"
                  @click="cloneGuest(guest)"
                  ><Copy class="h-3.5 w-3.5" />Clonar</Button
                ><Button
                  variant="outline"
                  :disabled="!canManage"
                  @click="editResources(guest)"
                  ><Settings class="h-3.5 w-3.5" />Config</Button
                ><Button
                  variant="outline"
                  :disabled="!canManage || resizeDisk.isPending.value"
                  @click="resizeGuestDisk(guest)"
                  ><HardDrive class="h-3.5 w-3.5" />Disco</Button
                ><Button
                  v-if="guest.status !== 'running'"
                  variant="outline"
                  :disabled="!canManage || power.isPending.value"
                  @click="execute(guest, 'start')"
                  ><Play class="h-3.5 w-3.5" />Iniciar</Button
                ><template v-else
                  ><Button
                    variant="outline"
                    :disabled="!canManage || power.isPending.value"
                    @click="execute(guest, 'reboot')"
                    ><RotateCcw class="h-3.5 w-3.5" />Reiniciar</Button
                  ><Button
                    variant="outline"
                    :disabled="!canManage || power.isPending.value"
                    @click="execute(guest, 'shutdown')"
                    ><Power class="h-3.5 w-3.5" />Desligar</Button
                  ><Button
                    variant="danger"
                    :disabled="!canManage || power.isPending.value"
                    @click="execute(guest, 'stop')"
                    ><Square class="h-3.5 w-3.5" />Parar</Button
                  ></template
                ><Button
                  variant="danger"
                  :disabled="!canManage || destructive.isPending.value"
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
          <div v-else-if="tab === 'network'" class="divide-y divide-line/60">
            <div v-if="!networkNode" class="p-10 text-center">
              <Network class="mx-auto h-12 w-12 text-muted" />
              <p class="mt-4 text-sm text-muted">
                Selecione um nó para visualizar interfaces de rede
              </p>
              <div class="mt-6 grid gap-2 md:grid-cols-2">
                <Button
                  v-for="node in inventory.data.value?.nodes ?? []"
                  :key="`${node.cluster}-${node.node}`"
                  variant="outline"
                  @click="viewNodeNetwork(node.cluster, node.node)"
                >
                  <Server class="h-4 w-4" />{{ node.cluster }} / {{ node.node }}
                </Button>
              </div>
            </div>
            <div v-else>
              <div class="bg-slate-950/20 p-4">
                <div class="flex items-center justify-between">
                  <div>
                    <p class="text-sm font-medium">
                      Interfaces de rede · {{ networkNode.node }}
                    </p>
                    <p class="mt-1 text-xs text-muted">
                      {{ networkNode.cluster }}
                    </p>
                  </div>
                  <Button
                    variant="outline"
                    size="sm"
                    @click="networkNode = null"
                    >Voltar</Button
                  >
                </div>
              </div>
              <div
                v-if="networkInterfaces.isError.value"
                class="m-5 rounded-lg border border-danger/20 bg-danger/5 p-4 text-sm text-danger"
              >
                Erro ao carregar interfaces:
                {{ networkInterfaces.error.value?.message }}
              </div>
              <div v-else class="divide-y divide-line/60">
                <div
                  v-for="iface in networkInterfaces.data.value"
                  :key="iface.id"
                  class="grid gap-3 p-4 md:grid-cols-[1fr_120px_200px]"
                >
                  <div>
                    <p class="font-mono text-sm text-white">{{ iface.name }}</p>
                    <p class="mt-1 text-xs text-muted">
                      Tipo: {{ iface.type }}
                      <template v-if="iface.bridge"
                        >· Bridge: {{ iface.bridge }}</template
                      >
                    </p>
                    <p
                      v-if="iface.address"
                      class="mt-1 font-mono text-[10px] text-muted"
                    >
                      {{ iface.address
                      }}{{ iface.netmask ? `/${iface.netmask}` : "" }}
                      <template v-if="iface.gateway"
                        >· GW: {{ iface.gateway }}</template
                      >
                    </p>
                  </div>
                  <StatusBadge
                    :status="iface.active ? 'healthy' : 'warning'"
                    :label="iface.active ? 'ativa' : 'inativa'"
                  />
                  <div class="flex items-center gap-2 text-xs text-muted">
                    <span
                      :class="iface.autostart ? 'text-signal' : 'text-muted'"
                    >
                      {{ iface.autostart ? "✓ Autostart" : "✗ Sem autostart" }}
                    </span>
                  </div>
                </div>
                <p
                  v-if="!networkInterfaces.data.value?.length"
                  class="p-10 text-center text-sm text-muted"
                >
                  Nenhuma interface encontrada.
                </p>
              </div>
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
              ><Button type="submit" :disabled="!canManage">Criar QEMU</Button>
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
              <Button type="submit" :disabled="!canManage">Criar LXC</Button>
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
                <template v-if="snap.parent"
                  >· Parent: {{ snap.parent }}</template
                >
                <template v-if="snap.vmstate">· COM estado de RAM</template>
              </p>
            </div>
            <div class="flex gap-2">
              <Button
                variant="outline"
                size="sm"
                :disabled="!canManage || restoreSnapshot.isPending.value"
                @click="rollbackSnapshot(snap.name)"
                ><Undo2 class="h-3.5 w-3.5" />Restaurar</Button
              ><Button
                variant="danger"
                size="sm"
                :disabled="!canManage || removeSnapshot.isPending.value"
                @click="deleteSnapshot(snap.name)"
                ><Trash2 class="h-3.5 w-3.5" />Excluir</Button
              >
            </div>
          </div>
          <p
            v-if="!snapshots.isLoading.value && !snapshots.data.value?.length"
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
            :disabled="!canManage || createSnapshot.isPending.value"
            class="mt-4"
            ><Camera class="h-4 w-4" />Criar Snapshot</Button
          >
        </form>
      </article>
    </div>
    <div
      v-if="firewallGuest"
      class="fixed inset-0 z-50 flex items-center justify-center bg-black/80 p-4"
      @click.self="closeFirewall"
    >
      <article
        class="w-full max-w-3xl overflow-hidden rounded-xl border border-line bg-panel shadow-2xl"
      >
        <header
          class="flex items-center justify-between border-b border-line bg-slate-950/40 p-5"
        >
          <div>
            <h2 class="text-lg font-medium">
              Firewall · {{ firewallGuest.type.toUpperCase() }}
              {{ firewallGuest.vmid }}
            </h2>
            <p class="mt-1 text-xs text-muted">
              {{ firewallGuest.name || "sem nome" }} ·
              {{ firewallGuest.cluster }} / {{ firewallGuest.node }}
            </p>
          </div>
          <Button variant="outline" @click="closeFirewall">Fechar</Button>
        </header>
        <div
          v-if="firewallRules.isError.value"
          class="m-5 rounded-lg border border-danger/20 bg-danger/5 p-4 text-sm text-danger"
        >
          Erro ao carregar regras de firewall:
          {{ firewallRules.error.value?.message }}
        </div>
        <div class="max-h-[50vh] divide-y divide-line/60 overflow-y-auto">
          <div
            v-for="rule in firewallRules.data.value"
            :key="rule.pos"
            class="flex items-center justify-between gap-4 p-4"
          >
            <div class="min-w-0 flex-1">
              <div class="flex items-center gap-2">
                <span
                  :class="[
                    'rounded px-2 py-0.5 font-mono text-xs font-medium',
                    rule.action === 'ACCEPT'
                      ? 'bg-green-500/20 text-green-400'
                      : 'bg-red-500/20 text-red-400',
                  ]"
                >
                  {{ rule.action }}
                </span>
                <span class="font-mono text-xs text-muted">
                  {{ rule.type.toUpperCase() }}
                </span>
                <span
                  v-if="rule.enable === 0"
                  class="rounded bg-warning/20 px-2 py-0.5 text-xs text-warning"
                >
                  DESABILITADA
                </span>
              </div>
              <p class="mt-2 font-mono text-xs text-slate-300">
                <template v-if="rule.proto">Proto: {{ rule.proto }}</template>
                <template v-if="rule.dport">
                  · Porta: {{ rule.dport }}</template
                >
                <template v-if="rule.source">
                  · Origem: {{ rule.source }}</template
                >
                <template v-if="rule.dest">
                  · Destino: {{ rule.dest }}</template
                >
                <template v-if="rule.sport">
                  · Sport: {{ rule.sport }}</template
                >
                <template v-if="rule.iface">
                  · Interface: {{ rule.iface }}</template
                >
              </p>
              <p v-if="rule.comment" class="mt-1 text-xs text-muted">
                {{ rule.comment }}
              </p>
            </div>
            <div class="flex items-center gap-3">
              <span class="font-mono text-xs text-muted">#{{ rule.pos }}</span>
              <Button
                variant="danger"
                size="sm"
                :disabled="!canManage || removeFirewallRule.isPending.value"
                @click="deleteFirewallRule(rule.pos)"
                ><Trash2 class="h-3.5 w-3.5" />Excluir</Button
              >
            </div>
          </div>
          <p
            v-if="
              !firewallRules.isLoading.value &&
              !firewallRules.data.value?.length
            "
            class="p-10 text-center text-sm text-muted"
          >
            Nenhuma regra de firewall configurada.
          </p>
        </div>
        <form
          class="border-t border-line bg-slate-950/20 p-5"
          @submit.prevent="submitFirewallRule"
        >
          <h3 class="mb-4 text-sm font-medium">Criar nova regra</h3>
          <div class="grid gap-3 md:grid-cols-3">
            <label class="text-xs text-muted"
              >Tipo<select
                v-model="firewallRuleForm.type"
                required
                class="mt-1 w-full rounded-lg border border-line bg-slate-950 p-2"
              >
                <option value="in">IN (entrada)</option>
                <option value="out">OUT (saída)</option>
              </select></label
            ><label class="text-xs text-muted"
              >Ação<select
                v-model="firewallRuleForm.action"
                required
                class="mt-1 w-full rounded-lg border border-line bg-slate-950 p-2"
              >
                <option value="ACCEPT">ACCEPT</option>
                <option value="REJECT">REJECT</option>
                <option value="DROP">DROP</option>
              </select></label
            ><label class="text-xs text-muted"
              >Protocolo<select
                v-model="firewallRuleForm.proto"
                class="mt-1 w-full rounded-lg border border-line bg-slate-950 p-2"
              >
                <option value="">Todos</option>
                <option value="tcp">TCP</option>
                <option value="udp">UDP</option>
                <option value="icmp">ICMP</option>
              </select></label
            ><label class="text-xs text-muted"
              >Porta destino<input
                v-model="firewallRuleForm.dport"
                type="text"
                placeholder="80, 443, 8080-8090"
                class="mt-1 w-full rounded-lg border border-line bg-slate-950 p-2" /></label
            ><label class="text-xs text-muted"
              >Origem (CIDR)<input
                v-model="firewallRuleForm.source"
                type="text"
                placeholder="0.0.0.0/0, 192.168.1.0/24"
                class="mt-1 w-full rounded-lg border border-line bg-slate-950 p-2" /></label
            ><label class="text-xs text-muted"
              >Comentário<input
                v-model="firewallRuleForm.comment"
                type="text"
                placeholder="Descrição da regra"
                class="mt-1 w-full rounded-lg border border-line bg-slate-950 p-2"
            /></label>
          </div>
          <p class="mt-3 text-[10px] text-muted">
            Regras são aplicadas imediatamente. A ordem importa: regras são
            avaliadas sequencialmente.
          </p>
          <Button
            type="submit"
            :disabled="!canManage || createFirewallRule.isPending.value"
            class="mt-4"
            ><Shield class="h-4 w-4" />Criar Regra</Button
          >
        </form>
      </article>
    </div>
    <ActionGuardModal
      :show="!!guardedAction"
      :target-name="guardedTarget"
      title="Operação Proxmox protegida"
      @cancel="guardedAction = null"
      @confirm="confirmGuardedAction"
    />
  </div>
</template>
