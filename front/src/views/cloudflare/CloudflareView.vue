<script setup lang="ts">
import { computed, ref } from "vue";
import { useMutation, useQuery, useQueryClient } from "@tanstack/vue-query";
import {
  Activity,
  Cloud,
  ExternalLink,
  Globe2,
  Network,
  Pencil,
  Plus,
  RefreshCw,
  Route,
  Settings,
  ShieldCheck,
  Trash2,
  Waypoints,
} from "lucide-vue-next";
import Button from "@/components/ui/Button.vue";
import StatusBadge from "@/components/ui/StatusBadge.vue";
import {
  getCloudflareOverview,
  getCloudflareRulesets,
  getCloudflareZoneSettings,
  getCloudflareTunnelConfiguration,
  purgeCloudflareCache,
  updateCloudflareZoneSetting,
  createCloudflareDNSRecord,
  createCloudflareTunnel,
  createCloudflarePrivateRoute,
  deleteCloudflareTunnel,
  getCloudflarePrivateRoutes,
  deleteCloudflareDNSRecord,
  updateCloudflareDNSRecord,
  updateCloudflareTunnel,
  updateCloudflareTunnelConfiguration,
  type CloudflareDNSRecord,
  type CloudflareProviderStatus,
  type CloudflareTunnelStatus,
} from "@/lib/api_cloudflare";

type InventoryMode = "tunnels" | "dns" | "administration";

const mode = ref<InventoryMode>("tunnels");
const queryClient = useQueryClient();
const query = useQuery({
  queryKey: ["cloudflare", "overview"],
  queryFn: getCloudflareOverview,
  refetchInterval: 30_000,
  retry: 1,
});

