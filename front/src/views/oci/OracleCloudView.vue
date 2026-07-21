<script setup lang="ts">
import { computed, ref } from "vue";
import { useMutation, useQuery, useQueryClient } from "@tanstack/vue-query";
import {
  Boxes,
  Cloud,
  Cpu,
  Database,
  Globe2,
  HardDrive,
  MemoryStick,
  Network,
  Play,
  Plus,
  Power,
  RefreshCw,
  RotateCcw,
  ShieldCheck,
  Square,
  Waypoints,
} from "lucide-vue-next";
import Button from "@/components/ui/Button.vue";
import StatusBadge from "@/components/ui/StatusBadge.vue";
import {
  createOCIAutonomousDatabase,
  getOCIOverview,
  launchOCIInstance,
  runOCIInstanceAction,
  type OCICreateAutonomousDatabaseInput,
  type OCILaunchInstanceInput,
  type OCIInstance,
  type OCIInstanceAction,
} from "@/lib/api";
import { apiErrorMessage } from "@/lib/api_error";

type Tab = "instances" | "databases" | "provision" | "network" | "identity";
const tab = ref<Tab>("instances");
const regionFilter = ref("all");
const client = useQueryClient();
const overview = useQuery({
  queryKey: ["oci-overview"],
  queryFn: getOCIOverview,
  refetchInterval: 30000,
  retry: 1,
});
const action = useMutation({
  mutationFn: (input: { instance: OCIInstance; action: OCIInstanceAction }) =>
    runOCIInstanceAction(input.instance.id, input.action, input.instance.region),
  onSuccess: () =>
    setTimeout(
      () => client.invalidateQueries({ queryKey: ["oci-overview"] }),
      1800,
    ),
});
const launchForm = ref<OCILaunchInstanceInput>({
  region: "",
  compartment_id: "",
  availability_domain: "",
  display_name: "",
  shape: "VM.Standard.E4.Flex",
  image_id: "",
  subnet_id: "",
  ocpus: 1,
  memory_gb: 8,
  assign_public_ip: false,
  ssh_authorized_key: "",
});
const databaseForm = ref<OCICreateAutonomousDatabaseInput>({
  region: "",
  compartment_id: "",
  display_name: "",
  db_name: "",
  admin_password: "",
  workload: "OLTP",
  compute_count: 2,
  storage_tb: 1,
  free_tier: false,
  auto_scaling: true,
});
const launchMutation = useMutation({
  mutationFn: () => launchOCIInstance(launchForm.value),
  onSuccess: () => {
    tab.value = "instances";
    setTimeout(
      () => client.invalidateQueries({ queryKey: ["oci-overview"] }),
      2000,
    );
  },
});
const databaseMutation = useMutation({
  mutationFn: () => createOCIAutonomousDatabase(databaseForm.value),
  onSuccess: () => {
    databaseForm.value.admin_password = "";
    tab.value = "databases";
    setTimeout(
      () => client.invalidateQueries({ queryKey: ["oci-overview"] }),
      2000,
    );
  },
});
const running = computed(
  () =>
    (overview.data.value?.instances ?? []).filter(
      (item) => item.lifecycle_state === "RUNNING",
    ).length ?? 0,
);
const visibleInstances = computed(() => (overview.data.value?.instances ?? []).filter((item) => regionFilter.value === "all" || item.region === regionFilter.value));
const ociError = computed(() =>
  apiErrorMessage(
    overview.error.value ??
      action.error.value ??
      launchMutation.error.value ??
      databaseMutation.error.value,
    "Não foi possível concluir a operação OCI.",
  ),
);
const compartmentName = (id: string) =>
  (overview.data.value?.compartments ?? []).find((item) => item.id === id)
    ?.name ??
  "tenancy raiz";
const vcnName = (id: string) =>
  (overview.data.value?.vcns ?? []).find((item) => item.id === id)
    ?.display_name ??
  id.split(".").at(-1)?.slice(0, 12) ??
  "VCN";
