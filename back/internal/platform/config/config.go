package config

import (
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Environment               string
	HTTPAddr                  string
	PublicURL                 string
	DatabaseURL               string
	LocalTargetName           string
	SelfProtected             bool
	LocalAllowlist            []string
	LogLevelValue             string
	SessionTTL                time.Duration
	SecureCookies             bool
	EncryptionKey             string
	TOTPIssuer                string
	ProxmoxURL                string
	ProxmoxTokenID            string
	ProxmoxSecret             string
	ProxmoxTLSCA              string
	ProxmoxTLSInsecure        bool
	ProxmoxAdditionalConfigs  []string
	DockerEndpoint            string
	DockerTLSCA               string
	DockerClientCert          string
	DockerClientKey           string
	DockerFallbackSocket      string
	KubernetesURL             string
	KubernetesToken           string
	KubernetesCA              string
	KubernetesKubeconfig      string
	CloudflareToken           string
	CloudflareGlobalAPIKey    string
	CloudflareGlobalAPIEmail  string
	CloudflareAccounts        []string
	CloudflareZones           []string
	GitHubToken               string
	GitHubRepositories        []string
	TerraformWorkerURL        string
	TerraformWorkerToken      string
	TerraformWorkspaces       []string
	MergerFSRoot              string
	MergerFSSSHAddress        string
	MergerFSSSHUser           string
	MergerFSSSHRoot           string
	OCIConfigPath             string
	OCIConfigProfile          string
	WorkerID                  string
	WorkerCount               int
	SSHPrivateKeyPath         string
	SSHKnownHostsPath         string
	DevelopmentMasterLogin    bool
	DevelopmentMasterTimezone string
	DevelopmentMasterEmail    string
	VNCTargets                []string
	PBSURL                    string
	PBSTokenID                string
	PBSSecret                 string
	PBSTLSCA                  string
	MonitoringEnabled         bool
	PowerNUTAddress           string
	PowerNUTUPSName           string
	PowerWOLTargets           []string
	PowerWOLBroadcast         string
}

