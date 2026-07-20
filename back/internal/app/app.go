package app

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"
	_ "time/tzdata"

	"github.com/jackc/pgx/v5/pgxpool"
	cloudflareadapter "github.com/webcreations/wc-hub/back/internal/adapters/cloudflare"
	dockeradapter "github.com/webcreations/wc-hub/back/internal/adapters/docker"
	githubadapter "github.com/webcreations/wc-hub/back/internal/adapters/github"
	kubernetesadapter "github.com/webcreations/wc-hub/back/internal/adapters/kubernetes"
	mergerfsadapter "github.com/webcreations/wc-hub/back/internal/adapters/mergerfs"
	monitoringadapter "github.com/webcreations/wc-hub/back/internal/adapters/monitoring"
	ociadapter "github.com/webcreations/wc-hub/back/internal/adapters/oci"
	pbsadapter "github.com/webcreations/wc-hub/back/internal/adapters/pbs"
	poweradapter "github.com/webcreations/wc-hub/back/internal/adapters/power"
	proxmoxadapter "github.com/webcreations/wc-hub/back/internal/adapters/proxmox"
	terraformadapter "github.com/webcreations/wc-hub/back/internal/adapters/terraform"
	vncadapter "github.com/webcreations/wc-hub/back/internal/adapters/vnc"
	auditrepo "github.com/webcreations/wc-hub/back/internal/audit/repository"
	authapp "github.com/webcreations/wc-hub/back/internal/auth/application"
	authdomain "github.com/webcreations/wc-hub/back/internal/auth/domain"
	authrepo "github.com/webcreations/wc-hub/back/internal/auth/repository"
	backupapp "github.com/webcreations/wc-hub/back/internal/backupapp"
	cloudflareapp "github.com/webcreations/wc-hub/back/internal/cloudflareapp"
	dockerapp "github.com/webcreations/wc-hub/back/internal/dockerapp"
	githubapp "github.com/webcreations/wc-hub/back/internal/githubapp"
	inventorydomain "github.com/webcreations/wc-hub/back/internal/inventory/domain"
	inventoryrepo "github.com/webcreations/wc-hub/back/internal/inventory/repository"
	jobapp "github.com/webcreations/wc-hub/back/internal/jobs/application"
	jobdomain "github.com/webcreations/wc-hub/back/internal/jobs/domain"
	jobrepo "github.com/webcreations/wc-hub/back/internal/jobs/repository"
	kubernetesapp "github.com/webcreations/wc-hub/back/internal/kubernetesapp"
	monitorapp "github.com/webcreations/wc-hub/back/internal/monitorapp"
	ociapp "github.com/webcreations/wc-hub/back/internal/ociapp"
	operationsapp "github.com/webcreations/wc-hub/back/internal/operationsapp"
	overview "github.com/webcreations/wc-hub/back/internal/overview/application"
	"github.com/webcreations/wc-hub/back/internal/platform/config"
	powerapp "github.com/webcreations/wc-hub/back/internal/powerapp"
	proxmoxapp "github.com/webcreations/wc-hub/back/internal/proxmox/application"
	proxmoxrepo "github.com/webcreations/wc-hub/back/internal/proxmox/repository"
	schedulerrepo "github.com/webcreations/wc-hub/back/internal/scheduler/repository"
	security "github.com/webcreations/wc-hub/back/internal/security/domain"
	storageapp "github.com/webcreations/wc-hub/back/internal/storageapp"
	telemetryapp "github.com/webcreations/wc-hub/back/internal/telemetry/application"
	telemetryrepo "github.com/webcreations/wc-hub/back/internal/telemetry/repository"
	terminalapp "github.com/webcreations/wc-hub/back/internal/terminal/application"
	terminalrepo "github.com/webcreations/wc-hub/back/internal/terminal/repository"
	terraformapp "github.com/webcreations/wc-hub/back/internal/terraformapp"
	vncapp "github.com/webcreations/wc-hub/back/internal/vncapp"
)

type contextKey string

const sessionContextKey contextKey = "session"
const sessionCookie = "wc_hub_session"