const execute = (instance: OCIInstance, operation: OCIInstanceAction) => {
  const confirmation = `${operation.toUpperCase()} ${instance.display_name}`;
  if (
    window.prompt(`Ação real na OCI. Digite exatamente: ${confirmation}`) ===
    confirmation
  )
    action.mutate({ instance, action: operation });
};
const provisionInstance = () => {
  const phrase = `CRIAR ${launchForm.value.display_name}`;
  if (
    window.prompt(
      `A OCI poderá gerar cobrança. Digite exatamente: ${phrase}`,
    ) === phrase
  )
    launchMutation.mutate();
};
const provisionDatabase = () => {
  const phrase = `CRIAR BANCO ${databaseForm.value.db_name}`;
  if (
    window.prompt(
      `O Autonomous Database poderá gerar cobrança. Digite exatamente: ${phrase}`,
    ) === phrase
  )
    databaseMutation.mutate();
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
            :status="overview.isError.value ? 'critical' : 'healthy'"
            :label="
              overview.isError.value ? 'OCI indisponível' : 'API assinada ativa'
            "
          /><span
            class="inline-flex items-center gap-1.5 font-mono text-[10px] uppercase text-signal"
            ><ShieldCheck class="h-3.5 w-3.5" />chave de assinatura da API ·
            controle total auditado</span
          >
        </div>
        <h1 class="mt-4 text-3xl font-semibold">Oracle Cloud Infrastructure</h1>
        <p class="mt-2 text-sm text-muted">
          Tenancy <span class="font-medium text-slate-300">{{ overview.data.value?.tenancy_name || "configurada" }}</span> · recursos organizados por região, compartimento e domínio de disponibilidade.
        </p>
      </div>
      <Button
        variant="outline"
        :disabled="overview.isFetching.value"
        @click="overview.refetch()"
        ><RefreshCw
          :class="['h-4 w-4', overview.isFetching.value && 'animate-spin']"
        />Atualizar</Button
      >
    </header>
    <div
      v-if="
        overview.isError.value ||
        action.isError.value ||
        launchMutation.isError.value ||
        databaseMutation.isError.value
      "
      class="rounded-xl border border-danger/20 bg-danger/5 p-4 text-sm text-danger"
    >
      <p class="font-medium">Falha no adaptador OCI</p>
      <p class="mt-1 break-words font-mono text-xs">{{ ociError }}</p>
    </div>
    <section class="grid gap-3 sm:grid-cols-2 xl:grid-cols-6">
      <article
        v-for="item in [
          {
            label: 'Regiões',
            value: overview.data.value?.regions?.length ?? 0,
            icon: Globe2,
          },
          {
            label: 'Compartimentos',
            value: overview.data.value?.compartments?.length ?? 0,
            icon: Boxes,
          },
          {
            label: 'Instâncias',
            value: overview.data.value?.instances?.length ?? 0,
            icon: Cpu,
          },
          { label: 'Em execução', value: running, icon: Play },
          {
            label: 'Bancos OCI',
            value:
              (overview.data.value?.autonomous_databases?.length ?? 0) +
              (overview.data.value?.db_systems?.length ?? 0),
            icon: Database,
          },
          {
            label: 'Volumes de bloco',
            value: overview.data.value?.block_volumes?.length ?? 0,
            icon: HardDrive,
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
    <section class="overflow-hidden rounded-xl border border-line bg-panel/65">
      <header
        class="flex flex-col gap-3 border-b border-line p-4 md:flex-row md:items-center md:justify-between"
      >
        <div>
          <h2 class="text-sm font-medium">
            Inventário e administração da tenancy
          </h2>
          <p class="mt-1 font-mono text-[9px] text-muted">
            {{ overview.data.value?.tenancy_name || "Tenancy configurada" }} · principal {{ overview.data.value?.home_region || "—" }} · captura
            {{
              overview.data.value
                ? new Date(overview.data.value.captured_at).toLocaleString(
                    "pt-BR",
                  )
                : "—"
            }}
          </p>
        </div>
        <nav class="flex flex-wrap rounded-lg border border-line p-1">
          <button
            v-for="item in [
              { id: 'instances', label: 'Computação' },
              { id: 'databases', label: 'Bancos' },
              { id: 'provision', label: 'Criar recursos' },
              { id: 'network', label: 'Rede' },
              { id: 'identity', label: 'Regiões e ADs' },
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

      <div class="flex flex-wrap gap-2 border-b border-line px-4 py-3" aria-label="Filtro de região OCI">
        <button :class="['rounded-full border px-3 py-1 font-mono text-[10px]', regionFilter === 'all' ? 'border-signal/40 bg-signal/10 text-signal' : 'border-line text-muted']" @click="regionFilter = 'all'">Todas as regiões</button>
        <button v-for="region in overview.data.value?.regions" :key="region.name" :class="['rounded-full border px-3 py-1 font-mono text-[10px]', regionFilter === region.name ? 'border-signal/40 bg-signal/10 text-signal' : 'border-line text-muted']" @click="regionFilter = region.name">{{ region.name }}{{ region.home ? ' · principal' : '' }}</button>
      </div>

      <div v-if="tab === 'instances'" class="divide-y divide-line/60">
        <div
          v-for="instance in visibleInstances"
          :key="instance.id"
          class="grid gap-4 p-4 xl:grid-cols-[1fr_260px_320px] xl:items-center"
        >
          <div>
            <div class="flex flex-wrap items-center gap-2">
              <p class="text-sm text-slate-200">
                {{ instance.display_name || "instância sem nome" }}
              </p>
              <StatusBadge
                :status="
                  instance.lifecycle_state === 'RUNNING'
                    ? 'healthy'
                    : instance.lifecycle_state === 'STOPPED'
                      ? 'warning'
                      : 'info'
                "
                :label="instance.lifecycle_state"
              />
            </div>
            <p class="mt-2 font-mono text-[9px] text-muted">
              {{ instance.region || "região não informada" }} · {{ instance.shape }} · {{ instance.availability_domain }} ·
              {{ instance.fault_domain || "sem domínio de falha" }}
            </p>
            <p
              class="mt-1 truncate font-mono text-[9px] text-muted"
              :title="instance.id"
            >
              {{ instance.id }}
            </p>
          </div>
          <div class="grid grid-cols-2 gap-3">
            <div class="rounded-lg border border-line/60 p-3">
              <Cpu class="h-3.5 w-3.5 text-muted" />
              <p class="mt-2 font-mono text-sm">{{ instance.ocpus || "—" }}</p>
              <p class="text-[9px] text-muted">OCPUs</p>
            </div>
            <div class="rounded-lg border border-line/60 p-3">
              <MemoryStick class="h-3.5 w-3.5 text-muted" />
              <p class="mt-2 font-mono text-sm">
                {{ instance.memory_gb || "—" }} GB
              </p>
              <p class="text-[9px] text-muted">Memória</p>
            </div>
            <p class="col-span-2 truncate text-[10px] text-muted">
              {{ compartmentName(instance.compartment_id) }}
            </p>
          </div>
          <div class="flex flex-wrap justify-end gap-2">
            <Button
              v-if="instance.lifecycle_state !== 'RUNNING'"
              variant="outline"
              :disabled="action.isPending.value"
              @click="execute(instance, 'start')"
              ><Play class="h-3.5 w-3.5" />Iniciar</Button
            ><template v-else
              ><Button
                variant="outline"
                :disabled="action.isPending.value"
                @click="execute(instance, 'reboot')"
                ><RotateCcw class="h-3.5 w-3.5" />Reiniciar</Button
              ><Button
                variant="outline"
                :disabled="action.isPending.value"
                @click="execute(instance, 'shutdown')"
                ><Power class="h-3.5 w-3.5" />Desligar</Button
              ><Button
                variant="danger"
                :disabled="action.isPending.value"
                @click="execute(instance, 'stop')"
                ><Square class="h-3.5 w-3.5" />Parar</Button
              ></template
            >
          </div>
        </div>
        <p
          v-if="!overview.data.value?.instances?.length"
          class="p-10 text-center text-sm text-muted"
        >
          Nenhuma instância ativa retornada.
        </p>
      </div>

      <div v-else-if="tab === 'databases'" class="space-y-px bg-line/60">
        <article
          v-for="database in overview.data.value?.autonomous_databases"
          :key="database.id"
          class="grid gap-3 bg-panel p-5 lg:grid-cols-[1fr_180px_180px]"
        >
          <div>
            <div class="flex items-center gap-2">
              <Database class="h-4 w-4 text-signal" />
              <p class="text-sm text-slate-200">
                {{ database.display_name || database.db_name }}
              </p>
              <StatusBadge
                :status="
                  database.lifecycle_state === 'AVAILABLE'
                    ? 'healthy'
                    : 'warning'
                "
                :label="database.lifecycle_state"
              />
            </div>
            <p class="mt-2 font-mono text-[9px] text-muted">
              Autonomous {{ database.workload }} ·
              {{ compartmentName(database.compartment_id) }}
            </p>
          </div>
          <p class="text-xs text-muted">
            {{ database.compute_count }} {{ database.compute_model || "CPU"
            }}<br />{{ database.storage_tb }} TB
          </p>
          <p class="text-xs text-muted">
            {{ database.free_tier ? "Camada gratuita" : "Licença incluída" }}
          </p>
        </article>
        <article
          v-for="system in overview.data.value?.db_systems"
          :key="system.id"
          class="grid gap-3 bg-panel p-5 lg:grid-cols-[1fr_180px_180px]"
        >
          <div>
            <p class="text-sm text-slate-200">{{ system.display_name }}</p>
            <p class="mt-2 font-mono text-[9px] text-muted">
              DB System · {{ system.database_edition }} ·
              {{ system.availability_domain }}
            </p>
          </div>
          <StatusBadge
            :status="
              system.lifecycle_state === 'AVAILABLE' ? 'healthy' : 'warning'
            "
            :label="system.lifecycle_state"
          />
          <p class="text-xs text-muted">
            {{ system.shape }} · {{ system.cpu_core_count }} CPU ·
            {{ system.memory_gb }} GB
          </p>
        </article>
        <p
          v-if="
            !overview.data.value?.autonomous_databases?.length &&
            !overview.data.value?.db_systems?.length
          "
          class="bg-panel p-10 text-center text-sm text-muted"
        >
          Nenhum banco encontrado nos compartimentos acessíveis.
        </p>
      </div>

      <div
        v-else-if="tab === 'provision'"
        class="grid gap-px bg-line/60 xl:grid-cols-2"
      >
        <form
          class="space-y-4 bg-panel p-5"
          @submit.prevent="provisionInstance"
        >
          <div>
            <h3 class="flex items-center gap-2 text-sm text-white">
              <Plus class="h-4 w-4 text-signal" />Nova instância de computação
            </h3>
            <p class="mt-1 text-[10px] leading-4 text-muted">
              Cria uma VM a partir de uma imagem e sub-rede existentes. O IP
              público é opcional e vem desativado.
            </p>
          </div>
          <div class="grid gap-3 md:grid-cols-2">
            <label
              v-for="field in [
                { k: 'display_name', l: 'Nome' },
                { k: 'shape', l: 'Shape' },
                { k: 'image_id', l: 'OCID da imagem' },
                { k: 'ssh_authorized_key', l: 'Chave pública SSH' },
              ]"
              :key="field.k"
              class="text-xs text-muted"
              >{{ field.l
              }}<input
                v-model="(launchForm as any)[field.k]"
                required
                class="mt-1 w-full rounded-lg border border-line bg-slate-950 p-2 text-slate-200" /></label
            ><label class="text-xs text-muted"
              >Compartimento<select
                v-model="launchForm.compartment_id"
                required
                class="mt-1 w-full rounded-lg border border-line bg-slate-950 p-2"
              >
                <option value="" disabled>Selecione</option>
                <option
                  v-for="item in overview.data.value?.compartments"
                  :key="item.id"
                  :value="item.id"
                >
                  {{ item.name }}
                </option>
              </select></label
            ><label class="text-xs text-muted"
              >Região<select
                v-model="launchForm.region"
                required
                class="mt-1 w-full rounded-lg border border-line bg-slate-950 p-2"
              >
                <option value="" disabled>Selecione</option>
                <option v-for="item in overview.data.value?.regions" :key="item.name" :value="item.name">{{ item.name }}{{ item.home ? " · principal" : "" }}</option>
              </select></label
            ><label class="text-xs text-muted"
              >Domínio de disponibilidade<select
                v-model="launchForm.availability_domain"
                required
                class="mt-1 w-full rounded-lg border border-line bg-slate-950 p-2"
              >
                <option value="" disabled>Selecione</option>
                <option
                  v-for="item in overview.data.value?.availability_domains"
                  :key="item.name"
                >
                  {{ item.name }}
                </option>
              </select></label
            ><label class="text-xs text-muted"
              >Sub-rede<select
                v-model="launchForm.subnet_id"
                required
                class="mt-1 w-full rounded-lg border border-line bg-slate-950 p-2"
              >
                <option value="" disabled>Selecione</option>
                <option
                  v-for="item in overview.data.value?.subnets"
                  :key="item.id"
                  :value="item.id"
                >
                  {{ item.display_name }} · {{ item.cidr_block }}
                </option>
              </select></label
            ><label class="text-xs text-muted"
              >OCPUs<input
                v-model.number="launchForm.ocpus"
                type="number"
                min="0"
                step="0.1"
                class="mt-1 w-full rounded-lg border border-line bg-slate-950 p-2" /></label
            ><label class="text-xs text-muted"
              >Memória (GB)<input
                v-model.number="launchForm.memory_gb"
                type="number"
                min="0"
                class="mt-1 w-full rounded-lg border border-line bg-slate-950 p-2"
            /></label>
          </div>
          <label class="flex items-center gap-2 text-xs text-warning"
            ><input
              v-model="launchForm.assign_public_ip"
              type="checkbox"
            />Atribuir IP público à VNIC principal</label
          ><Button type="submit" :disabled="launchMutation.isPending.value"
            >Criar instância</Button
          >
        </form>
        <form
          class="space-y-4 bg-panel p-5"
          @submit.prevent="provisionDatabase"
        >
          <div>
            <h3 class="flex items-center gap-2 text-sm text-white">
              <Database class="h-4 w-4 text-pulse" />Novo Autonomous Database
            </h3>
            <p class="mt-1 text-[10px] leading-4 text-muted">
              Provisiona um banco Serverless. A senha administrativa é enviada
              uma única vez e nunca entra no log de auditoria.
            </p>
          </div>
          <div class="grid gap-3 md:grid-cols-2">
            <label class="text-xs text-muted"
              >Nome de exibição<input
                v-model="databaseForm.display_name"
                required
                class="mt-1 w-full rounded-lg border border-line bg-slate-950 p-2" /></label
            ><label class="text-xs text-muted"
              >Nome do banco<input
                v-model="databaseForm.db_name"
                required
                maxlength="30"
                pattern="[A-Za-z][A-Za-z0-9]*"
                class="mt-1 w-full rounded-lg border border-line bg-slate-950 p-2" /></label
            ><label class="text-xs text-muted"
              >Compartimento<select
                v-model="databaseForm.compartment_id"
                required
                class="mt-1 w-full rounded-lg border border-line bg-slate-950 p-2"
              >
                <option value="" disabled>Selecione</option>
                <option
                  v-for="item in overview.data.value?.compartments"
                  :key="item.id"
                  :value="item.id"
                >
                  {{ item.name }}
                </option>
              </select></label
            ><label class="text-xs text-muted"
              >Região<select
                v-model="databaseForm.region"
                required
                class="mt-1 w-full rounded-lg border border-line bg-slate-950 p-2"
              >
                <option value="" disabled>Selecione</option>
                <option v-for="item in overview.data.value?.regions" :key="item.name" :value="item.name">{{ item.name }}{{ item.home ? " · principal" : "" }}</option>
              </select></label
            ><label class="text-xs text-muted"
              >Carga<select
                v-model="databaseForm.workload"
                class="mt-1 w-full rounded-lg border border-line bg-slate-950 p-2"
              >
                <option>OLTP</option>
                <option>DW</option>
                <option>AJD</option>
                <option>APEX</option>
              </select></label
            ><label class="text-xs text-muted"
              >ECPUs<input
                v-model.number="databaseForm.compute_count"
                type="number"
                min="1"
                class="mt-1 w-full rounded-lg border border-line bg-slate-950 p-2" /></label
            ><label class="text-xs text-muted"
              >Armazenamento (TB)<input
                v-model.number="databaseForm.storage_tb"
                type="number"
                min="1"
                class="mt-1 w-full rounded-lg border border-line bg-slate-950 p-2" /></label
            ><label class="text-xs text-muted md:col-span-2"
              >Senha de administrador (12–30 caracteres)<input
                v-model="databaseForm.admin_password"
                required
                type="password"
                minlength="12"
                maxlength="30"
                autocomplete="new-password"
                class="mt-1 w-full rounded-lg border border-line bg-slate-950 p-2"
            /></label>
          </div>
          <div class="flex flex-wrap gap-4">
            <label class="flex items-center gap-2 text-xs text-muted"
              ><input
                v-model="databaseForm.auto_scaling"
                type="checkbox"
              />Escalonamento automático</label
            ><label class="flex items-center gap-2 text-xs text-muted"
              ><input v-model="databaseForm.free_tier" type="checkbox" />Camada
              gratuita</label
            >
          </div>
          <Button type="submit" :disabled="databaseMutation.isPending.value"
            >Criar Autonomous Database</Button
          >
        </form>
      </div>

      <div v-else-if="tab === 'network'" class="space-y-px bg-line/60">
        <div class="grid gap-px lg:grid-cols-2">
          <article
            v-for="vcn in overview.data.value?.vcns"
            :key="vcn.id"
            class="bg-panel p-5"
          >
            <div class="flex items-center justify-between gap-3">
              <div>
                <div class="flex items-center gap-2">
                  <Network class="h-4 w-4 text-signal" />
                  <p class="text-sm text-slate-200">
                    {{ vcn.display_name || vcn.dns_label || "VCN sem nome" }}
                  </p>
                </div>
                <p class="mt-2 font-mono text-[9px] text-muted">
                  {{ (vcn.cidr_blocks ?? []).join(", ") }} ·
                  {{ compartmentName(vcn.compartment_id) }}
                </p>
              </div>
              <StatusBadge
                :status="
                  vcn.lifecycle_state === 'AVAILABLE' ? 'healthy' : 'warning'
                "
                :label="vcn.lifecycle_state"
              />
            </div>
            <div class="mt-4 space-y-2">
              <div
                v-for="subnet in (overview.data.value?.subnets ?? []).filter(
                  (item) => item.vcn_id === vcn.id,
                )"
                :key="subnet.id"
                class="flex items-center justify-between rounded-lg border border-line/60 p-3"
              >
                <div class="flex items-center gap-2">
                  <Waypoints class="h-3.5 w-3.5 text-muted" />
                  <div>
                    <p class="text-xs text-slate-300">
                      {{ subnet.display_name || "sub-rede sem nome" }}
                    </p>
                    <p class="mt-1 font-mono text-[9px] text-muted">
                      {{ subnet.cidr_block }} ·
                      {{ subnet.availability_domain || "regional" }}
                    </p>
                  </div>
                </div>
                <StatusBadge
                  :status="
                    subnet.lifecycle_state === 'AVAILABLE'
                      ? 'healthy'
                      : 'warning'
                  "
                  :label="
                    subnet.prohibit_public_ip_on_vnic
                      ? 'privada'
                      : 'IP público permitido'
                  "
                />
              </div>
            </div>
          </article>
          <p
            v-if="!overview.data.value?.vcns?.length"
            class="bg-panel p-10 text-center text-sm text-muted lg:col-span-2"
          >
            Nenhuma VCN retornada.
          </p>
        </div>
        <section class="border-t border-line bg-panel p-5">
          <div class="mb-4 flex items-center gap-2">
            <HardDrive class="h-4 w-4 text-signal" />
            <div>
              <h3 class="text-sm text-slate-200">Volumes de bloco</h3>
              <p class="text-[10px] text-muted">
                Volumes ativos encontrados em todos os compartimentos
                acessíveis.
              </p>
            </div>
          </div>
          <div class="grid gap-2 md:grid-cols-2 xl:grid-cols-3">
            <article
              v-for="volume in overview.data.value?.block_volumes"
              :key="volume.id"
              class="rounded-lg border border-line/60 p-3"
            >
              <div class="flex items-center justify-between gap-2">
                <p
                  class="truncate text-xs text-slate-200"
                  :title="volume.display_name"
                >
                  {{ volume.display_name || "volume sem nome" }}
                </p>
                <StatusBadge
                  :status="
                    volume.lifecycle_state === 'AVAILABLE'
                      ? 'healthy'
                      : 'warning'
                  "
                  :label="volume.lifecycle_state"
                />
              </div>
              <p class="mt-2 font-mono text-[10px] text-muted">
                {{ volume.size_gb || "—" }} GB ·
                {{ volume.vpus_per_gb || "padrão" }} VPUs/GB
              </p>
              <p class="mt-1 truncate font-mono text-[9px] text-muted">
                {{ volume.availability_domain }} ·
                {{ compartmentName(volume.compartment_id) }}
              </p>
            </article>
          </div>
          <p
            v-if="!overview.data.value?.block_volumes?.length"
            class="py-5 text-center text-sm text-muted"
          >
            Nenhum volume de bloco ativo retornado.
          </p>
        </section>
      </div>

      <div v-else class="grid gap-px bg-line/60 md:grid-cols-2 xl:grid-cols-3">
        <article
          v-for="region in overview.data.value?.regions"
          :key="region.name"
          class="bg-panel p-5"
        >
          <div class="flex items-center justify-between">
            <div class="flex items-center gap-2">
              <Cloud class="h-4 w-4 text-muted" />
              <p class="text-sm text-slate-200">{{ region.name }}</p>
            </div>
            <StatusBadge
              :status="region.status === 'READY' ? 'healthy' : 'warning'"
              :label="region.home ? 'região principal' : region.status"
            />
          </div>
        </article>
        <article
          v-for="domain in overview.data.value?.availability_domains"
          :key="domain.name"
          class="bg-panel p-5"
        >
          <div class="flex items-center gap-2">
            <Globe2 class="h-4 w-4 text-signal" />
            <div>
              <p class="text-sm text-slate-200">{{ domain.name }}</p>
              <p class="mt-1 text-[10px] text-muted">
                Domínio de disponibilidade
              </p>
            </div>
          </div>
        </article>
      </div>
    </section>
  </div>
</template>