const overview = computed(() => query.data.value);
const generatedAt = computed(() => {
  const value = overview.value?.generated_at;
  return value
    ? new Date(value).toLocaleString("pt-BR")
    : "Aguardando primeira leitura";
});
const healthRatio = computed(() => {
  const total = overview.value?.summary.tunnels ?? 0;
  return total
    ? Math.round(((overview.value?.summary.healthy_tunnels ?? 0) / total) * 100)
    : 0;
});
const principalRecords = computed(() => overview.value?.dns_records ?? []);
const zoneIDs = computed(
  () =>
    overview.value?.targets
      .filter((target) => target.kind === "zone")
      .map((target) => target.id) ?? [],
);
const selectedZone = ref("");
const selectedAccount = ref("");
const tunnelName = ref("");
const accountIDs = computed(() => overview.value?.targets.filter((target) => target.kind === "account").map((target) => target.id) ?? []);
const selectedAccountID = computed(() => selectedAccount.value || accountIDs.value[0] || "");
const privateNetwork=ref(''); const privateTunnel=ref('');
const privateRoutes=useQuery({queryKey:['cloudflare','private-routes',selectedAccountID],queryFn:()=>getCloudflarePrivateRoutes(selectedAccountID.value),enabled:computed(()=>!!selectedAccountID.value)});
const configuredTunnel = ref<{accountID:string;id:string;name:string}|null>(null);
const tunnelIngress = ref('[]');
const tunnelConfiguration = useQuery({
  queryKey:['cloudflare','tunnel-configuration',configuredTunnel],
  queryFn:()=>getCloudflareTunnelConfiguration(configuredTunnel.value!.accountID,configuredTunnel.value!.id),
  enabled:computed(()=>!!configuredTunnel.value),
});
const tunnelConfigurationMutation=useMutation({
  mutationFn:()=>{
    const ingress=JSON.parse(tunnelIngress.value);
    if(!Array.isArray(ingress)) throw new Error('Ingress deve ser uma lista JSON.');
    return updateCloudflareTunnelConfiguration(configuredTunnel.value!.accountID,configuredTunnel.value!.id,ingress);
  },
  onSuccess:(value)=>{tunnelIngress.value=JSON.stringify(value.config.ingress,null,2);tunnelConfiguration.refetch()},
});
const selectedZoneID = computed(
  () => selectedZone.value || zoneIDs.value[0] || "",
);
const settingsQuery = useQuery({
  queryKey: ["cloudflare", "settings", selectedZoneID],
  queryFn: () => getCloudflareZoneSettings(selectedZoneID.value),
  enabled: computed(() => !!selectedZoneID.value),
});
const rulesetsQuery = useQuery({
  queryKey: ["cloudflare", "rulesets", selectedZoneID],
  queryFn: () => getCloudflareRulesets(selectedZoneID.value),
  enabled: computed(() => !!selectedZoneID.value),
});
const zoneMutation = useMutation({
  mutationFn: (input: {
    action: "setting" | "purge";
    setting?: string;
    value?: unknown;
  }) => input.action === "purge"
    ? purgeCloudflareCache(selectedZoneID.value).then(() => undefined)
    : updateCloudflareZoneSetting(
        selectedZoneID.value,
        input.setting!,
        input.value,
      ).then(() => undefined),
  onSuccess: () => settingsQuery.refetch(),
});
const dnsType = ref("A");
const dnsName = ref("");
const dnsContent = ref("");
const dnsProxied = ref(true);
const dnsMutation = useMutation({
  mutationFn: async (input: {
    operation: "create" | "update" | "delete";
    zoneID: string;
    record?: CloudflareDNSRecord;
    content?: string;
  }) => {
    if (input.operation === "delete" && input.record)
      return deleteCloudflareDNSRecord(input.zoneID, input.record.id);
    if (input.operation === "update" && input.record)
      return updateCloudflareDNSRecord(input.zoneID, input.record.id, {
        type: input.record.type,
        name: input.record.name,
        content: input.content ?? input.record.content,
        proxied: input.record.proxied,
        ttl: input.record.ttl,
        comment: input.record.comment,
      });
    return createCloudflareDNSRecord(input.zoneID, {
      type: dnsType.value,
      name: dnsName.value,
      content: dnsContent.value,
      proxied: dnsProxied.value,
      ttl: 1,
    });
  },
  onSuccess: () => {
    dnsName.value = "";
    dnsContent.value = "";
    queryClient.invalidateQueries({ queryKey: ["cloudflare", "overview"] });
  },
});
const tunnelMutation = useMutation({
  mutationFn: (input:{operation:'create'|'update'|'delete';id?:string;name?:string}) => {
    const accountID=selectedAccountID.value;
    if(input.operation==='delete') return deleteCloudflareTunnel(accountID,input.id!).then(() => undefined);
    if(input.operation==='update') return updateCloudflareTunnel(accountID,input.id!,input.name!).then(() => undefined);
    return createCloudflareTunnel(accountID,input.name!).then(() => undefined);
  },
  onSuccess:()=>{tunnelName.value='';queryClient.invalidateQueries({queryKey:['cloudflare','overview']})},
});
const privateRouteMutation=useMutation({mutationFn:()=>createCloudflarePrivateRoute(selectedAccountID.value,{network:privateNetwork.value,tunnel_id:privateTunnel.value}),onSuccess:()=>{privateNetwork.value='';privateRoutes.refetch()}})
const createPrivateRoute=()=>{if(selectedAccountID.value&&privateNetwork.value&&privateTunnel.value)privateRouteMutation.mutate()}
const createTunnel=()=>{if(selectedAccountID.value&&tunnelName.value)tunnelMutation.mutate({operation:'create',name:tunnelName.value})}
const renameTunnel=(id:string,name:string)=>{const updated=window.prompt('Novo nome do tunnel',name);if(updated&&updated!==name)tunnelMutation.mutate({operation:'update',id,name:updated})}
const removeTunnel=(id:string,name:string)=>{if(window.prompt(`Excluir o tunnel ${name}. Digite EXCLUIR:`)==='EXCLUIR')tunnelMutation.mutate({operation:'delete',id})}
const configureTunnel=(accountID:string,id:string,name:string)=>{configuredTunnel.value={accountID,id,name};tunnelIngress.value='[]';}
const loadTunnelConfiguration=()=>{if(tunnelConfiguration.data.value)tunnelIngress.value=JSON.stringify(tunnelConfiguration.data.value.config.ingress,null,2)}
const saveTunnelConfiguration=()=>{if(window.prompt(`Atualizar ingress de ${configuredTunnel.value?.name}. Digite ATUALIZAR:`)==='ATUALIZAR')tunnelConfigurationMutation.mutate()}
const zoneFor = (record?: CloudflareDNSRecord) =>
  record?.zone_id || selectedZone.value || zoneIDs.value[0] || "";