type App struct {
	cfg                       config.Config
	logger                    *slog.Logger
	overview                  *overview.Service
	policy                    *security.Engine
	db                        *pgxpool.Pool
	auth                      *authapp.Service
	audit                     *auditrepo.Postgres
	inventory                 inventorydomain.Repository
	jobs                      *jobrepo.Postgres
	proxmox                   *proxmoxrepo.Postgres
	proxmoxClient             *proxmoxadapter.Client
	proxmoxClients            []*proxmoxadapter.Client
	dockerClient              *dockeradapter.Client
	kubernetesClient          *kubernetesadapter.Client
	cloudflareHandler         *cloudflareapp.Handler
	githubClient              *githubadapter.Client
	terraformRunner           *terraformadapter.Runner
	storageClient             *mergerfsadapter.Client
	ociHandler                *ociapp.Handler
	telemetry                 *telemetryrepo.Postgres
	terminal                  *terminalrepo.Postgres
	terminalGateway           *terminalapp.Gateway
	vncGateway                *vncadapter.Gateway
	pbsClient                 *pbsadapter.Client
	monitorStore              *monitorapp.Store
	powerClient               *poweradapter.Client
	adapterErrors             map[string]string
	developmentMasterLocation *time.Location
	loginLimiter              *loginLimiter
	cancelWorkers             context.CancelFunc
}