func Load() Config {
	return Config{
		Environment:               env("WC_HUB_ENV", "development"),
		HTTPAddr:                  env("WC_HUB_HTTP_ADDR", ":8080"),
		PublicURL:                 env("WC_HUB_PUBLIC_URL", "http://localhost:5173"),
		DatabaseURL:               env("WC_HUB_DATABASE_URL", ""),
		LocalTargetName:           env("WC_HUB_LOCAL_TARGET_NAME", "wc-hub-local"),
		SelfProtected:             envBool("WC_HUB_SELF_PROTECTED", true),
		LocalAllowlist:            split(env("WC_HUB_LOCAL_COMMAND_ALLOWLIST", "uptime,df,free,ip,ss,journalctl,docker,kubectl")),
		LogLevelValue:             env("WC_HUB_LOG_LEVEL", "info"),
		SessionTTL:                envDuration("WC_HUB_SESSION_TTL", 12*time.Hour),
		SecureCookies:             envBool("WC_HUB_SECURE_COOKIES", false),
		EncryptionKey:             secretEnv("WC_HUB_ENCRYPTION_KEY", "WC_HUB_ENCRYPTION_KEY_FILE"),
		TOTPIssuer:                env("WC_HUB_TOTP_ISSUER", "WC Hub"),
		ProxmoxURL:                strings.TrimRight(env("PROXMOX_API_URL", ""), "/"),
		ProxmoxTokenID:            env("PROXMOX_API_TOKEN_ID", ""),
		ProxmoxSecret:             env("PROXMOX_API_TOKEN_SECRET", ""),
		ProxmoxTLSCA:              env("PROXMOX_TLS_CA_PATH", ""),
		ProxmoxTLSInsecure:        envBool("PROXMOX_TLS_INSECURE_SKIP_VERIFY", false),
		ProxmoxAdditionalConfigs:  split(env("PROXMOX_ADDITIONAL_CONFIG_PATHS", "")),
		DockerEndpoint:            strings.TrimRight(env("DOCKER_PROXY_URL", ""), "/"),
		DockerTLSCA:               env("DOCKER_TLS_CA_PATH", ""),
		DockerClientCert:          env("DOCKER_CLIENT_CERT_PATH", ""),
		DockerClientKey:           env("DOCKER_CLIENT_KEY_PATH", ""),
		DockerFallbackSocket:      env("DOCKER_FALLBACK_SOCKET_PATH", ""),
		KubernetesURL:             strings.TrimRight(env("KUBERNETES_API_URL", ""), "/"),
		KubernetesToken:           env("KUBERNETES_TOKEN_PATH", "/var/run/secrets/kubernetes.io/serviceaccount/token"),
		KubernetesCA:              env("KUBERNETES_CA_PATH", "/var/run/secrets/kubernetes.io/serviceaccount/ca.crt"),
		KubernetesKubeconfig:      env("KUBERNETES_KUBECONFIG_PATH", ""),
		CloudflareToken:           env("CLOUDFLARE_API_TOKEN", ""),
		CloudflareGlobalAPIKey:    env("CLOUDFLARE_GLOBAL_API_KEY", ""),
		CloudflareGlobalAPIEmail:  env("CLOUDFLARE_GLOBAL_API_EMAIL", ""),
		CloudflareAccounts:        split(env("CLOUDFLARE_ACCOUNT_ALLOWLIST", "")),
		CloudflareZones:           split(env("CLOUDFLARE_ZONE_ALLOWLIST", "")),
		GitHubToken:               env("GITHUB_TOKEN", ""),
		GitHubRepositories:        split(env("GITHUB_REPOSITORY_ALLOWLIST", "")),
		TerraformWorkerURL:        strings.TrimRight(env("TERRAFORM_WORKER_URL", ""), "/"),
		TerraformWorkerToken:      env("TERRAFORM_WORKER_TOKEN", ""),
		TerraformWorkspaces:       split(env("TERRAFORM_WORKSPACE_ALLOWLIST", "")),
		MergerFSRoot:              env("MERGERFS_ROOT", ""),
		MergerFSSSHAddress:        env("MERGERFS_SSH_ADDRESS", ""),
		MergerFSSSHUser:           env("MERGERFS_SSH_USER", "root"),
		MergerFSSSHRoot:           env("MERGERFS_SSH_ROOT", ""),
		OCIConfigPath:             env("OCI_CONFIG_PATH", ""),
		OCIConfigProfile:          env("OCI_CONFIG_PROFILE", "DEFAULT"),
		WorkerID:                  env("WC_HUB_WORKER_ID", "wc-hub-1"),
		WorkerCount:               envInt("WC_HUB_WORKER_COUNT", 2),
		SSHPrivateKeyPath:         env("WC_HUB_SSH_PRIVATE_KEY_PATH", ""),
		SSHKnownHostsPath:         env("WC_HUB_SSH_KNOWN_HOSTS_PATH", ""),
		DevelopmentMasterLogin:    envBool("WC_HUB_DEV_MASTER_LOGIN", false),
		DevelopmentMasterTimezone: env("WC_HUB_DEV_MASTER_TIMEZONE", "America/Sao_Paulo"),
		DevelopmentMasterEmail:    strings.ToLower(strings.TrimSpace(env("WC_HUB_MASTER_EMAIL", "gabrielbiaggi3@gmail.com"))),
		VNCTargets:                split(env("VNC_ALLOWED_TARGETS", "")),
		PBSURL:                    strings.TrimRight(env("PBS_API_URL", ""), "/"),
		PBSTokenID:                env("PBS_API_TOKEN_ID", ""),
		PBSSecret:                 env("PBS_API_TOKEN_SECRET", ""),
		PBSTLSCA:                  env("PBS_TLS_CA_PATH", ""),
		MonitoringEnabled:         envBool("MONITORING_ENABLED", true),
		PowerNUTAddress:           env("NUT_SERVER", ""),
		PowerNUTUPSName:           env("NUT_UPS_NAME", ""),
		PowerWOLTargets:           split(env("WOL_ALLOWED_TARGETS", "")),
		PowerWOLBroadcast:         env("WOL_BROADCAST_ADDRESS", "255.255.255.255:9"),
	}
}

func secretEnv(valueKey, fileKey string) string {
	if value := strings.TrimSpace(os.Getenv(valueKey)); value != "" {
		return value
	}
	path := strings.TrimSpace(os.Getenv(fileKey))
	if path == "" {
		return ""
	}
	value, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(value))
}

func envDuration(key string, fallback time.Duration) time.Duration {
	value, err := time.ParseDuration(os.Getenv(key))
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}
func envInt(key string, fallback int) int {
	value, err := strconv.Atoi(os.Getenv(key))
	if err != nil || value < 1 {
		return fallback
	}
	return value
}

func (c Config) LogLevel() slog.Level {
	switch strings.ToLower(c.LogLevelValue) {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
func envBool(key string, fallback bool) bool {
	value, err := strconv.ParseBool(os.Getenv(key))
	if err != nil {
		return fallback
	}
	return value
}
func split(value string) []string {
	var out []string
	for _, item := range strings.Split(value, ",") {
		if v := strings.TrimSpace(item); v != "" {
			out = append(out, v)
		}
	}
	return out
}