const createRecord = () => {
  const zoneID = zoneFor();
  if (zoneID && dnsName.value && dnsContent.value)
    dnsMutation.mutate({ operation: "create", zoneID });
};
const editRecord = (record: CloudflareDNSRecord) => {
  const content = window.prompt(
    `Novo conteúdo para ${record.name}`,
    record.content,
  );
  if (content !== null && content !== record.content)
    dnsMutation.mutate({
      operation: "update",
      zoneID: zoneFor(record),
      record,
      content,
    });
};
const removeRecord = (record: CloudflareDNSRecord) => {
  if (window.confirm(`Excluir definitivamente ${record.type} ${record.name}?`))
    dnsMutation.mutate({
      operation: "delete",
      zoneID: zoneFor(record),
      record,
    });
};
const changeSetting = (setting: string, value: unknown) => {
  if (
    window.confirm(
      `Alterar ${setting} para ${String(value)} na zona selecionada?`,
    )
  )
    zoneMutation.mutate({ action: "setting", setting, value });
};
const promptSetting = (setting: string, current: unknown) => {
  const value = window.prompt("Novo valor", String(current));
  if (value !== null) changeSetting(setting, value);
};
const purgeCache = () => {
  const phrase = "PURGAR CACHE";
  if (
    window.prompt(
      `Limpa todo o cache da zona. Digite exatamente: ${phrase}`,
    ) === phrase
  )
    zoneMutation.mutate({ action: "purge" });
};

const providerTone = (status?: CloudflareProviderStatus) =>
  status === "healthy"
    ? "healthy"
    : status === "degraded"
      ? "warning"
      : "critical";
const tunnelTone = (status: CloudflareTunnelStatus) =>
  status === "healthy"
    ? "healthy"
    : status === "degraded" || status === "inactive"
      ? "warning"
      : status === "down"
        ? "critical"
        : "info";
const recordTone = (record: CloudflareDNSRecord) =>
  record.proxied ? "healthy" : "info";
const shortID = (value: string) =>
  value.length > 18 ? `${value.slice(0, 8)}…${value.slice(-6)}` : value;
</script>