func New(ctx context.Context, cfg config.Config, logger *slog.Logger) (*App, func(), error) {
	if cfg.DatabaseURL == "" {
		return nil, nil, fmt.Errorf("WC_HUB_DATABASE_URL is required")
	}
	var developmentMasterLocation *time.Location
	if cfg.DevelopmentMasterLogin {
		if !developmentMasterAllowed(cfg.Environment) {
			return nil, nil, fmt.Errorf("WC_HUB_DEV_MASTER_LOGIN is forbidden outside development, local, or test environments")
		}
		location, locationErr := time.LoadLocation(cfg.DevelopmentMasterTimezone)
		if locationErr != nil {
			return nil, nil, fmt.Errorf("load development master timezone: %w", locationErr)
		}
		developmentMasterLocation = location
	}
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return nil, nil, err
	}
	if err = pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, nil, err
	}
	application := &App{cfg: cfg, logger: logger, overview: overview.New(pool, cfg.Environment, cfg.SelfProtected), policy: security.NewEngine(cfg.LocalAllowlist), db: pool, loginLimiter: newLoginLimiter(), adapterErrors: map[string]string{}}
	authRepository := authrepo.NewPostgres(pool)
	application.auth = authapp.New(authRepository, cfg.SessionTTL)
	if cfg.DevelopmentMasterLogin {
		if _, masterErr := authRepository.EnsureDevelopmentMaster(ctx); masterErr != nil {
			pool.Close()
			return nil, nil, fmt.Errorf("ensure development master: %w", masterErr)
		}
		application.developmentMasterLocation = developmentMasterLocation
		logger.Warn("development-only hourly master login enabled", "username", authdomain.DevelopmentMasterUsername, "timezone", developmentMasterLocation.String())
	}
	if err := application.auth.ConfigureTOTP(cfg.EncryptionKey, cfg.TOTPIssuer); err != nil {
		logger.Warn("TOTP enrollment disabled", "error", err)
	}
	application.audit = auditrepo.NewPostgres(pool)
	application.inventory = inventoryrepo.NewPostgres(pool)
	application.jobs = jobrepo.NewPostgres(pool)
	application.proxmox = proxmoxrepo.NewPostgres(pool)
	application.telemetry = telemetryrepo.NewPostgres(pool)
	application.terminal = terminalrepo.NewPostgres(pool)
	application.monitorStore = monitorapp.NewStore(pool)
	if cfg.ProxmoxURL != "" {
		client, clientErr := proxmoxadapter.NewWithConfig(proxmoxadapter.Config{URL: cfg.ProxmoxURL, TokenID: cfg.ProxmoxTokenID, Secret: []byte(cfg.ProxmoxSecret), CAPath: cfg.ProxmoxTLSCA, InsecureSkipVerify: cfg.ProxmoxTLSInsecure})
		if clientErr != nil {
			logger.Error("Proxmox adapter disabled", "error", clientErr)
			application.setAdapterError("proxmox", clientErr)
		} else {
			application.proxmoxClient = client
			application.proxmoxClients = append(application.proxmoxClients, client)
			if _, err = pool.Exec(ctx, `UPDATE schedules SET enabled=true,next_run_at=LEAST(next_run_at,now()) WHERE name='proxmox-inventory-sync'`); err != nil {
				logger.Error("enable Proxmox schedule failed", "error", err)
			}
		}
	} else {
		application.setAdapterError("proxmox", fmt.Errorf("PROXMOX_API_URL is not configured"))
	}
	for _, configPath := range cfg.ProxmoxAdditionalConfigs {
		client, clientErr := proxmoxadapter.NewFromEnvFile(configPath)
		if clientErr != nil {
			logger.Error("additional Proxmox adapter disabled", "config_path", configPath, "error", clientErr)
			application.setAdapterError("proxmox", clientErr)
			continue
		}
		application.proxmoxClients = append(application.proxmoxClients, client)
	}
	if cfg.DockerEndpoint != "" || cfg.DockerFallbackSocket != "" {
		client, clientErr := dockeradapter.NewWithConfig(dockeradapter.Config{Endpoint: cfg.DockerEndpoint, FallbackSocketPath: cfg.DockerFallbackSocket, CACertificatePath: cfg.DockerTLSCA, ClientCertPath: cfg.DockerClientCert, ClientKeyPath: cfg.DockerClientKey})
		if clientErr != nil {
			logger.Error("Docker adapter disabled", "error", clientErr)
			application.setAdapterError("docker", clientErr)
		} else {
			application.dockerClient = client
		}
	} else {
		application.setAdapterError("docker", fmt.Errorf("DOCKER_PROXY_URL and DOCKER_FALLBACK_SOCKET_PATH are not configured"))
	}
	client, clientErr := kubernetesadapter.New(kubernetesadapter.Config{Endpoint: cfg.KubernetesURL, TokenPath: cfg.KubernetesToken, CAPath: cfg.KubernetesCA, KubeconfigPath: cfg.KubernetesKubeconfig})
	if clientErr != nil {
		logger.Error("Kubernetes adapter disabled", "error", clientErr)
		application.setAdapterError("kubernetes", clientErr)
	} else {
		application.kubernetesClient = client
	}
	if cfg.CloudflareToken != "" && (len(cfg.CloudflareAccounts) > 0 || len(cfg.CloudflareZones) > 0) {
		kek, decodeErr := base64.StdEncoding.DecodeString(cfg.EncryptionKey)
		if decodeErr != nil || len(kek) != 32 {
			logger.Error("Cloudflare adapter disabled", "error", "WC_HUB_ENCRYPTION_KEY must be base64-encoded 32 bytes")
		} else {
			envelope, sealErr := cloudflareadapter.SealToken(kek, []byte(cfg.CloudflareToken))
			decryptor, decryptErr := cloudflareadapter.NewAESGCMEnvelopeDecryptor(kek)
			if sealErr != nil || decryptErr != nil {
				logger.Error("Cloudflare adapter disabled", "seal_error", sealErr, "decryptor_error", decryptErr)
			} else {
				source := cloudflareadapter.CredentialSourceFunc(func(context.Context) (cloudflareadapter.TokenEnvelope, error) { return envelope, nil })
				client, clientErr := cloudflareadapter.New(cloudflareadapter.Config{CredentialSource: source, Decryptor: decryptor, AllowedAccounts: cfg.CloudflareAccounts, AllowedZones: cfg.CloudflareZones})
				if clientErr != nil {
					logger.Error("Cloudflare adapter disabled", "error", clientErr)
				} else {
					handler, handlerErr := cloudflareapp.NewHandler(client, cloudflareapp.Config{Audit: func(ctx context.Context, event cloudflareapp.AuditEvent) {
						_ = application.audit.Append(ctx, auditrepo.Record{Action: event.Action, Scope: security.ScopeCloud, ResourceType: event.ResourceType, TargetName: event.TargetName, Risk: security.RiskSafe, Decision: event.Decision, Reason: event.Reason, Payload: event.Payload})
					}})
					if handlerErr != nil {
						logger.Error("Cloudflare handler disabled", "error", handlerErr)
					} else {
						application.cloudflareHandler = handler
					}
				}
			}
		}
	}
	if cfg.GitHubToken != "" && len(cfg.GitHubRepositories) > 0 {
		client, clientErr := githubadapter.New(githubadapter.Config{Token: []byte(cfg.GitHubToken), Repositories: cfg.GitHubRepositories})
		if clientErr != nil {
			logger.Error("GitHub adapter disabled", "error", clientErr)
		} else {
			application.githubClient = client
		}
	}
	if cfg.TerraformWorkerURL != "" && cfg.TerraformWorkerToken != "" {
		runner, runnerErr := terraformadapter.New(terraformadapter.Config{WorkerURL: cfg.TerraformWorkerURL, WorkerToken: []byte(cfg.TerraformWorkerToken), Workspaces: cfg.TerraformWorkspaces})
		if runnerErr != nil {
			logger.Error("Terraform adapter disabled", "error", runnerErr)
		} else {
			application.terraformRunner = runner
		}
	}
	if cfg.MergerFSRoot != "" || (cfg.MergerFSSSHAddress != "" && cfg.MergerFSSSHRoot != "") {
		client, clientErr := mergerfsadapter.NewWithConfig(mergerfsadapter.Config{Root: cfg.MergerFSRoot, SSHAddress: cfg.MergerFSSSHAddress, SSHUser: cfg.MergerFSSSHUser, SSHRoot: cfg.MergerFSSSHRoot, SSHPrivateKeyPath: cfg.SSHPrivateKeyPath, SSHKnownHostsPath: cfg.SSHKnownHostsPath})
		if clientErr != nil {
			logger.Error("MergerFS adapter disabled", "error", clientErr)
		} else {
			application.storageClient = client
		}
	}
	if cfg.OCIConfigPath != "" {
		client, clientErr := ociadapter.New(cfg.OCIConfigPath, cfg.OCIConfigProfile)
		if clientErr != nil {
			logger.Error("OCI adapter disabled", "error", clientErr)
			application.setAdapterError("oci", clientErr)
		} else {
			application.ociHandler = ociapp.NewHandler(client, func(ctx context.Context, event ociapp.AuditEvent) {
				session, _ := ctx.Value(sessionContextKey).(authdomain.Session)
				_ = application.audit.Append(ctx, auditrepo.Record{ActorID: session.User.ID, Action: event.Action, Scope: security.ScopeCloud, ResourceType: "oci_instance", ResourceID: event.ResourceID, TargetName: event.TargetName, Risk: security.RiskCritical, Decision: "allowed", Payload: event.Payload})
			})
		}
	} else {
		application.setAdapterError("oci", fmt.Errorf("OCI_CONFIG_PATH is not configured"))
	}
	if len(cfg.VNCTargets) > 0 {
		gateway, gatewayErr := vncadapter.New(vncadapter.Config{Targets: cfg.VNCTargets})
		if gatewayErr != nil {
			logger.Error("VNC gateway disabled", "error", gatewayErr)
		} else {
			application.vncGateway = gateway
		}
	}
	if cfg.PBSURL != "" && cfg.PBSTokenID != "" && cfg.PBSSecret != "" {
		client, clientErr := pbsadapter.New(pbsadapter.Config{URL: cfg.PBSURL, TokenID: cfg.PBSTokenID, Secret: cfg.PBSSecret, CAPath: cfg.PBSTLSCA})
		if clientErr != nil {
			logger.Error("PBS adapter disabled", "error", clientErr)
		} else {
			application.pbsClient = client
		}
	}
	if cfg.PowerNUTAddress != "" || len(cfg.PowerWOLTargets) > 0 {
		client, clientErr := poweradapter.New(poweradapter.Config{NUTAddress: cfg.PowerNUTAddress, UPSName: cfg.PowerNUTUPSName, WOLTargets: cfg.PowerWOLTargets, Broadcast: cfg.PowerWOLBroadcast})
		if clientErr != nil {
			logger.Error("power adapter disabled", "error", clientErr)
		} else {
			application.powerClient = client
		}
	}
	if gateway, gatewayErr := terminalapp.NewGateway(application.terminal, cfg.SSHPrivateKeyPath, cfg.SSHKnownHostsPath, cfg.PublicURL); gatewayErr != nil {
		logger.Warn("SSH terminal disabled", "error", gatewayErr)
	} else {
		application.terminalGateway = gateway
		gateway.SetAudit(func(ctx context.Context, target terminalrepo.Target, status, reason string) {
			_ = application.audit.Append(ctx, auditrepo.Record{ActorID: target.UserID, Action: "terminal.session." + status, Scope: security.ScopeRemote, ResourceType: "terminal_session", ResourceID: target.SessionID, TargetName: target.HostName, Risk: security.RiskDangerous, Decision: "allowed", Reason: reason})
		})
	}
	workerCtx, cancelWorkers := context.WithCancel(ctx)
	application.cancelWorkers = cancelWorkers
	if cfg.MonitoringEnabled {
		monitoringadapter.New(application.monitorStore, 15*time.Second).Start(workerCtx)
	}
	handlers := []jobdomain.Handler{proxmoxapp.NewSyncHandler(application.proxmoxClient, application.proxmox, cfg.ProxmoxURL), telemetryapp.NewMaintenanceHandler(application.telemetry)}
	runner := jobapp.NewRunner(application.jobs, handlers, logger, cfg.WorkerID, cfg.WorkerCount)
	runner.SetResultHook(func(ctx context.Context, job jobdomain.Job, status string, jobErr error) {
		reason := ""
		if jobErr != nil {
			reason = jobErr.Error()
		}
		_ = application.audit.Append(ctx, auditrepo.Record{Action: "job." + status, Scope: security.ScopeRemote, ResourceType: "job", ResourceID: job.ID, TargetName: job.Kind, Risk: security.RiskDangerous, Decision: map[string]string{"succeeded": "allowed", "failed": "denied"}[status], Reason: reason})
	})
	runner.Start(workerCtx)
	schedulerrepo.NewPostgres(pool, application.jobs).Start(workerCtx)
	return application, func() { cancelWorkers(); pool.Close() }, nil
}

func (a *App) setAdapterError(provider string, err error) {
	if err == nil {
		return
	}
	if previous := a.adapterErrors[provider]; previous != "" {
		a.adapterErrors[provider] = previous + "; " + err.Error()
		return
	}
	a.adapterErrors[provider] = err.Error()
}

func (a *App) adapterError(provider string) string {
	return a.adapterErrors[provider]
}

func (a *App) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", a.health)
	mux.HandleFunc("GET /api/v1/auth/bootstrap-status", a.bootstrapStatus)
	mux.HandleFunc("POST /api/v1/auth/bootstrap", a.bootstrap)
	mux.HandleFunc("POST /api/v1/auth/login", a.login)
	mux.HandleFunc("GET /api/v1/auth/session", a.protect("", a.session))
	mux.HandleFunc("POST /api/v1/auth/logout", a.protect("", a.logout))
	mux.HandleFunc("POST /api/v1/auth/totp/enroll", a.protect("", a.enrollTOTP))
	mux.HandleFunc("POST /api/v1/auth/totp/confirm", a.protect("", a.confirmTOTP))
	mux.HandleFunc("GET /api/v1/overview", a.protect("overview.read", a.getOverview))
	mux.HandleFunc("POST /api/v1/security/evaluate", a.protect("hosts.execute.safe", a.evaluatePolicy))
	mux.HandleFunc("GET /api/v1/modules", a.protect("overview.read", a.modules))
	mux.HandleFunc("GET /api/v1/integrations", a.protect("overview.read", a.listIntegrations))
	mux.HandleFunc("POST /api/v1/integrations", a.protect("hosts.execute.safe", a.createIntegration))
	mux.HandleFunc("GET /api/v1/hosts", a.protect("overview.read", a.listHosts))
	mux.HandleFunc("POST /api/v1/hosts", a.protect("hosts.execute.safe", a.createHost))
	mux.HandleFunc("GET /api/v1/audit", a.protect("audit.read", a.listAudit))
	mux.HandleFunc("GET /api/v1/admin/users", a.protect("", a.adminOnly(a.listAdminUsers)))
	mux.HandleFunc("POST /api/v1/admin/users", a.protect("", a.adminOnly(a.createAdminUser)))
	mux.HandleFunc("PATCH /api/v1/admin/users/{id}", a.protect("", a.adminOnly(a.updateAdminUser)))
	mux.HandleFunc("DELETE /api/v1/admin/users/{id}", a.protect("", a.adminOnly(a.disableAdminUser)))
	mux.HandleFunc("GET /api/v1/admin/roles", a.protect("", a.adminOnly(a.listAdminRoles)))
	mux.HandleFunc("POST /api/v1/admin/roles", a.protect("", a.adminOnly(a.createAdminRole)))
	mux.HandleFunc("PATCH /api/v1/admin/roles/{id}", a.protect("", a.adminOnly(a.updateAdminRole)))
	mux.HandleFunc("DELETE /api/v1/admin/roles/{id}", a.protect("", a.adminOnly(a.deleteAdminRole)))
	mux.HandleFunc("GET /api/v1/admin/permissions", a.protect("", a.adminOnly(a.listAdminPermissions)))
	mux.HandleFunc("GET /api/v1/alerts", a.protect("overview.read", a.listAlerts))
	mux.HandleFunc("PATCH /api/v1/alerts/{id}", a.protect("hosts.execute.safe", a.updateAlert))
	mux.HandleFunc("GET /api/v1/proxmox/summary", a.protect("proxmox.read", a.proxmoxSummary))
	mux.HandleFunc("POST /api/v1/proxmox/sync", a.protect("proxmox.sync", a.proxmoxSync))
	mux.HandleFunc("GET /api/v1/proxmox/inventory", a.protect("proxmox.read", a.proxmoxInventory))
	mux.HandleFunc("POST /api/v1/proxmox/nodes/{node}/{kind}/{vmid}/{action}", a.protect("proxmox.manage", a.proxmoxPowerAction))
	mux.HandleFunc("POST /api/v1/proxmox/qemu", a.protect("proxmox.manage", a.proxmoxCreateQEMU))
	mux.HandleFunc("POST /api/v1/proxmox/lxc", a.protect("proxmox.manage", a.proxmoxCreateLXC))
	mux.HandleFunc("POST /api/v1/proxmox/clone", a.protect("proxmox.manage", a.proxmoxClone))
	mux.HandleFunc("DELETE /api/v1/proxmox/nodes/{node}/{kind}/{vmid}", a.protect("proxmox.manage", a.proxmoxDeleteGuest))
	mux.HandleFunc("GET /api/v1/proxmox/nodes/{node}/{kind}/{vmid}/snapshots", a.protect("proxmox.read", a.proxmoxSnapshots))
	mux.HandleFunc("POST /api/v1/proxmox/nodes/{node}/{kind}/{vmid}/snapshots", a.protect("proxmox.manage", a.proxmoxCreateSnapshot))
	mux.HandleFunc("POST /api/v1/proxmox/nodes/{node}/{kind}/{vmid}/snapshots/{name}/rollback", a.protect("proxmox.manage", a.proxmoxRollbackSnapshot))
	mux.HandleFunc("DELETE /api/v1/proxmox/nodes/{node}/{kind}/{vmid}/snapshots/{name}", a.protect("proxmox.manage", a.proxmoxDeleteSnapshot))
	mux.HandleFunc("POST /api/v1/proxmox/nodes/{node}/{kind}/{vmid}/migrate", a.protect("proxmox.manage", a.proxmoxMigrateGuest))
	mux.HandleFunc("PUT /api/v1/proxmox/nodes/{node}/{kind}/{vmid}/resize", a.protect("proxmox.manage", a.proxmoxResizeDisk))
	mux.HandleFunc("PUT /api/v1/proxmox/nodes/{node}/{kind}/{vmid}/config", a.protect("proxmox.manage", a.proxmoxUpdateConfig))
	mux.HandleFunc("GET /api/v1/proxmox/nodes/{node}/network", a.protect("proxmox.read", a.proxmoxNodeNetwork))
	mux.HandleFunc("GET /api/v1/proxmox/nodes/{node}/{kind}/{vmid}/firewall/rules", a.protect("proxmox.read", a.proxmoxGuestFirewallRules))
	mux.HandleFunc("POST /api/v1/proxmox/nodes/{node}/{kind}/{vmid}/firewall/rules", a.protect("proxmox.manage", a.proxmoxCreateFirewallRule))
	mux.HandleFunc("DELETE /api/v1/proxmox/nodes/{node}/{kind}/{vmid}/firewall/rules/{pos}", a.protect("proxmox.manage", a.proxmoxDeleteFirewallRule))
	mux.HandleFunc("GET /api/v1/proxmox/nodes/{node}/backups", a.protect("proxmox.read", a.proxmoxNodeBackups))
	mux.HandleFunc("POST /api/v1/proxmox/nodes/{node}/{kind}/{vmid}/backup", a.protect("proxmox.manage", a.proxmoxCreateBackup))
	mux.HandleFunc("GET /api/v1/docker/inventory", a.protect("docker.read", a.dockerInventory))
	mux.HandleFunc("POST /api/v1/docker/containers/{id}/{action}", a.protect("docker.manage", a.dockerContainerAction))
	mux.HandleFunc("POST /api/v1/docker/containers/{id}/exec", a.protect("docker.manage", a.dockerContainerExec))
	mux.HandleFunc("GET /api/v1/kubernetes/overview", a.protect("kubernetes.read", a.kubernetesOverview))
	mux.HandleFunc("GET /api/v1/kubernetes/namespaces/{namespace}/pods/{pod}/logs", a.protect("kubernetes.read", a.kubernetesPodLogs))
	mux.HandleFunc("POST /api/v1/kubernetes/namespaces/{namespace}/pods/{pod}/exec", a.protect("kubernetes.manage", a.kubernetesPodExec))
	mux.HandleFunc("POST /api/v1/kubernetes/namespaces/{namespace}/deployments/{name}/{action}", a.protect("kubernetes.manage", a.kubernetesDeploymentAction))
	mux.HandleFunc("GET /api/v1/github/overview", a.protect("github.read", a.githubOverview))
	mux.HandleFunc("GET /api/v1/github/commits", a.protect("github.read", a.githubCommits))
	mux.HandleFunc("GET /api/v1/github/workflows", a.protect("github.read", a.githubWorkflows))
	mux.HandleFunc("POST /api/v1/github/workflow/action", a.protect("github.manage", a.githubWorkflowAction))
	mux.HandleFunc("POST /api/v1/github/run/action", a.protect("github.manage", a.githubRunAction))
	mux.HandleFunc("GET /api/v1/jobs", a.protect("jobs.read", a.listJobs))
	mux.HandleFunc("POST /api/v1/jobs", a.protect("jobs.manage", a.createJob))
	mux.HandleFunc("POST /api/v1/agents/hosts/{host_id}/token", a.protect("agents.manage", a.provisionAgentToken))
	mux.HandleFunc("POST /agent/v1/metrics", a.ingestAgentMetrics)
	mux.HandleFunc("POST /agent/v1/events", a.ingestAgentEvent)
	mux.HandleFunc("GET /api/v1/telemetry/hosts", a.protect("telemetry.read", a.hostTelemetry))
	mux.HandleFunc("POST /api/v1/terminal/tickets", a.protect("terminal.connect", a.createTerminalTicket))
	mux.HandleFunc("GET /api/v1/terminal/sessions", a.protect("audit.read", a.terminalSessions))
	mux.HandleFunc("GET /ws/terminal", a.terminalWebSocket)
	var dockerReader dockerapp.Reader
	if a.dockerClient != nil {
		dockerReader = a.dockerClient
	}
	dockerapp.MountRoutes(mux, a.protect, dockerReader, a.adapterError("docker"))
	kubernetesapp.MountRoutes(mux, a.protect, a.kubernetesClient, a.adapterError("kubernetes"))
	cloudflareapp.MountRoutes(mux, a.protect, a.cloudflareHandler)
	githubapp.MountRoutes(mux, a.protect, a.githubClient)
	terraformapp.MountRoutes(mux, a.protect, a.terraformRunner)
	operationsapp.MountRoutes(mux, a.protect)
	storageapp.MountRoutes(mux, a.protect, a.storageClient)
	ociapp.MountRoutes(mux, a.protect, a.ociHandler, a.adapterError("oci"))
	vncapp.MountRoutes(mux, a.protect, a.vncGateway, a.proxmoxClients, func(r *http.Request, action, target string) {
		session := currentSession(r)
		_ = a.audit.Append(r.Context(), auditrepo.Record{ActorID: session.User.ID, Action: action, Scope: security.ScopeRemote, ResourceType: "vnc_session", TargetName: target, Risk: security.RiskCritical, Decision: "allowed", RequestID: requestID(r.Context()), SourceIP: remoteIP(r)})
	})
	var pbsReader backupapp.Reader
	if a.pbsClient != nil {
		pbsReader = a.pbsClient
	}
	backupapp.MountRoutes(mux, a.protect, pbsReader)
	monitorapp.MountRoutes(mux, a.protect, a.monitorStore)
	powerapp.MountRoutes(mux, a.protect, a.powerClient, func(ctx context.Context, action, target, reason string) {
		session, _ := ctx.Value(sessionContextKey).(authdomain.Session)
		_ = a.audit.Append(ctx, auditrepo.Record{ActorID: session.User.ID, Action: action, Scope: security.ScopeRemote, ResourceType: "power_target", TargetName: target, Risk: security.RiskCritical, Decision: "allowed", Reason: reason})
	})
	return a.middleware(mux)
}