<template>
  <div class="mx-auto max-w-[1680px] space-y-5">
    <header
      class="flex flex-col justify-between gap-5 lg:flex-row lg:items-end"
    >
      <div>
        <div class="flex flex-wrap items-center gap-2">
          <StatusBadge
            :status="providerTone(overview?.status)"
            :label="overview?.status ?? 'connecting'"
          />
          <span
            class="inline-flex items-center gap-1.5 font-mono text-[10px] uppercase tracking-wider text-signal"
          >
            <ShieldCheck class="h-3.5 w-3.5" aria-hidden="true" />
            token completo da API · lista de zonas permitidas
          </span>
        </div>
        <h1 class="mt-4 text-3xl font-semibold tracking-tight text-slate-50">
          Malha de borda Cloudflare
        </h1>
        <p class="mt-2 max-w-3xl text-sm leading-6 text-muted">
          Saúde operacional dos túneis Zero Trust e inventário DNS limitado às
          contas e zonas autorizadas.
        </p>
      </div>
      <div class="flex flex-col items-start gap-2 sm:flex-row sm:items-center">
        <p class="font-mono text-[9px] uppercase tracking-wider text-muted">
          Captura: {{ generatedAt }}
        </p>
        <Button
          variant="outline"
          :disabled="query.isFetching.value"
          aria-label="Atualizar inventário Cloudflare"
          @click="query.refetch()"
        >
          <RefreshCw
            :class="['h-4 w-4', query.isFetching.value && 'animate-spin']"
            aria-hidden="true"
          />
          Atualizar
        </Button>
      </div>
    </header>

    <div
      v-if="query.isLoading.value"
      class="grid min-h-72 place-items-center rounded-xl border border-line bg-panel/50"
      role="status"
      aria-live="polite"
    >
      <div class="text-center">
        <div
          class="mx-auto h-8 w-8 animate-spin rounded-full border-2 border-line border-t-signal"
        />
        <p class="mt-4 text-sm text-muted">
          Consultando o ambiente de borda autorizado…
        </p>
      </div>
    </div>

    <section
      v-else-if="query.isError.value"
      class="rounded-xl border border-danger/25 bg-danger/[.05] p-6"
      role="alert"
    >
      <div class="flex items-start gap-4">
        <div
          class="grid h-10 w-10 shrink-0 place-items-center rounded-xl border border-danger/25 bg-danger/[.08]"
        >
          <Activity class="h-5 w-5 text-danger" aria-hidden="true" />
        </div>
        <div class="min-w-0 flex-1">
          <h2 class="text-sm font-medium text-slate-100">
            Inventário Cloudflare indisponível
          </h2>
          <p class="mt-2 text-xs leading-5 text-muted">
            Verifique o token com escopo de leitura, a envelope key e as
            allowlists configuradas no backend.
          </p>
          <Button class="mt-4" variant="outline" @click="query.refetch()"
            >Tentar novamente</Button
          >
        </div>
      </div>
    </section>

    <template v-else-if="overview">
      <div
        v-if="dnsMutation.isError.value"
        class="rounded-xl border border-danger/20 bg-danger/5 p-4 text-sm text-danger"
      >
        A Cloudflare rejeitou a alteração DNS. Verifique tipo, conteúdo e
        permissões do token.
      </div>
      <section
        class="grid gap-3 sm:grid-cols-2 xl:grid-cols-4"
        aria-label="Resumo Cloudflare"
      >
        <article
          class="rounded-xl border border-line bg-panel/65 p-5 shadow-panel"
        >
          <Waypoints class="h-4 w-4 text-muted" aria-hidden="true" />
          <div class="mt-5 flex items-end justify-between gap-3">
            <p class="font-mono text-2xl text-white">
              {{ overview.summary.tunnels }}
            </p>
            <span class="font-mono text-[10px] text-signal"
              >{{ healthRatio }}% saudáveis</span
            >
          </div>
          <p class="mt-1 text-xs text-muted">Túneis Zero Trust</p>
        </article>
        <article
          class="rounded-xl border border-line bg-panel/65 p-5 shadow-panel"
        >
          <Globe2 class="h-4 w-4 text-muted" aria-hidden="true" />
          <p class="mt-5 font-mono text-2xl text-white">
            {{ overview.summary.dns_records }}
          </p>
          <p class="mt-1 text-xs text-muted">Registros DNS</p>
        </article>
        <article
          class="rounded-xl border border-line bg-panel/65 p-5 shadow-panel"
        >
          <Cloud class="h-4 w-4 text-muted" aria-hidden="true" />
          <p class="mt-5 font-mono text-2xl text-white">
            {{ overview.summary.proxied_records }}
          </p>
          <p class="mt-1 text-xs text-muted">Com proxy Cloudflare</p>
        </article>
        <article
          class="rounded-xl border border-line bg-panel/65 p-5 shadow-panel"
        >
          <Network class="h-4 w-4 text-muted" aria-hidden="true" />
          <p class="mt-5 font-mono text-2xl text-white">
            {{ overview.summary.accounts
            }}<span class="px-1 text-sm text-muted">/</span
            >{{ overview.summary.zones }}
          </p>
          <p class="mt-1 text-xs text-muted">Contas / zonas permitidas</p>
        </article>
      </section>

      <section class="grid gap-5 xl:grid-cols-[minmax(0,1fr)_340px]">
        <article
          class="min-w-0 overflow-hidden rounded-xl border border-line bg-panel/65"
        >
          <header
            class="flex flex-col gap-4 border-b border-line px-5 py-4 sm:flex-row sm:items-center sm:justify-between"
          >
            <div>
              <h2 class="text-sm font-medium text-slate-100">
                Inventário de borda
              </h2>
              <p class="mt-1 text-xs text-muted">
                Estado retornado diretamente pela API Cloudflare
              </p>
            </div>
            <div
              class="flex rounded-lg border border-line bg-slate-950/50 p-1"
              role="tablist"
              aria-label="Tipo de inventário"
            >
              <button
                v-for="item in [
                  { id: 'tunnels', label: 'Túneis' },
                  { id: 'dns', label: 'DNS' },
                  { id: 'administration', label: 'Administração' },
                ]"
                :key="item.id"
                type="button"
                role="tab"
                :aria-selected="mode === item.id"
                :class="[
                  'cursor-pointer rounded-md px-3 py-1.5 text-xs transition-colors duration-200 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-signal',
                  mode === item.id
                    ? 'bg-slate-800 text-slate-100'
                    : 'text-muted hover:text-slate-200',
                ]"
                @click="mode = item.id as InventoryMode"
              >
                {{ item.label }}
              </button>
            </div>
          </header>

          <div
            v-if="mode === 'tunnels'"
            class="divide-y divide-line/60"
            role="tabpanel"
          >
            <form class="grid gap-3 border-b border-line bg-slate-950/25 p-4 md:grid-cols-[180px_1fr_140px]" @submit.prevent="createTunnel">
              <select v-model="selectedAccount" class="rounded-lg border border-line bg-slate-950 px-3 text-xs text-slate-200"><option value="">Conta padrão</option><option v-for="account in accountIDs" :key="account" :value="account">{{shortID(account)}}</option></select>
              <input v-model.trim="tunnelName" required maxlength="100" placeholder="Nome do novo Tunnel" class="rounded-lg border border-line bg-slate-950 px-3 py-2 text-xs text-slate-200"/>
              <Button type="submit" :disabled="!selectedAccountID||tunnelMutation.isPending.value"><Plus class="h-4 w-4"/>Criar Tunnel</Button>
            </form>
            <form class="grid gap-3 border-b border-line bg-slate-950/15 p-4 md:grid-cols-[180px_1fr_1fr_140px]" @submit.prevent="createPrivateRoute"><p class="self-center font-mono text-[10px] uppercase text-muted">Rota privada</p><input v-model.trim="privateNetwork" required placeholder="10.10.50.0/24" class="rounded-lg border border-line bg-slate-950 px-3 py-2 text-xs text-slate-200"/><select v-model="privateTunnel" required class="rounded-lg border border-line bg-slate-950 px-3 text-xs text-slate-200"><option value="">Selecione o Tunnel</option><option v-for="tunnel in overview.tunnels" :key="tunnel.id" :value="tunnel.id">{{tunnel.name}}</option></select><Button type="submit" variant="outline" :disabled="privateRouteMutation.isPending.value"><Route class="h-4 w-4"/>Adicionar rota</Button><p v-if="privateRoutes.data.value?.length" class="md:col-span-4 font-mono text-[10px] text-muted">{{privateRoutes.data.value.map(route=>route.network).join(' · ')}}</p></form>
            <section v-if="configuredTunnel" class="border-b border-line bg-slate-950/25 p-4">
              <div class="flex flex-wrap items-center justify-between gap-3"><div><p class="text-sm text-slate-100">Ingress do tunnel · {{ configuredTunnel.name }}</p><p class="mt-1 font-mono text-[10px] text-muted">A última regra precisa ser o catch-all http_status exigido pela Cloudflare.</p></div><div class="flex gap-2"><Button variant="outline" :disabled="tunnelConfiguration.isFetching.value" @click="tunnelConfiguration.refetch().then(loadTunnelConfiguration)"><RefreshCw class="h-3.5 w-3.5"/>Ler</Button><Button variant="outline" @click="configuredTunnel=null">Fechar</Button></div></div>
              <p v-if="tunnelConfiguration.isError.value || tunnelConfigurationMutation.isError.value" class="mt-3 text-xs text-danger">A configuração ingress foi rejeitada. Revise a sintaxe JSON e a regra final de fallback.</p>
              <textarea v-model="tunnelIngress" class="mt-3 h-48 w-full rounded-lg border border-line bg-slate-950 p-3 font-mono text-xs text-slate-200" spellcheck="false" aria-label="Regras ingress JSON"/>
              <Button class="mt-3" :disabled="tunnelConfigurationMutation.isPending.value" @click="saveTunnelConfiguration"><Settings class="h-4 w-4"/>Salvar ingress</Button>
            </section>
            <article
              v-for="tunnel in overview.tunnels"
              :key="tunnel.id"
              class="grid gap-4 px-5 py-4 transition-colors duration-200 hover:bg-white/[.018] md:grid-cols-[minmax(0,1fr)_150px_140px] md:items-center"
            >
              <div class="flex min-w-0 items-start gap-3">
                <div
                  class="grid h-9 w-9 shrink-0 place-items-center rounded-lg border border-line bg-slate-950/60"
                >
                  <Route class="h-4 w-4 text-pulse" aria-hidden="true" />
                </div>
                <div class="min-w-0">
                  <p class="truncate text-sm font-medium text-slate-200">
                    {{ tunnel.name }}
                  </p>
                  <p
                    class="mt-1 font-mono text-[9px] uppercase tracking-wider text-muted"
                    :title="tunnel.id"
                  >
                    {{ shortID(tunnel.id) }} ·
                    {{ tunnel.connections.length }} connector{{
                      tunnel.connections.length === 1 ? "" : "s"
                    }}
                  </p>
                </div>
              </div>
              <StatusBadge
                :status="tunnelTone(tunnel.status)"
                :label="tunnel.status"
              />
              <p class="font-mono text-[10px] text-muted md:text-right">
                {{ tunnel.connections[0]?.colocation || "NO ACTIVE COLO" }}
              </p>
              <div class="flex justify-end gap-1 md:col-span-3"><button class="rounded p-2 text-muted hover:bg-white/5 hover:text-white" title="Ingress" @click="configureTunnel(tunnel.account_id,tunnel.id,tunnel.name)"><Settings class="h-3.5 w-3.5"/></button><button class="rounded p-2 text-muted hover:bg-white/5 hover:text-white" title="Renomear" @click="renameTunnel(tunnel.id,tunnel.name)"><Pencil class="h-3.5 w-3.5"/></button><button class="rounded p-2 text-danger hover:bg-danger/10" title="Excluir" @click="removeTunnel(tunnel.id,tunnel.name)"><Trash2 class="h-3.5 w-3.5"/></button></div>
            </article>
            <div
              v-if="!overview.tunnels.length"
              class="grid min-h-56 place-items-center px-5 text-center"
            >
              <div>
                <Waypoints
                  class="mx-auto h-7 w-7 text-muted"
                  aria-hidden="true"
                />
                <p class="mt-3 text-sm text-muted">
                  Nenhum túnel encontrado nas contas autorizadas.
                </p>
              </div>
            </div>
          </div>

          <div v-else-if="mode === 'dns'" role="tabpanel">
            <form
              class="grid gap-3 border-b border-line bg-slate-950/25 p-4 md:grid-cols-[180px_90px_1fr_1fr_110px]"
              @submit.prevent="createRecord"
            >
              <select
                v-model="selectedZone"
                class="rounded-lg border border-line bg-slate-950 px-3 text-xs text-slate-200"
              >
                <option value="">Zona padrão</option>
                <option v-for="zone in zoneIDs" :key="zone" :value="zone">
                  {{ shortID(zone) }}
                </option>
              </select>
              <select
                v-model="dnsType"
                class="rounded-lg border border-line bg-slate-950 px-3 text-xs text-slate-200"
              >
                <option
                  v-for="type in ['A', 'AAAA', 'CNAME', 'TXT', 'MX', 'CAA']"
                  :key="type"
                >
                  {{ type }}
                </option>
              </select>
              <input
                v-model.trim="dnsName"
                required
                maxlength="255"
                placeholder="Nome completo"
                class="rounded-lg border border-line bg-slate-950 px-3 py-2 text-xs text-slate-200"
              />
              <input
                v-model.trim="dnsContent"
                required
                maxlength="4096"
                placeholder="Conteúdo / destino"
                class="rounded-lg border border-line bg-slate-950 px-3 py-2 text-xs text-slate-200"
              />
              <Button
                type="submit"
                :disabled="dnsMutation.isPending.value || !zoneFor()"
                ><Plus class="h-3.5 w-3.5" />Criar</Button
              >
            </form>
            <div class="overflow-x-auto">
              <table class="w-full min-w-[760px] text-left">
                <thead
                  class="border-b border-line bg-slate-950/30 font-mono text-[9px] uppercase tracking-widest text-muted"
                >
                  <tr>
                    <th class="px-5 py-3 font-medium">Registro</th>
                    <th class="px-4 py-3 font-medium">Destino</th>
                    <th class="px-4 py-3 font-medium">Proxy</th>
                    <th class="px-5 py-3 text-right font-medium">TTL</th>
                    <th class="px-5 py-3 text-right font-medium">Ações</th>
                  </tr>
                </thead>
                <tbody class="divide-y divide-line/60">
                  <tr
                    v-for="record in principalRecords"
                    :key="record.id"
                    class="transition-colors duration-200 hover:bg-white/[.018]"
                  >
                    <td class="px-5 py-4">
                      <div class="flex min-w-0 items-center gap-3">
                        <span
                          class="inline-flex w-14 shrink-0 justify-center rounded-md border border-line bg-slate-950/50 px-2 py-1 font-mono text-[10px] text-pulse"
                        >
                          {{ record.type }}
                        </span>
                        <span
                          class="truncate text-xs text-slate-200"
                          :title="record.name"
                          >{{ record.name }}</span
                        >
                      </div>
                    </td>
                    <td
                      class="max-w-sm truncate px-4 py-4 font-mono text-[10px] text-muted"
                      :title="record.content"
                    >
                      {{ record.content }}
                    </td>
                    <td class="px-4 py-4">
                      <StatusBadge
                        :status="recordTone(record)"
                        :label="record.proxied ? 'proxied' : 'DNS only'"
                      />
                    </td>
                    <td
                      class="px-5 py-4 text-right font-mono text-[10px] text-muted"
                    >
                      {{ record.ttl === 1 ? "AUTO" : `${record.ttl}s` }}
                    </td>
                    <td class="px-5 py-4">
                      <div class="flex justify-end gap-1">
                        <button
                          class="grid h-8 w-8 place-items-center rounded-lg text-muted hover:bg-white/5 hover:text-white"
                          title="Editar conteúdo"
                          @click="editRecord(record)"
                        >
                          <Pencil class="h-3.5 w-3.5" /></button
                        ><button
                          class="grid h-8 w-8 place-items-center rounded-lg text-danger hover:bg-danger/10"
                          title="Excluir registro"
                          @click="removeRecord(record)"
                        >
                          <Trash2 class="h-3.5 w-3.5" />
                        </button>
                      </div>
                    </td>
                  </tr>
                </tbody>
              </table>
            </div>
            <div
              v-if="!principalRecords.length"
              class="grid min-h-56 place-items-center px-5 text-center"
            >
              <div>
                <Globe2 class="mx-auto h-7 w-7 text-muted" aria-hidden="true" />
                <p class="mt-3 text-sm text-muted">
                  Nenhum apontamento DNS principal foi encontrado.
                </p>
              </div>
            </div>
          </div>
          <div v-else class="space-y-px bg-line/60" role="tabpanel">
            <header class="flex flex-wrap items-end gap-3 bg-panel p-5">
              <label class="min-w-72 flex-1 text-xs text-muted"
                >Zona<select
                  v-model="selectedZone"
                  class="mt-2 w-full rounded-lg border border-line bg-slate-950 p-2"
                >
                  <option value="">Zona padrão</option>
                  <option v-for="zone in zoneIDs" :key="zone" :value="zone">
                    {{ shortID(zone) }}
                  </option>
                </select></label
              ><Button
                variant="danger"
                :disabled="!selectedZoneID || zoneMutation.isPending.value"
                @click="purgeCache"
                ><Trash2 class="h-4 w-4" />Purgar todo o cache</Button
              >
            </header>
            <div class="grid gap-px bg-line/60 lg:grid-cols-2">
              <section class="bg-panel p-5">
                <h3 class="flex items-center gap-2 text-sm text-white">
                  <Settings class="h-4 w-4 text-signal" />Configurações
                  editáveis da zona
                </h3>
                <p class="mt-1 text-[10px] text-muted">
                  Atalhos para HTTPS, modo de desenvolvimento e nível de
                  segurança.
                </p>
                <div class="mt-4 space-y-3">
                  <div
                    v-for="setting in settingsQuery.data.value?.filter((item) =>
                      [
                        'always_use_https',
                        'development_mode',
                        'security_level',
                        'browser_cache_ttl',
                        'ssl',
                      ].includes(item.id),
                    )"
                    :key="setting.id"
                    class="flex items-center justify-between gap-3 rounded-lg border border-line p-3"
                  >
                    <div>
                      <p class="font-mono text-[10px] text-slate-200">
                        {{ setting.id }}
                      </p>
                      <p class="mt-1 text-[10px] text-muted">
                        Valor atual: {{ String(setting.value) }} ·
                        {{ setting.editable ? "editável" : "somente leitura" }}
                      </p>
                    </div>
                    <div v-if="setting.editable" class="flex gap-1">
                      <Button
                        v-if="
                          ['always_use_https', 'development_mode'].includes(
                            setting.id,
                          )
                        "
                        size="sm"
                        variant="outline"
                        @click="
                          changeSetting(
                            setting.id,
                            setting.value === 'on' ? 'off' : 'on',
                          )
                        "
                        >Alternar</Button
                      ><Button
                        v-else
                        size="sm"
                        variant="outline"
                        @click="promptSetting(setting.id, setting.value)"
                        >Alterar</Button
                      >
                    </div>
                  </div>
                </div>
              </section>
              <section class="bg-panel p-5">
                <h3 class="text-sm text-white">Rulesets ativos</h3>
                <p class="mt-1 text-[10px] text-muted">
                  Inventário das fases de firewall, transformação, cache e
                  redirecionamento.
                </p>
                <div class="mt-4 space-y-3">
                  <article
                    v-for="ruleset in rulesetsQuery.data.value"
                    :key="ruleset.id"
                    class="rounded-lg border border-line p-3"
                  >
                    <div class="flex items-center justify-between gap-3">
                      <p class="text-xs text-slate-200">{{ ruleset.name }}</p>
                      <StatusBadge status="info" :label="ruleset.kind" />
                    </div>
                    <p class="mt-2 font-mono text-[9px] text-muted">
                      {{ ruleset.phase }} · versão {{ ruleset.version }}
                    </p>
                    <p
                      v-if="ruleset.description"
                      class="mt-2 text-[10px] text-muted"
                    >
                      {{ ruleset.description }}
                    </p>
                  </article>
                  <p
                    v-if="!rulesetsQuery.data.value?.length"
                    class="py-8 text-center text-xs text-muted"
                  >
                    Nenhum ruleset retornado.
                  </p>
                </div>
              </section>
            </div>
          </div>
        </article>

        <aside class="space-y-4">
          <article class="rounded-xl border border-line bg-panel/55 p-5">
            <div class="flex items-center gap-3">
              <Activity class="h-4 w-4 text-signal" aria-hidden="true" />
              <h2 class="text-sm font-medium text-slate-100">
                Saúde do provedor
              </h2>
            </div>
            <div class="mt-4 space-y-3">
              <div
                v-for="target in overview.targets"
                :key="`${target.kind}-${target.id}`"
                class="rounded-lg border border-line bg-slate-950/35 p-3"
              >
                <div class="flex items-center justify-between gap-3">
                  <p
                    class="font-mono text-[9px] uppercase tracking-wider text-muted"
                  >
                    {{ target.kind }}
                  </p>
                  <StatusBadge
                    :status="
                      target.status === 'healthy' ? 'healthy' : 'critical'
                    "
                    :label="target.status"
                  />
                </div>
                <p
                  class="mt-3 truncate font-mono text-[10px] text-slate-300"
                  :title="target.id"
                >
                  {{ shortID(target.id) }}
                </p>
                <p class="mt-1 text-[11px] leading-5 text-muted">
                  {{ target.message }} {{ target.item_count }} items.
                </p>
              </div>
            </div>
          </article>

          <article
            class="rounded-xl border border-pulse/20 bg-pulse/[.035] p-5"
          >
            <div class="flex items-center gap-3">
              <ExternalLink class="h-4 w-4 text-pulse" aria-hidden="true" />
              <h2 class="text-sm font-medium text-slate-100">
                Limite de segurança
              </h2>
            </div>
            <p class="mt-3 text-xs leading-5 text-muted">
              O token administrativo é decriptado somente durante cada chamada.
              Toda conta ou zona fora da lista de permissões continua recusada
              antes da API externa, inclusive nas alterações.
            </p>
          </article>
        </aside>
      </section>
    </template>
  </div>
</template>