func (a *App) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]any{"status": "ok", "time": time.Now().UTC(), "self_protected": a.cfg.SelfProtected})
}
func (a *App) getOverview(w http.ResponseWriter, r *http.Request) {
	snapshot, err := a.overview.Snapshot(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "overview_failed", "Could not aggregate the operational overview.")
		return
	}
	writeJSON(w, 200, snapshot)
}
func (a *App) modules(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, []string{"overview", "proxmox", "cloud", "kubernetes", "docker", "github", "tunnels", "terraform", "telemetry", "remote-access", "storage", "settings", "audit"})
}

func (a *App) protect(permission string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(sessionCookie)
		if err != nil {
			writeError(w, 401, "authentication_required", "Authentication is required.")
			return
		}
		session, err := a.auth.Authenticate(r.Context(), cookie.Value)
		if err != nil {
			a.clearCookie(w)
			writeError(w, 401, "session_expired", "Your session is invalid or expired.")
			return
		}
		if permission != "" && !session.User.Can(permission) {
			_ = a.audit.Append(r.Context(), auditrepo.Record{ActorID: session.User.ID, Action: r.Method + " " + r.URL.Path, Scope: security.ScopeLocal, ResourceType: "http_route", Risk: security.RiskDangerous, Decision: "denied", Reason: "missing permission: " + permission, RequestID: requestID(r.Context()), SourceIP: remoteIP(r)})
			writeError(w, 403, "permission_denied", "You do not have permission for this action.")
			return
		}
		if r.Method != "GET" && r.Method != "HEAD" && r.Method != "OPTIONS" && !a.auth.VerifyCSRF(session, r.Header.Get("X-CSRF-Token")) {
			writeError(w, 403, "csrf_failed", "The CSRF token is invalid.")
			return
		}
		next(w, r.WithContext(context.WithValue(r.Context(), sessionContextKey, session)))
	}
}

func currentSession(r *http.Request) authdomain.Session {
	session, _ := r.Context().Value(sessionContextKey).(authdomain.Session)
	return session
}

func (a *App) middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		id := newRequestID()
		ctx := context.WithValue(r.Context(), contextKey("request_id"), id)
		w.Header().Set("X-Request-ID", id)
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Cache-Control", "no-store")
		if a.cfg.Environment == "development" {
			w.Header().Set("Access-Control-Allow-Origin", a.cfg.PublicURL)
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-CSRF-Token")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		}
		if r.Method == "OPTIONS" {
			w.WriteHeader(204)
			return
		}
		next.ServeHTTP(w, r.WithContext(ctx))
		a.logger.Info("http request", "request_id", id, "method", r.Method, "path", r.URL.Path, "duration_ms", time.Since(start).Milliseconds())
	})
}

func (a *App) setCookie(w http.ResponseWriter, token string, expires time.Time) {
	// WebSocket routes live under /ws, so limiting the session to /api makes an
	// authenticated REST UI fail when opening Terminal or noVNC. Retire the
	// legacy path-scoped cookie and issue one strict, host-only application cookie.
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Path: "/api", MaxAge: -1, HttpOnly: true, Secure: a.cfg.SecureCookies, SameSite: http.SameSiteStrictMode})
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: token, Path: "/", Expires: expires, HttpOnly: true, Secure: a.cfg.SecureCookies, SameSite: http.SameSiteStrictMode})
}
func (a *App) clearCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Path: "/api", MaxAge: -1, HttpOnly: true, Secure: a.cfg.SecureCookies, SameSite: http.SameSiteStrictMode})
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Path: "/", MaxAge: -1, HttpOnly: true, Secure: a.cfg.SecureCookies, SameSite: http.SameSiteStrictMode})
}
func decodeJSON(w http.ResponseWriter, r *http.Request, destination any) bool {
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		writeError(w, 400, "invalid_request", "Request body is invalid.")
		return false
	}
	return true
}
func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]any{"error": map[string]string{"code": code, "message": message}})
}
func newRequestID() string {
	value := make([]byte, 12)
	_, _ = rand.Read(value)
	return hex.EncodeToString(value)
}
func requestID(ctx context.Context) string {
	value, _ := ctx.Value(contextKey("request_id")).(string)
	return value
}
func remoteIP(r *http.Request) string {
	value := strings.TrimSpace(strings.Split(r.Header.Get("X-Forwarded-For"), ",")[0])
	if value != "" && net.ParseIP(value) != nil {
		return value
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return ""
}
func isAuthError(err error) bool {
	return errors.Is(err, authdomain.ErrInvalidCredentials) || errors.Is(err, authdomain.ErrUnauthorized)
}
